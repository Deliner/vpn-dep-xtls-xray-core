package guard

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

var tokenPattern = regexp.MustCompile(`^nhop:[0-9a-f]{32}$`)

// NewIntent reserves a unique identity without changing the kernel. Persist it
// in the protected session journal before calling AcquireReserved. Handle zero
// describes an intention, never evidence that protection has been installed.
func NewIntent() Lease {
	var token [16]byte
	rand.Read(token[:])
	return Lease{Token: fmt.Sprintf("nhop:%x", token)}
}
