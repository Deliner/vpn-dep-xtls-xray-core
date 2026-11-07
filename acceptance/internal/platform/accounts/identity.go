package accounts

import (
	"strconv"
	"strings"
)

// Inspect observes NSS identity without creating or modifying accounts. Callers
// retain responsibility for command deadlines, diagnostics and installation locks.
func (p Policy) Inspect(run Command) (Identity, error) {
	fields, err := p.record(run, "passwd", p.Name)
	if err != nil {
		return Identity{}, err
	}
	identity, err := p.user(fields)
	if err != nil {
		return Identity{}, err
	}
	fields, err = p.record(run, "group", p.Name)
	if err != nil {
		return Identity{}, err
	}
	if err := p.group(fields, identity.GID); err != nil {
		return Identity{}, err
	}
	raw, err := run("id", "-G", p.Name)
	if err != nil {
		return Identity{}, p.unsafe("cannot query supplementary groups")
	}
	if err := p.memberships(raw, identity.GID); err != nil {
		return Identity{}, err
	}
	if err := p.canonical(run, identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func (p Policy) canonical(run Command, expected Identity) error {
	fields, err := p.record(run, "passwd", strconv.Itoa(expected.UID))
	if err != nil {
		return err
	}
	actual, err := p.user(fields)
	if err != nil || actual != expected {
		return p.unsafe("UID lookup disagrees with the named service account")
	}
	fields, err = p.record(run, "group", strconv.Itoa(expected.GID))
	if err != nil {
		return err
	}
	return p.group(fields, expected.GID)
}

func (p Policy) record(run Command, database, key string) ([]string, error) {
	raw, err := run("getent", database, key)
	if err != nil {
		return nil, p.unsafe("cannot query " + database + " identity")
	}
	line := strings.TrimSuffix(string(raw), "\n")
	if strings.ContainsAny(line, "\r\n") {
		return nil, p.unsafe("ambiguous " + database + " response")
	}
	return strings.Split(line, ":"), nil
}
