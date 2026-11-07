package github

type Repository struct {
	ID          int64       `json:"id"`
	FullName    string      `json:"full_name"`
	HTMLURL     string      `json:"html_url"`
	Fork        bool        `json:"fork"`
	Parent      *Repository `json:"parent"`
	Source      *Repository `json:"source"`
	Description string      `json:"description,omitempty"`
	Private     bool        `json:"private,omitempty"`
}

type ForkRecord struct {
	Upstream      Repository `json:"upstream"`
	Fork          Repository `json:"fork"`
	Created       bool       `json:"created"`
	SharedNetwork bool       `json:"shared_network,omitempty"`
}
