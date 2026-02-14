package types

// GPUInfo holds data for a single GPU device.
type GPUInfo struct {
	Index          int     // Device index
	Name           string  // Device name (e.g. "NVIDIA GeForce RTX 3080")
	UtilizationPct float64 // GPU core utilization 0-100
	MemoryUsed     uint64  // Bytes used
	MemoryTotal    uint64  // Bytes total
	MemoryPct      float64 // Memory utilization 0-100
	TemperatureC   float64 // Temperature in Celsius; -1 if unavailable
	ClockCoreMHz   int     // Core clock in MHz; 0 if unavailable
	ClockMemMHz    int     // Memory clock in MHz; 0 if unavailable
	PowerWatts     float64 // Power draw in watts; -1 if unavailable
	PowerCapWatts  float64 // Power cap in watts; -1 if unavailable
	PCIeTxMBps     float64 // PCIe TX throughput MB/s; -1 if unavailable
	PCIeRxMBps     float64 // PCIe RX throughput MB/s; -1 if unavailable
	EncoderPct     float64 // Encoder utilization 0-100; -1 if unavailable
	DecoderPct     float64 // Decoder utilization 0-100; -1 if unavailable
}

// GPUStats is the snapshot returned by the GPU collector.
type GPUStats struct {
	GPUs []GPUInfo
	// FeatureFlags indicates which optional data fields are populated.
	HasTemp    bool
	HasPower   bool
	HasPCIe    bool
	HasEncoder bool
	// Error is set when GPUs are detected but data collection partially failed.
	// Individual GPUs may still have valid data.
	Error string
}
