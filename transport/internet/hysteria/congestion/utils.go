package congestion

import (
	"fmt"
	"strings"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/congestion"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/bbr"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/brutal"
)

const (
	TypeBBR  = "bbr"
	TypeReno = "reno"
)

func NormalizeType(congestionType string) (string, error) {
	switch normalized := strings.ToLower(congestionType); normalized {
	case "", TypeBBR:
		return TypeBBR, nil
	case TypeReno:
		return TypeReno, nil
	default:
		return "", fmt.Errorf("unsupported congestion type %q", congestionType)
	}
}

func NormalizeBBRProfile(profile string) (string, error) {
	normalized, err := bbr.ParseProfile(profile)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func UseBBR(conn *quic.Conn, profile bbr.Profile) {
	conn.SetCongestionControl(bbr.NewBbrSender(
		bbr.DefaultClock{},
		seedPacketSize(conn.InitialPacketSize(), bbr.GetInitialPacketSize(conn.RemoteAddr())),
		profile,
	))
}

// seedPacketSize picks the datagram size to seed a replacement congestion
// controller with, given the size QUIC itself starts at and the guess derived
// from the remote address.
//
// Start with a size supported by QUIC rather than relying on the next PMTU
// notification to correct packet-sized congestion and pacing windows. Later
// notifications can increase or decrease the size as the active path changes.
func seedPacketSize(quicSize, byAddr congestion.ByteCount) congestion.ByteCount {
	if quicSize <= 0 {
		return byAddr
	}
	return min(quicSize, byAddr)
}

func UseBrutal(conn *quic.Conn, tx uint64, disableLossCompensation bool) {
	conn.SetCongestionControl(brutal.NewBrutalSender(tx, disableLossCompensation))
}

func UseConfigured(conn *quic.Conn, congestionType, bbrProfile string) {
	switch congestionType {
	case TypeReno:
		return
	default:
		UseBBR(conn, bbr.Profile(bbrProfile))
	}
}
