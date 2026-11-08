package peer

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func (s *server) serveDNS(writer http.ResponseWriter, request *http.Request) {
	peer, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil || peer != s.config.PeerIP {
		http.Error(writer, "resolver requires the exit", http.StatusForbidden)
		return
	}
	var query []byte
	if request.Method == http.MethodGet {
		query, err = base64.RawURLEncoding.DecodeString(request.URL.Query().Get("dns"))
	} else {
		query, err = io.ReadAll(io.LimitReader(request.Body, 4097))
	}
	if err == nil {
		query, err = dnsReply(query, s.config.PeerIP)
	}
	if err != nil {
		http.Error(writer, "invalid fixture DNS request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.metrics.TunnelDNS++
	s.metrics.DNSQueries[questionName(query)]++
	s.mu.Unlock()
	writer.Header().Set("Content-Type", "application/dns-message")
	_, _ = writer.Write(query)
}

// Called only after dnsReply validated the uncompressed, single-question name.
func questionName(query []byte) string {
	var labels []string
	for offset := 12; query[offset] != 0; {
		length := int(query[offset])
		labels = append(labels, string(query[offset+1:offset+1+length]))
		offset += 1 + length
	}
	return strings.ToLower(strings.Join(labels, "."))
}

// The fixture answers one bounded IN/A question, with TTL zero to prevent caching.
func dnsReply(query []byte, address string) ([]byte, error) {
	if len(query) < 17 || len(query) > 4096 || binary.BigEndian.Uint16(query[4:6]) != 1 {
		return nil, fmt.Errorf("expected one DNS question")
	}
	end, err := questionEnd(query)
	if err != nil {
		return nil, err
	}
	answer := append([]byte(nil), query[:end]...)
	answer[2], answer[3], answer[6], answer[7] = 0x81, 0x80, 0, 1
	for index := 8; index < 12; index++ {
		answer[index] = 0
	}
	answer = append(answer, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 0, 0, 4)
	return append(answer, net.ParseIP(address).To4()...), nil
}

func questionEnd(query []byte) (int, error) {
	end := 12
	for {
		if end >= len(query) || query[end] > 63 {
			return 0, fmt.Errorf("invalid DNS name")
		}
		length := int(query[end])
		end += 1 + length
		if length == 0 {
			break
		}
	}
	if end+4 > len(query) || binary.BigEndian.Uint16(query[end:end+2]) != 1 || binary.BigEndian.Uint16(query[end+2:end+4]) != 1 {
		return 0, fmt.Errorf("expected IN/A question")
	}
	return end + 4, nil
}
