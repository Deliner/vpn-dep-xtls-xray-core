package netem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordedOrdinaryExperiments(t *testing.T) {
	root := os.Getenv("NHOP_MASQUE_RECORDED_RUNS")
	if root == "" {
		t.Skip("optional completed ordinary-candidate controls")
	}
	for _, test := range []struct{ run, policy string }{{"masque-netem-tCX9EY6q", "default"}, {"masque-netem-5qhGNSKW", "reno"}} {
		report := filepath.Join(root, test.run, "reports")
		if _, err := completeInventory(report, "evidence.sha256"); err != nil {
			t.Fatal(err)
		}
		result, err := verifyPhases(report, test.policy)
		if err != nil {
			t.Fatal(err)
		}
		result.Packets, err = packetCapture(report)
		if err != nil {
			t.Fatal(err)
		}
		if err := matchingCredentials(report); err != nil {
			t.Fatal(err)
		}
		if err := processEvidence(report); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %+v", test.run, result)
	}
}
