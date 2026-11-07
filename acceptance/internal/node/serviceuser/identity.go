package serviceuser

import (
	"strconv"
	"strings"
)

// Inspect observes NSS identity without creating or modifying accounts. Callers
// retain responsibility for command deadlines, diagnostics and installation locks.
func Inspect(run Command) (Identity, error) {
	fields, err := record(run, "passwd", Name)
	if err != nil {
		return Identity{}, err
	}
	identity, err := user(fields)
	if err != nil {
		return Identity{}, err
	}
	fields, err = record(run, "group", Name)
	if err != nil {
		return Identity{}, err
	}
	if err := group(fields, identity.GID); err != nil {
		return Identity{}, err
	}
	raw, err := run("id", "-G", Name)
	if err != nil {
		return Identity{}, unsafe("cannot query supplementary groups")
	}
	if err := memberships(raw, identity.GID); err != nil {
		return Identity{}, err
	}
	if err := canonical(run, identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func canonical(run Command, expected Identity) error {
	fields, err := record(run, "passwd", strconv.Itoa(expected.UID))
	if err != nil {
		return err
	}
	actual, err := user(fields)
	if err != nil || actual != expected {
		return unsafe("UID lookup disagrees with the named service account")
	}
	fields, err = record(run, "group", strconv.Itoa(expected.GID))
	if err != nil {
		return err
	}
	return group(fields, expected.GID)
}

func record(run Command, database, key string) ([]string, error) {
	raw, err := run("getent", database, key)
	if err != nil {
		return nil, unsafe("cannot query " + database + " identity")
	}
	line := strings.TrimSuffix(string(raw), "\n")
	if strings.ContainsAny(line, "\r\n") {
		return nil, unsafe("ambiguous " + database + " response")
	}
	return strings.Split(line, ":"), nil
}
