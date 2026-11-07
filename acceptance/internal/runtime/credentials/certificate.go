package credentials

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
	"vpn-script/internal/platform/safeio"
)

func EnsureTLS(service model.Service, secret *model.Secret) error {
	spec, _ := protocol.Lookup(service.Protocol)
	if !spec.TLS {
		if service.Protocol == "shadowtls-v3" {
			return donorTrust(service.TLS.CA, secret)
		}
		return nil
	}
	if service.TLS.Certificate != "" {
		if err := importTLS(service.TLS, secret); err != nil {
			return err
		}
	} else if secret.Certificate == "" {
		if err := generateTLS(service.ServerName, secret); err != nil {
			return err
		}
	}
	return verifyTLS(service.ServerName, *secret)
}

func verifyTLS(name string, secret model.Secret) error {
	pair, err := tls.X509KeyPair([]byte(secret.Certificate), []byte(secret.CertificateKey))
	if err != nil {
		return fmt.Errorf("invalid TLS certificate/key pair")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil || leaf.VerifyHostname(name) != nil {
		return fmt.Errorf("TLS certificate does not cover server_name")
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if secret.TrustCertificate != "" {
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(secret.TrustCertificate)) {
			return fmt.Errorf("invalid CA certificate")
		}
	}
	intermediates := x509.NewCertPool()
	for _, raw := range pair.Certificate[1:] {
		cert, parseErr := x509.ParseCertificate(raw)
		if parseErr != nil {
			return parseErr
		}
		intermediates.AddCert(cert)
	}
	_, err = leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, DNSName: name})
	if err != nil {
		return fmt.Errorf("TLS certificate trust/validity check failed; supply valid certificate and CA: %w", err)
	}
	return nil
}

func importTLS(files model.TLSFiles, secret *model.Secret) error {
	cert, err := safeio.Read(files.Certificate, 1_000_000)
	if err != nil {
		return err
	}
	key, err := safeio.Read(files.Key, 1_000_000)
	if err != nil {
		return err
	}
	var ca []byte
	if files.CA != "" {
		ca, err = safeio.Read(files.CA, 1_000_000)
		if err != nil {
			return err
		}
	}
	secret.Certificate, secret.CertificateKey, secret.TrustCertificate = string(cert), string(key), string(ca)
	return nil
}

func generateTLS(name string, secret *model.Secret) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(90 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true}
	if address := net.ParseIP(name); address != nil {
		cert.IPAddresses = []net.IP{address}
	} else {
		cert.DNSNames = []string{name}
	}
	raw, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		return err
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	secret.Certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}))
	secret.CertificateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}))
	secret.TrustCertificate = secret.Certificate
	return nil
}

func donorTrust(path string, secret *model.Secret) error {
	secret.TrustCertificate = ""
	if path == "" {
		return nil
	}
	raw, err := safeio.Read(path, 1_000_000)
	if err != nil {
		return err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(raw) {
		return fmt.Errorf("invalid donor CA")
	}
	secret.TrustCertificate = string(raw)
	return nil
}
