package main

import (
	"fmt"
	"os"

	"vpn-script/internal/quality/runtimebuild"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "use just masque-probe-build CANDIDATE NEW_BUILD_DIRECTORY")
		os.Exit(2)
	}
	if err := runtimebuild.MasqueOverlay(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
