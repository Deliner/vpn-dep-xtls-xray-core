package integration

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
	"vpn-script/internal/runtime/xray"
)

func TestIPv6ProtocolMatrix(t *testing.T) {
	if os.Getenv("NHOP_IPV6_ACCEPTANCE") != "1" {
		t.Skip("explicit IPv6 runtime acceptance; use just payload-ipv6")
	}
	for _, scenario := range []struct {
		name      string
		hops      int
		addresses []string
	}{
		{"ipv6-single", 1, []string{"::1"}},
		{"ipv6-chain", 3, []string{"::1"}},
		{"mixed-6-4-6", 3, []string{"::1", "127.0.0.1", "::1"}},
		{"mixed-4-6-4", 3, []string{"127.0.0.1", "::1", "127.0.0.1"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if directory := os.Getenv("NHOP_CONCURRENT_REPORT"); directory != "" {
				t.Setenv("NHOP_CONCURRENT_REPORT", filepath.Join(directory, scenario.name))
			}
			concurrentScenario(t, scenario.hops, scenario.addresses...)
		})
	}
	t.Run("certificate-trust", func(t *testing.T) { certificateTrustControls(t, "::1") })
}

func TestMissingRoutingRulesFailClosed(t *testing.T) {
	for _, stage := range []string{"client", "server"} {
		t.Run(stage, func(t *testing.T) {
			l := matrixPrepare(t, 1)
			captureLabRuntimes(t, l)
			l.startMatrixServers(t)
			// A mistaken direct path must reach this canary too, rather than an
			// unroutable synthetic address that could hide the routing failure.
			l.origin.http.URL = strings.Replace(l.origin.http.URL, "198.18.0.2", "127.0.0.1", 1)
			names := []string{"vless-reality", "trojan", "tuic"}
			for _, name := range names {
				config, profile := engines.Client(l.topology, name, l.secrets)
				stop := l.startClient(t, "valid-"+name, profile.Engine, config)
				require(t, waitPort(profile.SOCKSPort))
				routingProbe(t, l, profile.SOCKSPort, false)
				stop()
			}
			if stage == "server" {
				l.servers["node0"]()
				l.matrixServer(t, "node0", engines.NewCompiler(l.topology), "missing-routes")
			}
			for _, name := range names {
				config, profile := engines.Client(l.topology, name, l.secrets)
				if stage == "client" {
					dropInboundRoutes(profile.Engine, config)
				}
				stop := l.startClient(t, "unrouted-"+name, profile.Engine, config)
				require(t, waitPort(profile.SOCKSPort))
				routingProbe(t, l, profile.SOCKSPort, true)
				stop()
			}
		})
	}
}

func TestUDP443RemainsBlocked(t *testing.T) {
	if os.Getenv("NHOP_ROUTING_ISOLATED") != "private-netns" {
		t.Skip("real UDP/443 canary needs an isolated Linux namespace; use just lab-routing")
	}
	require(t, evidence.PrivateRoutingNamespace())
	for _, hops := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d-hop", hops), func(t *testing.T) { udp443Scenario(t, hops) })
	}
}

func udp443Scenario(t *testing.T, hops int) {
	t.Helper()
	l := matrixPrepare(t, hops)
	captureLabRuntimes(t, l)
	l.origin = startUDPEcho(t, l.origin, "198.18.0.2:0")
	second := startUDPEcho(t, origins{}, "198.18.0.2:0")
	compiler := engines.NewCompiler(l.topology)
	for _, id := range dag.Keys(l.topology.Nodes) {
		l.matrixServer(t, id, compiler, "isolated-routing")
	}
	canary := startUDPEcho(t, origins{}, "0.0.0.0:443")
	checkUDPEcho(t, canary)
	for _, name := range dag.Keys(l.topology.Routes) {
		t.Run(name, func(t *testing.T) {
			hits := canary.udpHits.Load()
			t.Cleanup(func() {
				if got := canary.udpHits.Load() - hits; got != 0 {
					t.Errorf("UDP/443 canary received %d forbidden packets", got)
				}
			})
			config, profile := engines.Client(l.topology, name, l.secrets)
			l.startClient(t, "udp443-"+name, profile.Engine, config)
			require(t, waitPort(profile.SOCKSPort))
			probeUDPPolicy(t, profile.SOCKSPort, l.origin.udp.LocalAddr().(*net.UDPAddr).Port, second.udp.LocalAddr().(*net.UDPAddr).Port)
			t.Log("one UDP source: allowed A, three denied UDP/443 packets, allowed B, allowed A again")
		})
	}
}

func TestShadowTLSPrivateListener(t *testing.T) {
	l := matrixLab(t, 1)
	data, err := engines.Node(l.topology, "node0", l.secrets)
	require(t, err)
	for _, listener := range data.Listeners {
		if listener.Tag != "node0.shadowtls-v3" || !listener.Internal {
			continue
		}
		if listener.Address != "127.0.0.1" || listener.Network != "tcp" {
			t.Fatalf("unexpected ShadowTLS private listener: %+v", listener)
		}
		require(t, waitPort(listener.Port))
		udp, err := net.ListenPacket("udp4", net.JoinHostPort(listener.Address, strconv.Itoa(listener.Port)))
		require(t, err)
		require(t, udp.Close())
		return
	}
	t.Fatal("ShadowTLS inner socket absent from manifest")
}

