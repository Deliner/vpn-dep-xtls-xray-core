package main

import (
	"fmt"

	"vpn-script/internal/quality/linuxlab/netem"
)

func checkNetem(args []string) error {
	if len(args) != 4 {
		return fmt.Errorf("use just acceptance-masque-verify DIRECTORY ARCH COMMIT SNAPSHOT")
	}
	results, err := netem.Verify(args[0], args[1], args[2], args[3])
	if err != nil {
		return err
	}
	for _, result := range results {
		fmt.Printf("round=%d policy=%s complete_TCP=%d/%d complete_UDP=%d/%d delay_failures=%d capture=%d\n",
			result.Round, result.Policy, result.TCP, result.ExpectedTCP, result.UDP, result.ExpectedUDP, result.DelayFailures, result.Packets)
	}
	return nil
}
