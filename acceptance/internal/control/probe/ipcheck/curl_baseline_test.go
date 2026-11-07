package ipcheck

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

// This opt-in historical control reproduces the pre-native-client invocation.
// Normal verification and tests do not require curl.
func TestLegacyCurlProbeConfigControl(t *testing.T) {
	if os.Getenv("NHOP_CURL_BASELINE") != "1" {
		t.Skip("historical curl control; set NHOP_CURL_BASELINE=1 explicitly")
	}
	fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "198.51.100.42\n")
	}))
	hostileCurlConfig(t, fixture)
	response, err := process.Run(context.Background(), 30*time.Second, "curl", "--fail", "--silent", "--show-error", "--max-time", "25",
		"--noproxy", "", "--proxy", "socks5h://127.0.0.1:"+strconv.Itoa(fixture.port), fixture.endpoint)
	if err != nil || !response.Success || strings.TrimSpace(string(response.Output)) != "198.51.100.99" {
		t.Fatalf("historical config override was not reproduced: %s %v", response.Output, err)
	}
	t.Log("origin returned 198.51.100.42, but the historical probe accepted curl stdout 198.51.100.99")
}
