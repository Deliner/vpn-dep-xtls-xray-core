package modules

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"

	"vpn-script/internal/quality/dependencies/commands"
)

func queryOrigin(ctx context.Context, run commands.Runner, cache string, request Module) (Resolution, error) {
	result := Resolution{Path: request.Path, Version: request.Version}
	args := []string{"GOMODCACHE=" + cache, "GOPROXY=direct", "GOSUMDB=sum.golang.org", "GOTOOLCHAIN=local",
		"GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=", "GOINSECURE=", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_ALLOW_PROTOCOL=https", "go", "list", "-mod=readonly", "-m", "-json", request.Path + "@" + request.Version}
	raw, err := run(ctx, "env", args...)
	if err != nil {
		result.Failure = err.Error()
		return result, err
	}
	origin, err := resolvedOrigin(raw, request)
	if err != nil {
		result.Failure = err.Error()
		return result, err
	}
	result.Origin = origin
	return result, nil
}

func resolvedOrigin(raw []byte, request Module) (*Origin, error) {
	var module Module
	if err := json.Unmarshal(raw, &module); err != nil {
		return nil, err
	}
	if module.Path != request.Path || module.Version != request.Version || module.Error != nil || module.Origin == nil {
		return nil, fmt.Errorf("direct VCS query did not resolve %s@%s", request.Path, request.Version)
	}
	return module.Origin, checkOrigin(module.Origin)
}

func checkOrigin(origin *Origin) error {
	if origin == nil {
		return fmt.Errorf("source origin is missing")
	}
	digest, err := hex.DecodeString(origin.Hash)
	if err != nil || origin.VCS != "git" || len(digest) != 20 && len(digest) != 32 {
		return fmt.Errorf("direct VCS query has no full Git commit identity")
	}
	address, err := url.Parse(origin.URL)
	if err != nil || address.Scheme != "https" || address.Host == "" || address.User != nil || address.RawQuery != "" || address.Fragment != "" {
		return fmt.Errorf("direct VCS query returned an unsupported source URL")
	}
	return nil
}
