package checkout

type Identity struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	SourceTree string `json:"source_tree"`
}
