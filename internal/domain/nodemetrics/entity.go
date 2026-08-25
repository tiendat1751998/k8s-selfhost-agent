// Package nodemetrics provides domain entities and repository ports for per-node historical time-series telemetry.
package nodemetrics

import "time"

// DiskIOStats represents real-time I/O performance and latency metrics for a block device.
type DiskIOStats struct {
	DeviceName        string  `json:"device_name"`         // e.g. "sda", "nvme0n1", "dm-0"
	ReadBytesPerSec   int64   `json:"read_bytes_per_sec"`  // Read throughput (Bytes/s)
	WriteBytesPerSec  int64   `json:"write_bytes_per_sec"` // Write throughput (Bytes/s)
	ReadIOPS          float64 `json:"read_iops"`           // Read operations per second
	WriteIOPS         float64 `json:"write_iops"`          // Write operations per second
	AvgWaitMs         float64 `json:"avg_wait_ms"`         // await (Average I/O wait time in ms)
	AvgRequestSizeKB  float64 `json:"avg_req_size_kb"`     // avgrq-sz (Average request size in KB)
	CurrentQueueDepth int64   `json:"current_queue_depth"` // in_flight requests currently queued
	IoUtilizationPct  float64 `json:"io_utilization_pct"`  // %util (Disk busy time %)
	IsRootDevice      bool    `json:"is_root_device"`      // true for sda, nvme0n1, false for partitions sda1
}

// NodeMetricRollup represents a consolidated hardware and workload telemetry sample for a single compute node.
type NodeMetricRollup struct {
	ID                   string        `json:"id"`
	TenantID             string        `json:"tenant_id"`
	NodeID               string        `json:"node_id"`
	NodeName             string        `json:"node_name"`
	CPUPercent           float64       `json:"cpu_percent"`
	CPUPeak              float64       `json:"cpu_peak"`
	MemUsedBytes         int64         `json:"mem_used_bytes"`
	MemTotalBytes        int64         `json:"mem_total_bytes"`
	MemPercent           float64       `json:"mem_percent"`
	DiskUsedBytes        int64         `json:"disk_used_bytes"`
	DiskTotalBytes       int64         `json:"disk_total_bytes"`
	DiskPercent          float64       `json:"disk_percent"`
	DiskReadBytesPerSec  int64         `json:"disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec int64         `json:"disk_write_bytes_per_sec"`
	DiskReadIOPS         float64       `json:"disk_read_iops"`
	DiskWriteIOPS        float64       `json:"disk_write_iops"`
	DiskAvgAwaitMs       float64       `json:"disk_avg_await_ms"`
	DiskMaxIoUtilPct     float64       `json:"disk_max_io_util_pct"`
	DiskDevices          []DiskIOStats `json:"disk_devices,omitempty"`
	RxBytesPerSec        int64         `json:"rx_bytes_per_sec"`
	TxBytesPerSec        int64         `json:"tx_bytes_per_sec"`
	ProcessCount         int           `json:"process_count"`
	ContainerCount       int           `json:"container_count"`
	Status               string        `json:"status"`     // "online" | "degraded" | "offline"
	Resolution           string        `json:"resolution"` // "1m" | "1h" | "1d"
	RecordedAt           time.Time     `json:"recorded_at"`
}

// NodeHistoryQuery contains parameters for fetching node historical telemetry.
type NodeHistoryQuery struct {
	TenantID   string
	NodeID     string
	Resolution string
	StartTime  time.Time
	EndTime    time.Time
	Limit      int
}

// NodeHistoricalSummary provides high-level summary KPIs over the requested window.
type NodeHistoricalSummary struct {
	NodeID         string    `json:"node_id"`
	NodeName       string    `json:"node_name"`
	AvgCPUPercent  float64   `json:"avg_cpu_percent"`
	PeakCPUPercent float64   `json:"peak_cpu_percent"`
	AvgMemPercent  float64   `json:"avg_mem_percent"`
	PeakMemPercent float64   `json:"peak_mem_percent"`
	PeakRxBytesSec int64     `json:"peak_rx_bytes_sec"`
	PeakTxBytesSec int64     `json:"peak_tx_bytes_sec"`
	UptimePercent  float64   `json:"uptime_percent"`
	OfflineCount   int       `json:"offline_count"`
	TotalSamples   int       `json:"total_samples"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
}
