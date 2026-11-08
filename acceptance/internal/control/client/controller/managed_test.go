package controller

import (
	"testing"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/session"
)

func TestManagedResumeRequiresSameIntent(t *testing.T) {
	original := session.Request{Network: network.Defaults(100), Native: session.Artifact{Path: "/old/native", SHA256: "native"},
		Frontend: session.Artifact{SHA256: "frontend"}, Cronet: session.Artifact{SHA256: "cronet"}}
	for _, kind := range []string{"source-path", "uid", "policy", "profile", "native", "frontend", "cronet"} {
		changed := original
		switch kind {
		case "source-path":
			changed.Native.Path = "/new/native"
		case "uid":
			changed.Network.EngineUID = 0
		case "policy":
			changed.Network.IPv6 = "block"
		case "profile":
			changed.Profile.Route = "different"
		case "native":
			changed.Native.SHA256 = "different"
		case "frontend":
			changed.Frontend.SHA256 = "different"
		case "cronet":
			changed.Cronet.SHA256 = "different"
		}
		allowed := kind == "source-path" || kind == "uid"
		if err := sameIntent(changed, original); (err == nil) != allowed {
			t.Fatalf("intent %s: allowed=%v err=%v", kind, allowed, err)
		}
	}
}
