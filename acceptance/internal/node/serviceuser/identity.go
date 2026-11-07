package serviceuser

import "vpn-script/internal/platform/accounts"

// Inspect retains the server's named identity contract; creation belongs to its installer.
func Inspect(run Command) (Identity, error) {
	return (accounts.Policy{Name: Name}).Inspect(run)
}
