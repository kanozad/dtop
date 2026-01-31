package types

import "time"

// NetworkStats captures network interface usage and rates.
type NetworkStats struct {
	Interface string
	LinkUp    bool
	IPv4      []string
	IPv6      []string

	RxBytes uint64
	TxBytes uint64

	RxBytesPerSec float64
	TxBytesPerSec float64

	PeakRxBytesPerSec float64
	PeakTxBytesPerSec float64

	Timestamp time.Time

	// History buffers sized to the current viewport width (trimmed/padded on resize).
	RxHistory []float64
	TxHistory []float64
}
