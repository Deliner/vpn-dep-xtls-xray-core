package modules

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func originResponse(t *testing.T, path, version string, origin *Origin) []byte {
	t.Helper()
	raw, err := json.Marshal(Module{Path: path, Version: version, Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDirectOriginUsesIsolatedHTTPSCacheAndExactVersion(t *testing.T) {
	origin := &Origin{VCS: "git", URL: "https://github.com/owner/repo", Hash: strings.Repeat("a", 40), Subdir: "nested"}
	request := Module{Path: "example.org/nested", Version: "v1.2.3"}
	run := func(_ context.Context, binary string, args ...string) ([]byte, error) {
		if binary != "env" || args[len(args)-1] != "example.org/nested@v1.2.3" {
			t.Fatalf("unexpected command %s %q", binary, args)
		}
		for _, setting := range []string{"GOMODCACHE=/isolated", "GOPROXY=direct", "GOTOOLCHAIN=local", "GIT_ALLOW_PROTOCOL=https", "GIT_CONFIG_GLOBAL=/dev/null", "-mod=readonly"} {
			if !slices.Contains(args, setting) {
				t.Fatalf("missing %s in %q", setting, args)
			}
		}
		return originResponse(t, request.Path, request.Version, origin), nil
	}
	result, err := queryOrigin(context.Background(), run, "/isolated", request)
	if err != nil || result.Origin == nil || *result.Origin != *origin {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestDirectOriginRejectsUnverifiableIdentities(t *testing.T) {
	valid := Origin{VCS: "git", URL: "https://github.com/o/r", Hash: strings.Repeat("a", 40)}
	request := Module{Path: "module", Version: "v1.0.0"}
	for _, address := range []string{"http://github.com/o/r", "ssh://git@github.com/o/r", "https://u:p@github.com/o/r", "https://github.com/o/r?query", "https://github.com/o/r#ref"} {
		origin := valid
		origin.URL = address
		if _, err := resolvedOrigin(originResponse(t, request.Path, request.Version, &origin), request); err == nil {
			t.Fatalf("accepted unsafe origin %s", address)
		}
	}
	for _, origin := range []*Origin{nil, {VCS: "git", URL: valid.URL, Hash: "abc"}, {VCS: "svn", URL: valid.URL, Hash: valid.Hash}} {
		if _, err := resolvedOrigin(originResponse(t, request.Path, request.Version, origin), request); err == nil {
			t.Fatalf("accepted unverified origin %+v", origin)
		}
	}
	for _, raw := range [][]byte{originResponse(t, "other", request.Version, &valid), originResponse(t, request.Path, "v2.0.0", &valid)} {
		if _, err := resolvedOrigin(raw, request); err == nil {
			t.Fatalf("accepted wrong module identity %s", raw)
		}
	}
}
