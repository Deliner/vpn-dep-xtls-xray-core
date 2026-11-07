package main

import (
	"fmt"
	"os"

	"vpn-script/internal/control/client/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
