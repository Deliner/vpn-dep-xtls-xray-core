// Package session owns bounded, ephemeral browser authority and review drafts.
// It never stores the administrator credential or changes Fleet state.
package session

import (
	"errors"
	"sync"
	"time"
)

const Lifetime = 30 * time.Minute
const draftLifetime = 10 * time.Minute
const maxBytes = 64 << 20

var ErrExpired = errors.New("panel session or review expired")
var ErrBusy = errors.New("panel capacity or confirmation in progress")

type Identity struct {
	CSRF    string
	Expires time.Time
}

type Draft struct {
	Kind    string
	Body    []byte
	Result  string
	Expires time.Time
	running bool
}

type entry struct {
	Identity
	drafts map[string]*Draft
}

type Store struct {
	mu      sync.Mutex
	entries map[string]*entry
	bytes   int
	now     func() time.Time
}

func New() *Store {
	return &Store{entries: map[string]*entry{}, now: time.Now}
}
