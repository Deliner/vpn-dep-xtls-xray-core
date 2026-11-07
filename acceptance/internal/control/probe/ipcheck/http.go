package ipcheck

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const ipResponseLimit = 128

// Endpoint validates the operator's HTTPS IP service before external work.
func Endpoint(endpoint string) (*url.URL, error) {
	value, err := url.Parse(endpoint)
	if err != nil || value.Scheme != "https" || value.Hostname() == "" || value.Fragment != "" || value.Opaque != "" {
		return nil, fmt.Errorf("IP endpoint must be an absolute HTTPS URL without a fragment")
	}
	if port := value.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return nil, fmt.Errorf("invalid IP endpoint port")
		}
	}
	return value, nil
}

// NewClient uses only the selected local SOCKS listener and system TLS trust.
func NewClient(port int) *http.Client {
	proxy := &url.URL{Scheme: "socks5h", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DialContext: dialer.DialContext,
		TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
		MaxResponseHeaderBytes: 16 * 1024, DisableKeepAlives: true, DisableCompression: true, ForceAttemptHTTP2: true}
	return &http.Client{Transport: transport, Timeout: 25 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Query confirms a bounded IP response over the supplied HTTPS client.
func Query(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	target, err := Endpoint(endpoint)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "text/plain")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("HTTPS probe failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTPS probe returned status %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, ipResponseLimit+1))
	if err != nil {
		return "", fmt.Errorf("read IP response: %w", err)
	}
	if len(raw) > ipResponseLimit {
		return "", fmt.Errorf("oversized IP response")
	}
	address, err := netip.ParseAddr(strings.TrimSpace(string(raw)))
	if err != nil || address.Zone() != "" {
		return "", fmt.Errorf("probe endpoint did not return an unscoped IP literal")
	}
	return address.Unmap().String(), nil
}
