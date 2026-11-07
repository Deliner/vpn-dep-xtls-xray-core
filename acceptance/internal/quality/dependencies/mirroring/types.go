package mirroring

import "vpn-script/internal/quality/dependencies/github"

type Source struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Commit string `json:"commit"`
}

type Snapshot struct {
	Source     Source            `json:"source"`
	Tree       string            `json:"tree"`
	Ancestors  int               `json:"ancestors_including_commit"`
	Repository github.Repository `json:"repository"`
	Published  bool              `json:"published"`
}
