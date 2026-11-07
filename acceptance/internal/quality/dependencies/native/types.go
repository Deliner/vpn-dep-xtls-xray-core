package native

// Descriptor records upstream declarations, not proof that a component is linked.
type Descriptor struct {
	Path       string              `json:"path"`
	SHA256     string              `json:"sha256"`
	Components []map[string]string `json:"components"`
}

type Report struct {
	Schema      int          `json:"schema"`
	Repository  string       `json:"repository"`
	Revision    string       `json:"revision"`
	Chromium    string       `json:"chromium_version"`
	SourceTree  string       `json:"source_tree"`
	Descriptors []Descriptor `json:"descriptors"`
}
