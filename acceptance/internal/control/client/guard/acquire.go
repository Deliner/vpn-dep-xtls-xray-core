package guard

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"time"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
)

// Acquire uses an exclusive create in one nft transaction. Existing tables are
// never merged, flushed or adopted. The lease survives process failure.
func (m *Manager) Acquire(ctx context.Context, value profile.Imported, options network.Options) (Lease, error) {
	policy, err := network.Firewall(value, options)
	if err != nil {
		return Lease{}, err
	}
	var token [16]byte
	rand.Read(token[:])
	lease := Lease{Token: fmt.Sprintf("nhop:%x", token)}
	// nft's create-table operation ignores nested chain/rule declarations. Use
	// an exclusive empty table followed by the full policy in ONE transaction.
	policy = fmt.Sprintf("create table inet %s { comment %q; }\n", network.Table, lease.Token) + policy
	return m.install(ctx, lease, policy)
}

func (m *Manager) install(ctx context.Context, lease Lease, policy string) (Lease, error) {
	path, err := policyFile(policy)
	if err != nil {
		return Lease{}, err
	}
	defer os.Remove(path)
	result, err := m.run(ctx, 5*time.Second, "nft", "-f", path)
	inspect, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	current, inspectErr := m.inspect(inspect)
	if current.Token != lease.Token && (err != nil || !result.Success) {
		return Lease{}, fmt.Errorf("could not exclusively create client kill switch")
	}
	if inspectErr != nil || current.Token != lease.Token {
		return lease, fmt.Errorf("kill switch installed but identity could not be verified; protection retained")
	}
	if err != nil || !result.Success || ctx.Err() != nil {
		return current, fmt.Errorf("kill switch activation interrupted; protection retained")
	}
	return current, m.verifyProtection(inspect)
}

func policyFile(policy string) (string, error) {
	file, err := os.CreateTemp("", "nhop-guard-*.nft")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(policy); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
