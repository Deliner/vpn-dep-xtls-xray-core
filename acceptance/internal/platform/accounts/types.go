// Package accounts verifies dedicated non-login NSS identities without changing them.
package accounts

import "fmt"

const Home = "/nonexistent"
const Shell = "/usr/sbin/nologin"

type Identity struct {
	UID int
	GID int
}

type Command func(...string) ([]byte, error)

type Policy struct {
	Name      string
	MinimumID uint32
}

func (p Policy) unsafe(reason string) error {
	return fmt.Errorf("unsafe service identity: %s; inspect account %s before retrying", reason, p.Name)
}
