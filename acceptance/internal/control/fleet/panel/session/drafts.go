package session

import (
	"bytes"

	"vpn-script/internal/platform/credential"
)

func (s *Store) Stage(token, kind string, body []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	value := s.lookup(token)
	if value == nil {
		return "", ErrExpired
	}
	if len(value.drafts) >= 8 || len(body) > 16<<20 || s.bytes+len(body) > maxBytes {
		return "", ErrBusy
	}
	id, err := credential.NewToken()
	if err != nil {
		return "", err
	}
	value.drafts[id] = &Draft{Kind: kind, Body: bytes.Clone(body), Expires: s.now().Add(draftLifetime)}
	s.bytes += len(body)
	return id, nil
}

// Read returns a copy; Begin additionally reserves the single confirmation.
func (s *Store) Read(token, id string, begin bool) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	value := s.lookup(token)
	if value == nil || value.drafts[id] == nil {
		return Draft{}, ErrExpired
	}
	draft := value.drafts[id]
	if draft.running {
		return Draft{}, ErrBusy
	}
	if begin && draft.Result == "" {
		draft.running = true
	}
	result := *draft
	result.Body = bytes.Clone(draft.Body)
	return result, nil
}

// Finish retains successful receipts for repeat POSTs. Failed attempts keep the
// exact request, so registry request IDs and revision CAS govern safe retries.
func (s *Store) Finish(token, id, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value := s.lookup(token)
	if value == nil || value.drafts[id] == nil {
		return
	}
	draft := value.drafts[id]
	draft.running = false
	draft.Result = result
	if result != "" {
		s.bytes -= len(draft.Body)
		draft.Body = nil
	}
}
