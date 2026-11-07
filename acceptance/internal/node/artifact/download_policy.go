package artifact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"syscall"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
)

type downloadFailure struct {
	message string
	cause   error
	retry   bool
	after   time.Duration
}

// Keep signed URLs out of diagnostics while preserving cancellation/error types.
func (failure *downloadFailure) Error() string { return failure.message }
func (failure *downloadFailure) Unwrap() error { return failure.cause }

func downloadClient() *http.Client {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment,
		DialContext:       (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2: true, DisableCompression: true, MaxIdleConns: 10,
		IdleConnTimeout: 30 * time.Second, TLSHandshakeTimeout: 10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second, ExpectContinueTimeout: time.Second}
	return &http.Client{Transport: transport, CheckRedirect: httpsRedirect}
}

func validateAsset(asset model.Asset) error {
	parsed, err := url.Parse(asset.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("runtime asset requires an HTTPS URL without user info or fragment")
	}
	if asset.Size <= 0 || asset.Size > 150_000_000 || !validate.Hash(asset.SHA256) {
		return fmt.Errorf("runtime asset requires a SHA-256 and size between 1 and 150000000 bytes")
	}
	return nil
}

func httpsRedirect(request *http.Request, previous []*http.Request) error {
	if request.URL.Scheme != "https" || request.URL.User != nil || len(previous) >= 10 {
		return fmt.Errorf("refusing unsafe or excessive runtime redirects")
	}
	if request.Response != nil {
		if delay := retryAfter(request.Response.Header.Get("Retry-After")); delay > 0 {
			return waitRetry(request.Context(), delay)
		}
	}
	return nil
}

func transientStatus(status int) bool {
	return status == 408 || status == 429 || status == 500 || status == 502 || status == 503 || status == 504
}

func networkFailure(message string, err error) error {
	var timeout net.Error
	retry := errors.As(err, &timeout) && timeout.Timeout()
	var dns *net.DNSError
	retry = retry || errors.As(err, &dns) && dns.IsTemporary
	for _, cause := range []error{io.EOF, io.ErrUnexpectedEOF, syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.EPIPE} {
		retry = retry || errors.Is(err, cause)
	}
	if retry {
		message += " (temporary network failure)"
	}
	return &downloadFailure{message: message, cause: err, retry: retry}
}

func retryAfter(value string) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
		return time.Duration(min(seconds, 120)) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return min(max(time.Until(date), 0), 2*time.Minute)
	}
	return 0
}

func retryDelay(attempt int, after time.Duration) time.Duration {
	base := (500 * time.Millisecond) << attempt
	return max(after, base+time.Duration(rand.Int64N(int64(base/2))))
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
