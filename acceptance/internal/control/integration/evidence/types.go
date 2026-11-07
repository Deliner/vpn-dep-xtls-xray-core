// Package evidence records the inputs used by native runtime integration tests.
package evidence

type Input struct {
	Path            string
	ExpectedVersion string
}

type Runtime struct {
	Path              string `json:"path"`
	ExpectedVersion   string `json:"expected_version"`
	VersionOutput     string `json:"version_output"`
	SHA256            string `json:"sha256"`
	VerifiedUnchanged bool   `json:"verified_unchanged"`
	Error             string `json:"error,omitempty"`
}
