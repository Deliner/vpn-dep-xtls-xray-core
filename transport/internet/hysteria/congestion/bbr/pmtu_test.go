package bbr

import (
	"testing"

	"github.com/apernet/quic-go/congestion"
	"github.com/stretchr/testify/require"
)

func TestNHOPBBRPathMTUDecrease(t *testing.T) {
	for _, profile := range []Profile{ProfileStandard, ProfileConservative, ProfileAggressive} {
		for _, state := range []string{"minimum", "initial", "learned", "maximum"} {
			t.Run(string(profile)+"/"+state, func(t *testing.T) { checkBBRReduction(t, profile, state) })
		}
	}
}

func checkBBRReduction(t *testing.T, profile Profile, state string) {
	t.Helper()
	b := NewBbrSender(DefaultClock{}, 1439, profile)
	windows := map[string]congestion.ByteCount{
		"minimum": b.minCongestionWindow, "initial": b.initialCongestionWindow,
		"learned": 64000, "maximum": b.maxCongestionWindow,
	}
	b.congestionWindow = windows[state]
	b.recoveryWindow = b.maxCongestionWindow
	for _, size := range []congestion.ByteCount{1200, 1252, 1200} {
		require.NotPanics(t, func() { b.SetMaxDatagramSize(size) })
		require.Equal(t, size, b.maxDatagramSize)
		require.GreaterOrEqual(t, b.GetCongestionWindow(), b.minCongestionWindow)
		require.LessOrEqual(t, b.GetCongestionWindow(), b.maxCongestionWindow)
		require.GreaterOrEqual(t, b.recoveryWindow, b.minCongestionWindow)
		require.LessOrEqual(t, b.recoveryWindow, b.maxCongestionWindow)
		if state == "learned" {
			require.Equal(t, congestion.ByteCount(64000), b.congestionWindow)
		}
		if state == "minimum" {
			require.Equal(t, minCongestionWindowPackets*size, b.congestionWindow)
		}
		if state == "initial" {
			require.Equal(t, b.initialCongestionWindow, b.congestionWindow)
		}
		require.True(t, b.CanSend(0))
	}
}

func TestNHOPBBRRejectsNonpositiveDatagramSize(t *testing.T) {
	for _, size := range []congestion.ByteCount{0, -1} {
		b := NewBbrSender(DefaultClock{}, 1439, ProfileStandard)
		require.Panics(t, func() { b.SetMaxDatagramSize(size) })
		require.Equal(t, congestion.ByteCount(1439), b.maxDatagramSize)
	}
}
