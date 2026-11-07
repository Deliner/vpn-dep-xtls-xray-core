package cleanup

import "vpn-script/internal/quality/dependencies/scope"

const accountID int64 = 26311467
const sessionStart = "2026-11-07T13:04:42Z"
const sessionStop = "2026-11-07T16:19:33Z"

type Repository struct {
	ID          int64  `json:"id"`
	Name        string `json:"full_name"`
	Fork        bool   `json:"fork"`
	Private     bool   `json:"private"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description"`
	Owner       struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	} `json:"owner"`
	Parent *struct{ ID int64 } `json:"parent"`
	Source *struct{ ID int64 } `json:"source"`
}

type Target struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Fork        bool   `json:"fork"`
	ParentID    int64  `json:"parent_id,omitempty"`
	SourceID    int64  `json:"source_id,omitempty"`
	Description string `json:"description,omitempty"`
	Record      string `json:"record"`
	RecordSHA   string `json:"record_sha256"`
	CreatedAt   string `json:"created_at"`
}

type Plan struct {
	Schema       int          `json:"schema"`
	AccountID    int64        `json:"account_id"`
	ScopeSHA     string       `json:"scope_sha256"`
	InventorySHA string       `json:"inventory_sha256"`
	Keep         []scope.Fork `json:"keep"`
	Delete       []Target     `json:"delete"`
	Unknown      []string     `json:"unknown"`
	Absent       []string     `json:"already_absent"`
}

type Batch struct {
	PlanSHA  string   `json:"plan_sha256"`
	Offset   int      `json:"offset"`
	Verified []string `json:"verified_absent"`
	Failure  string   `json:"failure,omitempty"`
}
