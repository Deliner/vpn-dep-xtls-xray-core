package routing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func TestForeignRuleSelectorsNeverReachDelete(t *testing.T) {
	for _, raw := range []string{
		`{"priority":30911,"src":"all","table":"30910","protocol":"192"}`,
		`{"priority":30911,"src":"all","table":"30910","protocol":"191","iif":"other"}`,
		`{"priority":30911,"src":"all","table":"30910","protocol":"191","not":null}`,
		`{"priority":30911,"src":"all","table":"30910","protocol":"191","dport":22}`,
	} {
		var value item
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		called := false
		m := &Manager{rules: []rule{{"-4", "", "30910", 30911}}, run: func(context.Context, time.Duration, ...string) (process.Result, error) {
			called = true
			return process.Result{Success: true}, nil
		}}
		if err := m.remove(t.Context(), "-4", value); err == nil || called {
			t.Fatal("foreign rule was accepted or a delete was attempted")
		}
	}
}
