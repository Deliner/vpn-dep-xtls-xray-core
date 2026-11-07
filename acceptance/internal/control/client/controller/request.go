package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func load(options Options) (session.Request, error) {
	value, err := profile.Load(options.Profile)
	if err != nil {
		return session.Request{}, err
	}
	var runtimes Runtimes
	if err := safeio.LoadJSON(options.Runtimes, &runtimes); err != nil {
		return session.Request{}, err
	}
	if runtimes.SchemaVersion != 1 {
		return session.Request{}, fmt.Errorf("unsupported client runtime inventory")
	}
	for _, artifact := range []session.Artifact{runtimes.Xray, runtimes.SingBox, runtimes.Cronet} {
		if !filepath.IsAbs(artifact.Path) || !validate.Hash(artifact.SHA256) {
			return session.Request{}, fmt.Errorf("runtimes require absolute paths and explicit trusted SHA-256")
		}
		if strings.HasPrefix(filepath.Clean(artifact.Path), host.Directory+"/") {
			return session.Request{}, fmt.Errorf("runtime inputs must live outside disposable client session state")
		}
	}
	native := runtimes.Xray
	if value.Metadata.Engine == "sing-box" {
		native = runtimes.SingBox
	}
	return session.Request{Profile: value, Network: options.Network, Native: native, Frontend: runtimes.SingBox, Cronet: runtimes.Cronet}, nil
}

func prepare(ctx context.Context, owner *host.Owner, value session.Request) (session.Request, error) {
	directory, status, err := recorded(owner)
	if err != nil {
		return value, err
	}
	if directory != "" && status.Phase != "stopped" {
		return value, fmt.Errorf("previous client session requires an explicit stop before restart")
	}
	identity, err := host.Identity(ctx)
	if err != nil {
		return value, err
	}
	value.Network.EngineUID, value.EngineGID = identity.UID, identity.GID
	if err := pruneStopped(directory); err != nil {
		return value, err
	}
	value.Directory, err = owner.Next()
	return value, err
}

func recorded(owner *host.Owner) (string, session.Status, error) {
	directory, err := owner.Current()
	if err != nil || directory == "" {
		return directory, session.Status{}, err
	}
	var status session.Status
	err = host.ReadSession(directory, "status.json", &status)
	return directory, status, err
}
