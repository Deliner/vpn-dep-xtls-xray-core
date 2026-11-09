package main

import (
	"fmt"
	"os"
	"vpn-script/internal/quality/runtimerelease"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
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
