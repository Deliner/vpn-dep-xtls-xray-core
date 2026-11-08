package fleetapi

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"vpn-script/internal/core/validate"
)

func New(endpoint string, caPEM []byte) (*Client, error) {
	base, err := endpointURL(endpoint)
	if err != nil {
		return nil, err
	}
	var roots *x509.CertPool
	if len(caPEM) > 0 {
		roots = x509.NewCertPool()
		if len(caPEM) > 1<<20 || !roots.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("invalid Fleet CA bundle")
		}
	}
	transport := &http.Transport{Proxy: nil, ForceAttemptHTTP2: true, DisableCompression: true,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots},
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
		MaxResponseHeaderBytes: 32 << 10, MaxIdleConns: 8, MaxConnsPerHost: 4, IdleConnTimeout: 90 * time.Second}
	client := &http.Client{Transport: transport, Timeout: 45 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{base: base, http: client}, nil
}

func endpointURL(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("Fleet endpoint must be an HTTPS origin")
	}
	if u.Path != "" && u.Path != "/" || u.RawPath != "" || validate.Host(u.Hostname()) != nil || strings.HasSuffix(u.Host, ":") {
		return "", fmt.Errorf("Fleet endpoint must have a valid host and no path")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || validate.Port(port) != nil {
			return "", fmt.Errorf("invalid Fleet endpoint port")
		}
	}
	return "https://" + u.Host, nil
}
