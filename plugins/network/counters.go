package network

import "time"

type netDevCounters struct {
	rxBytes   uint64
	txBytes   uint64
	timestamp time.Time
}
