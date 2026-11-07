package owned

import (
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/quality/dependencies/modules"
)

func TestOwnedDownloadRejectsWrongNamespaceAndMutableRevisions(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, request := range [][2]string{{"upstream/repo", sha}, {"Deliner/unrelated", sha}, {"Deliner/vpn-dep-source/../escape", sha}, {"Deliner/vpn-dep-source", "main"}} {
		if _, err := requestIdentity(request[0], request[1]); err == nil {
			t.Fatal("accepted unreviewed download", request)
		}
	}
	if repo, err := requestIdentity("Deliner/vpn-dep-source/lib/linux_arm64", sha); err != nil || repo != "Deliner/vpn-dep-source" {
		t.Fatalf("lost root repository for submodule: %s, %v", repo, err)
	}
}

func TestOwnedIdentityRequiresOriginalCommitAndAuthenticatedChecksums(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("GOMODCACHE", cache)
	sha, sum := strings.Repeat("a", 40), "h1:"+strings.Repeat("a", 43)+"="
	for _, changed := range []string{"none", "commit", "repository", "sum", "path", "error"} {
		item := Module{Path: "github.com/Deliner/vpn-dep-source", Version: "v1.0.0", Sum: sum, GoModSum: sum,
			Origin: &modules.Origin{VCS: "git", URL: "https://github.com/Deliner/vpn-dep-source", Hash: sha},
			Info:   filepath.Join(cache, "module.info"), GoMod: filepath.Join(cache, "module.mod"), Zip: filepath.Join(cache, "module.zip"), Dir: filepath.Join(cache, "module")}
		switch changed {
		case "commit":
			item.Origin.Hash = strings.Repeat("b", 40)
		case "repository":
			item.Origin.URL = "https://github.com/upstream/source"
		case "sum":
			item.Sum = ""
		case "path":
			item.Zip = filepath.Join(cache, "..", "outside.zip")
		case "error":
			item.Error = "unavailable"
		}
		err := verifyIdentity(item, "Deliner/vpn-dep-source", "Deliner/vpn-dep-source", sha)
		if (err != nil) != (changed != "none") {
			t.Fatalf("incorrect identity decision %s: %v", changed, err)
		}
	}
}