func TestProtocolAuthenticationControls(t *testing.T) {
	l := matrixLab(t, 1)
	for _, name := range dag.Keys(l.topology.Routes) {
		t.Run(name, func(t *testing.T) {
			first := l.topology.Routes[name].Path[0]
			saved := l.secrets.Services[first]
			bad := saved
			fresh, err := credentials.NewSecret(saved.Protocol)
			require(t, err)
			bad.Password, bad.UUID = fresh.Password, fresh.UUID
			l.secrets.Services[first] = bad
			config, profile := engines.Client(l.topology, name, l.secrets)
			l.secrets.Services[first] = saved
			stop := l.startClient(t, "bad-auth-"+name, profile.Engine, config)
			defer stop()
			hits := l.origin.httpHits.Load()
			if _, err := l.http(profile.SOCKSPort); err == nil || l.origin.httpHits.Load() != hits {
				t.Fatal("wrong credentials reached origin")
			}
		})
	}
}

func TestMixedEngineRelayFailure(t *testing.T) {
	t.Run("mixed-three-hop", testMixedEngineRelayFailure)
}

func testMixedEngineRelayFailure(t *testing.T) {
	l := matrixLab(t, 3)
	l.servers["node1"]()
	for _, name := range []string{"vless-xhttp", "tuic", "naive-h2", "shadowsocks"} {
		t.Run(name, func(t *testing.T) {
			config, profile := engines.Client(l.topology, name, l.secrets)
			stop := l.startClient(t, "stopped-"+name, profile.Engine, config)
			defer stop()
			httpHits, udpHits := l.origin.httpHits.Load(), l.origin.udpHits.Load()
			if _, err := l.http(profile.SOCKSPort); err == nil {
				t.Fatal("TCP bypassed stopped relay")
			}
			_, err := traffic.UDPRoundTrip(profile.SOCKSPort, l.origin.udp.LocalAddr().(*net.UDPAddr).Port, []byte("denied"), [4]byte{198, 18, 0, 2})
			if err == nil || l.origin.httpHits.Load() != httpHits || l.origin.udpHits.Load() != udpHits {
				t.Fatal("traffic bypassed stopped relay")
			}
		})
	}
}

func TestCertificateTrustControls(t *testing.T) {
	certificateTrustControls(t)
}

func certificateTrustControls(t *testing.T, addresses ...string) {
	l := matrixPrepare(t, 1, addresses...)
	l.startMatrixServers(t)
	for _, name := range []string{"trojan", "tuic", "naive-h2", "naive-h3", "masque-h3", "shadowtls-v3"} {
		for _, check := range []string{"authority", "hostname"} {
			t.Run(name+"/"+check, func(t *testing.T) {
				first := l.topology.Routes[name].Path[0]
				saved := l.secrets.Services[first]
				bad := saved
				endpoint := l.topology.Services[first]
				if check == "authority" {
					other := model.Secret{}
					require(t, credentials.EnsureTLS(model.Service{Protocol: "trojan", ServerName: "unrelated.invalid"}, &other))
					bad.TrustCertificate = other.Certificate
				} else {
					changed := endpoint
					changed.Service.ServerName = "unrelated.invalid"
					l.topology.Services[first] = changed
				}
				l.secrets.Services[first] = bad
				config, profile := engines.Client(l.topology, name, l.secrets)
				l.secrets.Services[first] = saved
				l.topology.Services[first] = endpoint
				stop := l.startClient(t, "bad-cert-"+name+"-"+check, profile.Engine, config)
				defer stop()
				hits := l.origin.httpHits.Load()
				if _, err := l.http(profile.SOCKSPort); err == nil || l.origin.httpHits.Load() != hits {
					t.Fatal("wrong TLS trust reached origin")
				}
			})
		}
	}
}

func TestMixedThreeHopPayloadAndFailureControls(t *testing.T) {
	l := setup(t)
	for name, port := range l.clients {
		t.Run(name, func(t *testing.T) {
			body, err := l.http(port)
			require(t, err)
			if !bytes.Equal(body, payload) {
				t.Fatal("TCP payload mismatch")
			}
			packet := make([]byte, 512)
			_, err = rand.Read(packet)
			require(t, err)
			result, err := traffic.UDPRoundTrip(port, l.origin.udp.LocalAddr().(*net.UDPAddr).Port, packet)
			require(t, err)
			if !bytes.Equal(packet, result) {
				t.Fatal("UDP payload mismatch")
			}
		})
	}
	l.wrongKeys(t)
	l.stoppedRelay(t)
}

func (l *lab) wrongKeys(t *testing.T) {
	t.Helper()
	for name, route := range l.topology.Routes {
		first := route.Path[0]
		bad := l.secrets.Services[first]
		fresh, err := xray.NewSecret(bad.Protocol)
		require(t, err)
		if bad.Protocol == model.Reality {
			bad.UUID = fresh.UUID
		} else {
			bad.Password = fresh.Password
		}
		port := l.port(t)
		stop := l.start(t, "bad-auth-"+name, l.clientBinary("xray"), xray.Client(xray.Outbound(l.topology, first, bad), port))
		hits := l.origin.httpHits.Load()
		if _, err := l.http(port); err == nil || l.origin.httpHits.Load() != hits {
			t.Fatal("wrong credentials reached the origin")
		}
		stop()
	}
}

func (l *lab) stoppedRelay(t *testing.T) {
	t.Helper()
	l.servers["relay"]()
	httpHits, udpHits := l.origin.httpHits.Load(), l.origin.udpHits.Load()
	for _, port := range l.clients {
		if _, err := l.http(port); err == nil {
			t.Fatal("TCP bypassed stopped middle hop")
		}
		if _, err := traffic.UDPRoundTrip(port, l.origin.udp.LocalAddr().(*net.UDPAddr).Port, []byte("must not arrive")); err == nil {
			t.Fatal("UDP bypassed stopped middle hop")
		}
	}
	if l.origin.httpHits.Load() != httpHits || l.origin.udpHits.Load() != udpHits {
		t.Fatal("origin received traffic while middle hop was down")
	}
}
