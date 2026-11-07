package evidence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func runtimeFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCaptureUsesObservedVersionAndExactExecutableBytes(t *testing.T) {
	path := runtimeFixture(t, "printf 'Xray 26.10.31 nhop-security-1\\n'")
	observed, err := Capture(context.Background(), map[string]Input{"xray": {Path: path, ExpectedVersion: "26.10.31"}})
	if err != nil {
		t.Fatal(err)
	}
	want, err := safeio.FileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	actual := observed["xray"]
	if actual.SHA256 != want || actual.Path != path || actual.ExpectedVersion != "26.10.31" || actual.VersionOutput != "Xray 26.10.31 nhop-security-1" {
		t.Fatalf("expected pin substituted for observed identity: %+v", actual)
	}
	if actual.VerifiedUnchanged {
		t.Fatal("identity verified before the end of the run")
	}
	if err := Verify(observed); err != nil || !observed["xray"].VerifiedUnchanged {
		t.Fatalf("unchanged runtime was not verified: %v", err)
	}
}

func TestVerifyRejectsModifiedExecutable(t *testing.T) {
	path := runtimeFixture(t, "printf 'version\\n'")
	observed, err := Capture(context.Background(), map[string]Input{"engine": {Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("modified executable"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Verify(observed); err == nil || observed["engine"].VerifiedUnchanged || !strings.Contains(observed["engine"].Error, "changed") {
		t.Fatalf("modified executable retained a successful identity: %v, %+v", err, observed)
	}
}

func TestCaptureRetainsFailedIdentityAndChecksRemainingRuntime(t *testing.T) {
	bad := runtimeFixture(t, "printf 'diagnostic from runtime\\n'; exit 7")
	good := runtimeFixture(t, "printf 'actual runtime version\\n'")
	observed, err := Capture(context.Background(), map[string]Input{"a-bad": {Path: bad}, "b-good": {Path: good}})
	if err == nil || observed["a-bad"].Error == "" || observed["a-bad"].VersionOutput != "diagnostic from runtime" {
		t.Fatalf("failed version command lost its evidence: %v, %+v", err, observed)
	}
	if observed["b-good"].SHA256 == "" || observed["b-good"].VersionOutput != "actual runtime version" {
		t.Fatal("later runtime was not inspected")
	}
	if err := Verify(observed); err == nil || observed["a-bad"].VerifiedUnchanged || !observed["b-good"].VerifiedUnchanged {
		t.Fatalf("failed capture was promoted to verified: %v, %+v", err, observed)
	}
}

func TestEmptyOrTruncatedVersionOutputCannotIdentifyRuntime(t *testing.T) {
	for _, body := range []string{"exit 0", "printf '%070000d' 0"} {
		path := runtimeFixture(t, body)
		observed, err := Capture(context.Background(), map[string]Input{"engine": {Path: path}})
		if err == nil || observed["engine"].Error == "" {
			t.Fatal("empty or incomplete version accepted")
		}
	}
}

func TestMissingIdentitiesCannotBeVerified(t *testing.T) {
	if _, err := Capture(context.Background(), nil); err == nil {
		t.Fatal("empty capture accepted")
	}
	if err := Verify(nil); err == nil {
		t.Fatal("empty verification accepted")
	}
	if observed, err := Capture(context.Background(), map[string]Input{"missing": {}}); err == nil || observed["missing"].Error == "" {
		t.Fatal("missing executable accepted")
	}
}
