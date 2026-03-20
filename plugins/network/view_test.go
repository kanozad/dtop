package network

import (
	"strings"
	"testing"

	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/types"
)

func TestNetGraphRows(t *testing.T) {
	tests := []struct {
		h    int
		want int
	}{
		// Below the 4-line fixed overhead: no graphs.
		{0, 0},
		{3, 0},
		{4, 0}, // (4-4)/2=0
		{5, 0}, // (5-4)/2=0 (integer division)
		// (6-4)/2=1
		{6, 1},
		// (8-4)/2=2
		{8, 2},
		// (12-4)/2=4, exactly at cap
		{12, 4},
		// Capped at 4
		{16, 4},
		{20, 4},
	}
	for _, tt := range tests {
		got := netGraphRows(tt.h)
		if got != tt.want {
			t.Errorf("netGraphRows(%d) = %d, want %d", tt.h, got, tt.want)
		}
	}
}

func TestNetworkViewAtMinHeight(t *testing.T) {
	n := New()
	th := theme.Default()
	stats := types.NetworkStats{
		Interface:     "eth0",
		LinkUp:        true,
		IPv4:          []string{"192.168.1.1"},
		RxBytesPerSec: 1024,
		TxBytesPerSec: 512,
	}

	minH := n.SizeHint().MinH
	out := n.View(stats, 80, minH, th)
	if out == "" {
		t.Fatal("View returned empty output at MinH")
	}
	if !strings.Contains(out, "Network") {
		t.Errorf("expected box title 'Network' in output, got: %q", out)
	}
}

func TestNetworkViewFooterNotRenderedAtMinHeight(t *testing.T) {
	n := New()
	th := theme.Default()
	stats := types.NetworkStats{
		Interface: "eth0",
		LinkUp:    true,
		IPv4:      []string{"192.168.1.1"},
		RxBytes:   1024 * 1024,
		TxBytes:   512 * 1024,
	}

	// At MinH=3 (inner content rows): availableForFooter = 3 - 4 - 0 = -1 → 0. Footer suppressed.
	minH := n.SizeHint().MinH
	out := n.View(stats, 80, minH, th)
	if strings.Contains(out, "IPv4") {
		t.Errorf("footer should be suppressed at MinH=%d but found 'IPv4' in output", minH)
	}
	if strings.Contains(out, "Total:") {
		t.Errorf("footer should be suppressed at MinH=%d but found 'Total:' in output", minH)
	}
}

func TestNetworkViewFooterAppearsWithRoom(t *testing.T) {
	n := New()
	th := theme.Default()
	stats := types.NetworkStats{
		Interface: "eth0",
		LinkUp:    true,
		IPv4:      []string{"192.168.1.1"},
		RxBytes:   1024 * 1024,
		TxBytes:   512 * 1024,
	}

	// At h=14, graphH=4 (capped): availableForFooter = 14 - 4 - 8 = 2. Both footer lines fit.
	out := n.View(stats, 80, 14, th)
	if !strings.Contains(out, "Total:") {
		t.Errorf("expected 'Total:' in footer at h=14, got: %q", out)
	}
}

func TestNetworkViewFooterLineCap(t *testing.T) {
	n := New()
	th := theme.Default()
	stats := types.NetworkStats{
		Interface: "eth0",
		LinkUp:    true,
		IPv4:      []string{"192.168.1.1"},
		IPv6:      []string{"fe80::1"},
		RxBytes:   1024 * 1024,
		TxBytes:   512 * 1024,
	}

	// At h=7, graphH=(7-4)/2=1: availableForFooter = 7 - 4 - 2 = 1. Only the first footer
	// line (IPv4) fits; Total: and IPv6 are suppressed.
	out := n.View(stats, 80, 7, th)
	if strings.Contains(out, "Total:") {
		t.Errorf("'Total:' should be suppressed at h=7 (only 1 footer line fits)")
	}
	if strings.Contains(out, "IPv6") {
		t.Errorf("IPv6 should be suppressed at h=7 (only 1 footer line fits)")
	}
}
