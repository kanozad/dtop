//go:build linux

package network

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kanozad/dtop/pkg/types"
)

func readNetworkStats(prev map[string]netDevCounters, cfg Config) (types.NetworkStats, map[string]netDevCounters, error) {
	counters, err := readNetDev("/proc/net/dev")
	if err != nil {
		return types.NetworkStats{}, prev, err
	}

	ifaceName, err := selectInterface(cfg.Interface, counters)
	if err != nil {
		return types.NetworkStats{}, prev, err
	}
	current := counters[ifaceName]

	stats := types.NetworkStats{
		Interface: ifaceName,
		RxBytes:   current.rxBytes,
		TxBytes:   current.txBytes,
		Timestamp: current.timestamp,
	}

	if prev != nil {
		if prevCounters, ok := prev[ifaceName]; ok {
			stats.RxBytesPerSec, stats.TxBytesPerSec = rateFromCounters(prevCounters, current)
		}
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err == nil {
		stats.LinkUp = iface.Flags&net.FlagUp != 0
		stats.IPv4, stats.IPv6 = interfaceAddrs(iface)
	}

	return stats, counters, nil
}

func readNetDev(path string) (map[string]netDevCounters, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parseNetDev(file, time.Now())
}

func parseNetDev(r io.Reader, now time.Time) (map[string]netDevCounters, error) {
	counters := map[string]netDevCounters{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		rxBytes, err1 := strconv.ParseUint(fields[0], 10, 64)
		txBytes, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		counters[name] = netDevCounters{
			rxBytes:   rxBytes,
			txBytes:   txBytes,
			timestamp: now,
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(counters) == 0 {
		return nil, errors.New("no network counters found")
	}
	return counters, nil
}

func selectInterface(preferred string, counters map[string]netDevCounters) (string, error) {
	if len(counters) == 0 {
		return "", errors.New("no network interfaces found")
	}
	if preferred != "" {
		if _, ok := counters[preferred]; ok {
			return preferred, nil
		}
		return "", fmt.Errorf("interface %q not found", preferred)
	}
	names := make([]string, 0, len(counters))
	for name := range counters {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if name != "lo" {
			return name, nil
		}
	}
	return names[0], nil
}

func rateFromCounters(prev, current netDevCounters) (float64, float64) {
	elapsed := current.timestamp.Sub(prev.timestamp).Seconds()
	if elapsed <= 0 {
		return 0, 0
	}
	rxDelta := float64(current.rxBytes) - float64(prev.rxBytes)
	txDelta := float64(current.txBytes) - float64(prev.txBytes)
	if rxDelta < 0 {
		rxDelta = 0
	}
	if txDelta < 0 {
		txDelta = 0
	}
	return rxDelta / elapsed, txDelta / elapsed
}

func interfaceAddrs(iface *net.Interface) ([]string, []string) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil
	}
	ipv4 := []string{}
	ipv6 := []string{}
	for _, addr := range addrs {
		ip := extractIP(addr)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			ipv4 = append(ipv4, ip.String())
			continue
		}
		ipv6 = append(ipv6, ip.String())
	}
	sort.Strings(ipv4)
	sort.Strings(ipv6)
	return ipv4, ipv6
}

func extractIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}
