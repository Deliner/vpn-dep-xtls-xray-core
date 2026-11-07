package serviceuser

import (
	"strconv"
	"strings"
)

func user(fields []string) (Identity, error) {
	if len(fields) != 7 || fields[0] != Name {
		return Identity{}, unsafe("expected one named service account")
	}
	uid, uidErr := identifier(fields[2])
	gid, gidErr := identifier(fields[3])
	if uidErr != nil || gidErr != nil {
		return Identity{}, unsafe("service UID and primary GID must be nonzero numeric IDs")
	}
	if fields[5] != Home || fields[6] != Shell {
		return Identity{}, unsafe("service account requires /nonexistent home and /usr/sbin/nologin shell")
	}
	return Identity{UID: uid, GID: gid}, nil
}

func group(fields []string, expected int) error {
	if len(fields) != 4 || fields[0] != Name {
		return unsafe("expected one named service group")
	}
	gid, err := identifier(fields[2])
	if err != nil || gid != expected {
		return unsafe("primary GID differs from the named service group")
	}
	if fields[3] != "" && fields[3] != Name {
		return unsafe("service group contains other or ambiguous members")
	}
	return nil
}

func memberships(raw []byte, expected int) error {
	values := strings.Fields(string(raw))
	if len(values) == 0 {
		return unsafe("empty group membership response")
	}
	for _, value := range values {
		gid, err := identifier(value)
		if err != nil || gid != expected {
			return unsafe("service account has unexpected supplementary groups")
		}
	}
	return nil
}

func identifier(value string) (int, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 || id == 1<<32-1 {
		return 0, unsafe("invalid or privileged numeric identity")
	}
	return int(id), nil
}
