package singrelease

import (
	"encoding/json"
	"strings"
)

func archiveName(policy Policy, arch string) string {
	return strings.TrimSuffix(archivePrefix(policy, arch), "/") + ".tar.gz"
}

func archivePrefix(policy Policy, arch string) string {
	return "sing-box-" + policy.Version + "-linux-" + arch + "/"
}

func releaseMetadata(policy Policy) []byte {
	raw, _ := json.MarshalIndent(policy, "", "  ")
	return append(raw, '\n')
}
