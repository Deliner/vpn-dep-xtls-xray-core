package accounts_test

import (
	"strings"
	"testing"

	"vpn-script/internal/platform/accounts"
)

func TestDifferentClientPolicyDoesNotBorrowServerIdentity(t *testing.T) {
	policy := accounts.Policy{Name: "nhop-client", MinimumID: 100}
	if _, err := policy.Inspect(command(records(), "")); err == nil {
		t.Fatal("client accepted the node's identity")
	}
	responses := map[string]string{}
	for key, value := range records() {
		responses[strings.ReplaceAll(key, "vpn-nhop", "nhop-client")] = strings.ReplaceAll(value, "vpn-nhop", "nhop-client")
	}
	identity, err := policy.Inspect(command(responses, ""))
	if err != nil || identity.UID != 987 || identity.GID != 986 {
		t.Fatal("dedicated client identity rejected", identity, err)
	}
	responses["getent passwd nhop-client"] = "nhop-client:x:99:986::/nonexistent:/usr/sbin/nologin\n"
	if _, err := policy.Inspect(command(responses, "")); err == nil {
		t.Fatal("reserved low UID accepted")
	}
}
