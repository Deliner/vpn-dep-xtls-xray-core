package catalog

import (
	"net/url"
	"regexp"
	"strings"

	"vpn-script/internal/quality/dependencies/modules"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

func source(module modules.Module) (string, string) {
	if module.Origin == nil {
		return "", "module metadata has no VCS origin"
	}
	origin := module.Origin
	if origin.VCS != "git" || !commitPattern.MatchString(origin.Hash) {
		return "", "origin has no full Git commit SHA"
	}
	parsed, err := url.Parse(origin.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "origin needs an explicitly verified GitHub mirror"
	}
	name := strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), ".git")
	if parsed.RawPath != "" || !namePattern.MatchString(name) {
		return "", "unsupported GitHub repository URL"
	}
	return name, ""
}

func reference(scope string, module modules.Module) Reference {
	ref := Reference{Scope: scope, Module: module.Path, Version: module.Version}
	if module.Origin != nil {
		ref.Commit, ref.Subdir = module.Origin.Hash, module.Origin.Subdir
	}
	return ref
}
