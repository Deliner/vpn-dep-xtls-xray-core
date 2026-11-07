package accounts

import (
	"strconv"
	"strings"
)

func (p Policy) user(fields []string) (Identity, error) {
	if len(fields) != 7 || fields[0] != p.Name {
		return Identity{}, p.unsafe("expected one named service account")
	}
	uid, uidErr := p.identifier(fields[2])
	gid, gidErr := p.identifier(fields[3])
	if uidErr != nil || gidErr != nil {
		return Identity{}, p.unsafe("service UID and primary GID must be nonzero numeric IDs")
	}
	if fields[5] != Home || fields[6] != Shell {
		return Identity{}, p.unsafe("service account requires /nonexistent home and /usr/sbin/nologin shell")
	}
	return Identity{UID: uid, GID: gid}, nil
}

func (p Policy) group(fields []string, expected int) error {
	if len(fields) != 4 || fields[0] != p.Name {
		return p.unsafe("expected one named service group")
	}
	gid, err := p.identifier(fields[2])
	if err != nil || gid != expected {
		return p.unsafe("primary GID differs from the named service group")
	}
	if fields[3] != "" && fields[3] != p.Name {
		return p.unsafe("service group contains other or ambiguous members")
	}
	return nil
}

func (p Policy) memberships(raw []byte, expected int) error {
	values := strings.Fields(string(raw))
	if len(values) == 0 {
		return p.unsafe("empty group membership response")
	}
	for _, value := range values {
		gid, err := p.identifier(value)
		if err != nil || gid != expected {
			return p.unsafe("service account has unexpected supplementary groups")
		}
	}
	return nil
}

func (p Policy) identifier(value string) (int, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id < uint64(max(p.MinimumID, 1)) || id == 1<<32-1 {
		return 0, p.unsafe("invalid or privileged numeric identity")
	}
	return int(id), nil
}
