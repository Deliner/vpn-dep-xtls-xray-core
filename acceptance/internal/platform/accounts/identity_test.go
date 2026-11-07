package accounts_test

import (
	"fmt"
	"strings"
	"testing"

	"vpn-script/internal/platform/accounts"
)

func records() map[string]string {
	user := "vpn-nhop:x:987:986::/nonexistent:/usr/sbin/nologin\n"
	group := "vpn-nhop:x:986:\n"
	return map[string]string{"getent passwd vpn-nhop": user, "getent passwd 987": user,
		"getent group vpn-nhop": group, "getent group 986": group, "id -G vpn-nhop": "986\n"}
}

func command(responses map[string]string, failed string) accounts.Command {
	return func(args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		value, found := responses[key]
		if !found || key == failed {
			return nil, fmt.Errorf("query failed: %s", key)
		}
		return []byte(value), nil
	}
}

func rejected(t *testing.T, responses map[string]string, failed string) {
	t.Helper()
	identity, err := (accounts.Policy{Name: "vpn-nhop"}).Inspect(command(responses, failed))
	if err == nil || !strings.Contains(err.Error(), "unsafe service identity") || identity != (accounts.Identity{}) {
		t.Fatalf("unsafe or incomplete NSS identity accepted: %+v, %v", identity, err)
	}
}

func TestInspectAcceptsDedicatedNonLoginAccount(t *testing.T) {
	for _, members := range []string{"", "vpn-nhop"} {
		responses := records()
		responses["getent group vpn-nhop"] = "vpn-nhop:x:986:" + members + "\n"
		responses["getent group 986"] = responses["getent group vpn-nhop"]
		identity, err := (accounts.Policy{Name: "vpn-nhop"}).Inspect(command(responses, ""))
		if err != nil || identity != (accounts.Identity{UID: 987, GID: 986}) {
			t.Fatal("dedicated account rejected", identity, err)
		}
	}
}

func TestInspectRejectsUnsafeOrAmbiguousPasswdRecords(t *testing.T) {
	for _, user := range []string{
		"", "vpn-nhop:x:987:986", "other:x:987:986::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:0:986::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:987:0::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:-1:986::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:4294967295:986::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:4294967296:986::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:root:986::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:987:987::/nonexistent:/usr/sbin/nologin",
		"vpn-nhop:x:987:986::/tmp:/usr/sbin/nologin",
		"vpn-nhop:x:987:986::/nonexistent:/bin/sh",
		"vpn-nhop:x:987:986::/nonexistent:/usr/sbin/nologin\nroot:x:0:0::/root:/bin/sh",
	} {
		responses := records()
		responses["getent passwd vpn-nhop"] = user + "\n"
		rejected(t, responses, "")
	}
}

func TestInspectRejectsUnexpectedGroupAuthority(t *testing.T) {
	for _, group := range []string{"", "root:x:986:", "vpn-nhop:x:0:", "vpn-nhop:x:985:",
		"vpn-nhop:x:986:other", "vpn-nhop:x:986:vpn-nhop,other", "vpn-nhop:x:986:\nother:x:986:"} {
		responses := records()
		responses["getent group vpn-nhop"] = group + "\n"
		rejected(t, responses, "")
	}
	for _, groups := range []string{"", "0", "986 0", "986 27", "986 invalid", "4294967296"} {
		responses := records()
		responses["id -G vpn-nhop"] = groups + "\n"
		rejected(t, responses, "")
	}
}

func TestInspectRejectsLookupDisagreementAndQueryFailures(t *testing.T) {
	for key := range records() {
		rejected(t, records(), key)
	}
	for key, value := range map[string]string{
		"getent passwd 987": "other:x:987:986::/nonexistent:/usr/sbin/nologin\n",
		"getent group 986":  "other:x:986:\n",
	} {
		responses := records()
		responses[key] = value
		rejected(t, responses, "")
	}
}
