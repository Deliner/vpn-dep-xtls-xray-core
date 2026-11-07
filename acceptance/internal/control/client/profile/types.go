package profile

import "vpn-script/internal/core/model"

// Imported binds one native configuration to the route metadata checked at import.
// It is private credential material; diagnostics must not print Config or URI.
type Imported struct {
	SchemaVersion int           `json:"schema_version"`
	Topology      string        `json:"topology"`
	Route         string        `json:"route"`
	Metadata      model.Profile `json:"metadata"`
	Config        model.Object  `json:"config"`
}
