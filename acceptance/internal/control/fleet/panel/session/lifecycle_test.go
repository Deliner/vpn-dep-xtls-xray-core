package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviewIsolationAndConcurrentConfirmation(t *testing.T) {
	s := New()
	token, _, err := s.Create()
	require(t, err)
	other, _, err := s.Create()
	require(t, err)
	raw := []byte("immutable request")
	id, err := s.Stage(token, "publication", raw)
	require(t, err)
	raw[0] = 'X'
	if _, err := s.Read(other, id, true); !errors.Is(err, ErrExpired) {
		t.Fatal("another administrator session obtained this review")
	}
	var winners atomic.Int32
	var wait sync.WaitGroup
	for range 16 {
		wait.Go(func() {
			if value, err := s.Read(token, id, true); err == nil && string(value.Body) == "immutable request" {
				winners.Add(1)
			}
		})
	}
	wait.Wait()
	if winners.Load() != 1 {
		t.Fatal("concurrent confirmations did not have exactly one owner")
	}
	s.Finish(token, id, "/panel/nodes/node-a")
	value, err := s.Read(token, id, true)
	require(t, err)
	if value.Result != "/panel/nodes/node-a" || len(value.Body) != 0 {
		t.Fatal("committed confirmation lost its receipt or retained private manifest")
	}
}

func TestReviewAndSessionExpiryReleaseAuthority(t *testing.T) {
	s := New()
	now := time.Now()
	s.now = func() time.Time { return now }
	token, _, err := s.Create()
	require(t, err)
	id, err := s.Stage(token, "rollout", []byte("plan"))
	require(t, err)
	now = now.Add(draftLifetime)
	if _, err := s.Read(token, id, true); !errors.Is(err, ErrExpired) {
		t.Fatal("expired review can still be committed")
	}
	if _, ok := s.Lookup(token); !ok {
		t.Fatal("review expiry incorrectly ended the longer browser session")
	}
	now = now.Add(Lifetime)
	if _, ok := s.Lookup(token); ok {
		t.Fatal("expired session retained administrator authority")
	}
	if _, err := s.Stage(token, "rollout", []byte("plan")); !errors.Is(err, ErrExpired) {
		t.Fatal("expired session can stage another review")
	}
}

func TestPanelCapacityAndLogoutBoundPrivateRequests(t *testing.T) {
	s := New()
	token, _, err := s.Create()
	require(t, err)
	for range 8 {
		_, err := s.Stage(token, "publication", []byte("request"))
		require(t, err)
	}
	if _, err := s.Stage(token, "publication", []byte("request")); !errors.Is(err, ErrBusy) {
		t.Fatal("unbounded pending requests per session")
	}
	s.Delete(token)
	if _, ok := s.Lookup(token); ok {
		t.Fatal("logout retained authority")
	}
	for range 64 {
		_, _, err := s.Create()
		require(t, err)
	}
	if _, _, err := s.Create(); !errors.Is(err, ErrBusy) {
		t.Fatal("browser session count is unbounded")
	}
}
