// Package overlay owns the reviewed source changes and diagnostic artifacts.
// Source inspectors prepare changes without publishing or mutating candidates.
package overlay

// Change binds a target to its original hash and the reviewed replacement bytes.
// SourceSHA is empty only for a new diagnostic helper whose target is absent.
type Change struct {
	Target    string
	SourceSHA string
	Modified  []byte
}

// Policies describe the unchanged queue contracts selected by source hashes.
type Policies struct {
	HTTP3            string
	Connection       string
	Send             string
	ServerQueueLimit int
}
