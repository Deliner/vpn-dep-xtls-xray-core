package pull

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/credential"
)

func TestLostEnrollmentResponseReusesDurableCredentialAfterRestart(t *testing.T) {
	f := newFixture(t)
	before, err := readConnection(f.root)
	require(t, err)
	f.loseResponse = true
	if _, err := f.runner.Step(context.Background()); err == nil {
		t.Fatal("lost enrollment response reported success")
	}
	f.open(t)
	status, err := f.runner.Step(context.Background())
	require(t, err)
	if status.Phase != "waiting" {
		t.Fatal("unconfigured agent did not report waiting")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.enrollments) != 2 || f.enrollments[0] != f.enrollments[1] || f.enrollments[1].Credential != before.Credential || !credential.ValidToken(before.Credential) {
		t.Fatal("retry changed enrollment identity or credential")
	}
	after, err := readConnection(f.root)
	require(t, err)
	raw, err := os.ReadFile(filepath.Join(f.root, "connection.json"))
	require(t, err)
	if !after.Enrolled || after.Credential != before.Credential || after.EnrollmentToken != "" || strings.Contains(string(raw), f.bootstrap.Invitation.Token) {
		t.Fatal("successful enrollment did not retire the invitation")
	}
}

func TestForeignEnrollmentResponseDoesNotCommitOrPoll(t *testing.T) {
	f := newFixture(t)
	f.wrongIdentity = true
	if _, err := f.runner.Step(context.Background()); err == nil {
		t.Fatal("foreign enrollment identity accepted")
	}
	value, err := readConnection(f.root)
	require(t, err)
	f.mu.Lock()
	defer f.mu.Unlock()
	if value.Enrolled || value.EnrollmentToken == "" || len(f.reports) != 0 {
		t.Fatal("foreign identity altered local enrollment")
	}
}

func TestActiveCredentialSurvivesRestartWithoutReenrollment(t *testing.T) {
	f := newFixture(t)
	_, err := f.runner.Step(context.Background())
	require(t, err)
	f.open(t)
	_, err = f.runner.Step(context.Background())
	require(t, err)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.enrollments) != 1 || len(f.reports) != 2 {
		t.Fatal("restart re-enrolled or lost heartbeat")
	}
}
