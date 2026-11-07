package catalog

type Reference struct {
	Scope     string `json:"scope"`
	Module    string `json:"module"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Subdir    string `json:"subdir"`
	SourceURL string `json:"source_url,omitempty"`
}

type Entry struct {
	Upstream   string      `json:"upstream"`
	References []Reference `json:"references"`
}

type Unresolved struct {
	Reference
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

type Plan struct {
	Schema       int          `json:"schema"`
	InventorySHA string       `json:"inventory_sha256"`
	Entries      []Entry      `json:"entries"`
	Unresolved   []Unresolved `json:"unresolved"`
	BasePlanSHA  string       `json:"base_plan_sha256,omitempty"`
	MirrorsSHA   string       `json:"mirrors_sha256,omitempty"`
}

type Batch struct {
	PlanSHA  string   `json:"plan_sha256"`
	Offset   int      `json:"offset"`
	Limit    int      `json:"limit"`
	Verified []string `json:"verified"`
	Failure  string   `json:"failure,omitempty"`
}
