package acceptance

import "testing"

func checkSystemDNS(t *testing.T, probe *controller, origins *dnsOrigins, phase string) {
	t.Helper()
	direct, tunneled := origins.direct.Load(), origins.tunneled.Load()
	response := probe.call(t, "resolve")
	afterDirect, afterTunnel := origins.direct.Load(), origins.tunneled.Load()
	if (response.Error == "") != (phase != "blocked") {
		t.Fatalf("system DNS %s: %+v", phase, response)
	}
	switch phase {
	case "direct":
		if afterDirect <= direct || afterTunnel != tunneled {
			t.Fatal("system DNS did not use the original resolver")
		}
	case "tunnel":
		if afterTunnel <= tunneled || afterDirect != direct {
			t.Fatal("active system DNS bypassed the TLS tunnel or did not reach it")
		}
	case "blocked":
		if afterDirect != direct || afterTunnel != tunneled {
			t.Fatal("DNS escaped the failed client session")
		}
	default:
		t.Fatal("unknown DNS expectation")
	}
	t.Logf("system DNS %s: direct=%d->%d HTTPS=%d->%d", phase, direct, afterDirect, tunneled, afterTunnel)
}
