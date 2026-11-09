package owned

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestDownloadFailureKeepsServerDiagnostic(t *testing.T) {
	failure := errors.New("exit status 1")
	_, err := downloadResult([]byte(`{"Error":"proxy: fetch timed out"}`), failure)
	if err == nil || !strings.Contains(err.Error(), "fetch timed out") || !strings.Contains(err.Error(), failure.Error()) {
		t.Fatalf("download failure lost its diagnostic: %v", err)
	}
	_, err = downloadResult([]byte(`{"Path":"a/path"}`), failure)
	if !errors.Is(err, failure) {
		t.Fatalf("valid JSON masked failed command: %v", err)
	}
	if _, err = downloadResult([]byte(`not JSON`), nil); err == nil {
		t.Fatal("invalid response accepted")
	}
}

func TestDirectFallbackKeepsAuthenticatedChecksumPolicy(t *testing.T) {
	t.Chdir(t.TempDir())
	var executable string
	var arguments []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		executable, arguments = name, args
		return []byte(`{"Error":"deliberate offline test"}`), errors.New("download failed")
	}
	_, err := Download(context.Background(), run, "Deliner/vpn-dep-apernet-quic-go", strings.Repeat("a", 40))
	if err == nil || !strings.Contains(err.Error(), "deliberate offline test") || executable != "env" {
		t.Fatalf("failed download was not preserved: %v", err)
	}
	for _, setting := range []string{"GOPROXY=https://proxy.golang.org,direct", "GOSUMDB=sum.golang.org", "GOPRIVATE=", "GONOPROXY=", "GONOSUMDB="} {
		if !slices.Contains(arguments, setting) {
			t.Fatalf("authenticated fallback setting missing: %s", setting)
		}
	}
}
