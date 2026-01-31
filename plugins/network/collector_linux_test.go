//go:build linux

package network

import (
	"strings"
	"testing"
	"time"
)

func TestParseNetDev(t *testing.T) {
	input := `
Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  lo: 123 0 0 0 0 0 0 0 456 0 0 0 0 0 0 0
eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0
`
	now := time.Now()
	counters, err := parseNetDev(strings.NewReader(input), now)
	if err != nil {
		t.Fatalf("parseNetDev error: %v", err)
	}
	if len(counters) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(counters))
	}
	eth0 := counters["eth0"]
	if eth0.rxBytes != 1000 || eth0.txBytes != 2000 {
		t.Fatalf("unexpected eth0 counters: rx=%d tx=%d", eth0.rxBytes, eth0.txBytes)
	}
}

func TestSelectInterface(t *testing.T) {
	counters := map[string]netDevCounters{
		"lo":   {rxBytes: 1, txBytes: 1},
		"eth0": {rxBytes: 2, txBytes: 2},
	}
	iface, err := selectInterface("", counters)
	if err != nil {
		t.Fatalf("selectInterface error: %v", err)
	}
	if iface != "eth0" {
		t.Fatalf("expected eth0, got %s", iface)
	}
	if _, err := selectInterface("missing0", counters); err == nil {
		t.Fatalf("expected error for missing interface")
	}
}

func TestRateFromCounters(t *testing.T) {
	start := time.Now()
	prev := netDevCounters{rxBytes: 1000, txBytes: 2000, timestamp: start}
	current := netDevCounters{rxBytes: 3000, txBytes: 2600, timestamp: start.Add(2 * time.Second)}
	rxRate, txRate := rateFromCounters(prev, current)
	if rxRate != 1000 {
		t.Fatalf("expected rx 1000 B/s, got %v", rxRate)
	}
	if txRate != 300 {
		t.Fatalf("expected tx 300 B/s, got %v", txRate)
	}
}
