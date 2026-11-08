package session

import "vpn-script/internal/platform/credential"

func (s *Store) Create() (string, Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if len(s.entries) >= 64 {
		return "", Identity{}, ErrBusy
	}
	token, err := credential.NewToken()
	if err != nil {
		return "", Identity{}, err
	}
	csrf, err := credential.NewToken()
	if err != nil {
		return "", Identity{}, err
	}
	value := Identity{CSRF: csrf, Expires: s.now().Add(Lifetime)}
	s.entries[credential.TokenHash(token)] = &entry{Identity: value, drafts: map[string]*Draft{}}
	return token, value, nil
}

func (s *Store) Lookup(token string) (Identity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	value := s.lookup(token)
	if value == nil {
		return Identity{}, false
	}
	return value.Identity, true
}

func (s *Store) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := credential.TokenHash(token)
	if value := s.entries[key]; value != nil {
		for _, draft := range value.drafts {
			s.bytes -= len(draft.Body)
		}
		delete(s.entries, key)
	}
}

func (s *Store) lookup(token string) *entry {
	if !credential.ValidToken(token) {
		return nil
	}
	return s.entries[credential.TokenHash(token)]
}

func (s *Store) prune() {
	now := s.now()
	for key, value := range s.entries {
		for id, draft := range value.drafts {
			if !now.Before(draft.Expires) || !now.Before(value.Expires) {
				s.bytes -= len(draft.Body)
				delete(value.drafts, id)
			}
		}
		if !now.Before(value.Expires) {
			delete(s.entries, key)
		}
	}
}
