package peer

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func (s *server) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/dns-query" {
		s.serveDNS(writer, request)
		return
	}
	if request.URL.Query().Get("token") != s.config.Token {
		http.Error(writer, "unknown fixture", http.StatusForbidden)
		return
	}
	peer, _, _ := net.SplitHostPort(request.RemoteAddr)
	s.mu.Lock()
	defer s.mu.Unlock()
	writer.Header().Set("Content-Type", "application/json")
	if request.URL.Path == "/metrics" {
		_ = json.NewEncoder(writer).Encode(s.metrics)
		return
	}
	s.metrics.HTTPPeers[peer]++
	_ = json.NewEncoder(writer).Encode(contract.Origin{Peer: peer, Payload: request.URL.Query().Get("payload")})
}

func (s *server) serveUDP(connection net.PacketConn, port string) {
	var buffer [4096]byte
	for {
		size, remote, err := connection.ReadFrom(buffer[:])
		if err != nil {
			return
		}
		peer, _, _ := net.SplitHostPort(remote.String())
		if port == "53" {
			s.mu.Lock()
			s.metrics.DirectDNS++
			s.mu.Unlock()
			if reply, err := dnsReply(buffer[:size], s.config.PeerIP); err == nil {
				_, _ = connection.WriteTo(reply, remote)
			}
			continue
		}
		message := string(buffer[:size])
		if !strings.HasPrefix(message, s.config.Token+":") {
			continue
		}
		s.mu.Lock()
		if port == "18082" {
			s.metrics.Canary[strings.TrimPrefix(message, s.config.Token+":")]++
		} else {
			s.metrics.UDPPeers[peer]++
		}
		s.mu.Unlock()
		reply, err := json.Marshal(contract.Origin{Peer: peer, Payload: message})
		if err == nil {
			_, _ = connection.WriteTo(reply, remote)
		}
	}
}
