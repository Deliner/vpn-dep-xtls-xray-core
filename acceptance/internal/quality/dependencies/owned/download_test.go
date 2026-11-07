package owned

import (
	"errors"
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
