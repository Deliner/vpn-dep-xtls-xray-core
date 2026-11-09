package main

import (
	"fmt"
	"os"

	"vpn-script/internal/quality/acceptance/installation/node"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "use just acceptance-installer-verify DIRECTORY ARCH COMMIT SNAPSHOT")
		os.Exit(2)
	}
	if err := node.Verify(os.Args[1], os.Args[2], os.Args[3], os.Args[4]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Native installer source, archives, upgrade, SIGKILL recovery and all-protocol evidence verified.")
}
