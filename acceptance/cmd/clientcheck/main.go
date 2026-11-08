package main

import (
	"fmt"
	"os"

	"vpn-script/internal/quality/acceptance/installed"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "use just acceptance-client-installed-verify DIRECTORY IPV6_POLICY")
		os.Exit(2)
	}
	if err := installed.Verify(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
