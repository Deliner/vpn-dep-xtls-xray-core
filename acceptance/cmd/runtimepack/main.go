package main

import (
	"fmt"
	"os"
	"strings"
	"vpn-script/internal/quality/runtimebuild/native/attribution"
	"vpn-script/internal/quality/runtimebuild/native/release"
	"vpn-script/internal/quality/runtimerelease"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 4 && args[0] == "source-package" {
		return attribution.Package(args[2], args[1], args[3])
	}
	if len(args) == 4 && args[0] == "source-verify" {
		return attribution.Verify(args[2], args[3], args[1])
	}
	if len(args) >= 2 && strings.HasPrefix(args[0], "cronet-") {
		return cronet(args)
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: runtimepack package|stage|verify POLICY arguments...")
	}
	policy, err := runtimerelease.LoadPolicy(args[1])
	if err != nil {
		return err
	}
	switch {
	case args[0] == "package" && len(args) == 6:
		return runtimerelease.Prepare(policy, runtimerelease.Inputs{Arch: args[2], Candidate: args[3], Native: args[4]}, args[5])
	case args[0] == "stage" && len(args) == 5:
		return runtimerelease.Stage(policy, args[2], args[3], args[4])
	case args[0] == "verify" && len(args) == 3:
		return runtimerelease.VerifyStage(policy, args[2])
	default:
		return fmt.Errorf("invalid runtimepack arguments")
	}
}

func cronet(args []string) error {
	policy, err := release.Load(args[1])
	if err != nil {
		return err
	}
	switch {
	case args[0] == "cronet-verify" && len(args) == 3:
		return release.Verify(policy, args[2])
	case args[0] == "cronet-package" && len(args) == 4:
		return release.Package(policy, args[2], args[3])
	case args[0] == "cronet-package-verify" && len(args) == 3:
		return release.VerifyPackage(policy, args[2])
	case args[0] == "cronet-library" && len(args) == 5:
		return release.ExtractLibrary(policy, args[2], args[3], args[4])
	default:
		return fmt.Errorf("invalid Cronet release arguments")
	}
}
