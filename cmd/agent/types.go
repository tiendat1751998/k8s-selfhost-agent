package main

import "time"

// CPUMetrics holds CPU core count and usage percentage.
type CPUMetrics struct {
	Count        int     `json:"count"`
	UsagePercent float64 `json:"usage_percent"`
}

// MemoryMetrics holds memory capacity and usage in bytes.
type MemoryMetrics struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	AvailableBytes int64   `json:"available_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

// DiskMetrics holds filesystem disk mount usage metrics.
type DiskMetrics struct {
	MountPoint   string  `json:"mount_point"`
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	Filesystem   string  `json:"filesystem"`
}

// NetworkInterfaceMetrics holds network rates for an interface.
type NetworkInterfaceMetrics struct {
	Name          string `json:"name"`
	RxBytesPerSec int64  `json:"rx_bytes_per_sec"`
	TxBytesPerSec int64  `json:"tx_bytes_per_sec"`
}

// NetworkMetrics holds per-interface and total throughput metrics.
type NetworkMetrics struct {
	Interfaces         []NetworkInterfaceMetrics `json:"interfaces"`
	TotalRxBytesPerSec int64                     `json:"total_rx_bytes_per_sec"`
	TotalTxBytesPerSec int64                     `json:"total_tx_bytes_per_sec"`
}

// ProcessMetric holds resource consumption details for a single OS process.
type ProcessMetric struct {
	PID              int     `json:"pid"`
	Name             string  `json:"name"`
	CommandLine      string  `json:"command_line"`
	User             string  `json:"user"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryBytes      int64   `json:"memory_bytes"`
	MemoryPercent    float64 `json:"memory_percent"`
	ReadBytesPerSec  int64   `json:"read_bytes_per_sec"`
	WriteBytesPerSec int64   `json:"write_bytes_per_sec"`
	State            string  `json:"state"`
}

// MetricsResponse is the JSON schema returned by GET /metrics.
type MetricsResponse struct {
	Hostname      string          `json:"hostname"`
	OS            string          `json:"os"`
	Arch          string          `json:"arch"`
	UptimeSeconds int64           `json:"uptime_seconds"`
	LoadAverage   [3]float64      `json:"load_average"`
	CPU           CPUMetrics      `json:"cpu"`
	Memory        MemoryMetrics   `json:"memory"`
	Disks         []DiskMetrics   `json:"disks"`
	Network       NetworkMetrics  `json:"network"`
	Processes     int             `json:"processes"`
	TopProcesses  []ProcessMetric `json:"top_processes"`
	CollectedAt   time.Time       `json:"collected_at"`
}

// HealthResponse is the JSON schema returned by GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}
