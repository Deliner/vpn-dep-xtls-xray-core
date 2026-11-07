package serviceuser

import "fmt"

const Name = "vpn-nhop"
const Home = "/nonexistent"
const Shell = "/usr/sbin/nologin"

type Identity struct {
	UID int
	GID int
}

type Command func(...string) ([]byte, error)

func unsafe(reason string) error {
	return fmt.Errorf("unsafe service identity: %s; inspect account %s before retrying", reason, Name)
}
