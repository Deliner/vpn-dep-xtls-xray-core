package main

import (
	"fmt"
	"vpn-script/internal/quality/runtimebuild/singrelease"
)

func singbox(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: runtimepack singbox-verify|singbox-package|singbox-archive-verify POLICY arguments...")
	}
	policy, err := singrelease.Load(args[1])
	if err != nil {
		return err
	}
	switch {
	case args[0] == "singbox-metadata-verify" && len(args) == 5:
		return singrelease.VerifyMetadata(policy, singrelease.Inputs{Arch: args[2], Candidate: args[3], Native: args[4]})
	case args[0] == "singbox-verify" && len(args) == 5:
		_, err = singrelease.Verify(policy, singrelease.Inputs{Arch: args[2], Candidate: args[3], Native: args[4]})
		return err
	case args[0] == "singbox-package" && len(args) == 6:
		return singrelease.Package(policy, singrelease.Inputs{Arch: args[2], Candidate: args[3], Native: args[4]}, args[5])
	case args[0] == "singbox-archive-verify" && len(args) == 4:
		return singrelease.VerifyArchive(policy, args[2], args[3])
	default:
		return fmt.Errorf("invalid sing-box release arguments")
	}
}
