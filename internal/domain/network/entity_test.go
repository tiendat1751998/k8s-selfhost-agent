package network_test

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/network"
)

func TestCalculateTotalAllocatable(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		expected int
	}{
		{
			name:     "standard /24 ipv4",
			cidr:     "10.244.1.0/24",
			expected: 254,
		},
		{
			name:     "smaller /26 ipv4",
			cidr:     "192.168.0.0/26",
			expected: 62,
		},
		{
			name:     "empty cidr fallback",
			cidr:     "",
			expected: 254,
		},
		{
			name:     "invalid cidr fallback",
			cidr:     "invalid-cidr",
			expected: 254,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := network.CalculateTotalAllocatable(tt.cidr)
			if got != tt.expected {
				t.Errorf("CalculateTotalAllocatable(%q) = %d, want %d", tt.cidr, got, tt.expected)
			}
		})
	}
}

func TestNewIPAMUtilization_StatusTransitions(t *testing.T) {
	tests := []struct {
		name            string
		nodeName        string
		podCIDR         string
		total           int
		allocated       int
		activePods      int
		expectedLeaked  int
		expectedStatus  string
		expectedUsageMin float64
	}{
		{
			name:           "healthy when usage is low and leaks are low",
			nodeName:       "node-1",
			podCIDR:        "10.244.1.0/24",
			total:          254,
			allocated:      50,
			activePods:     45,
			expectedLeaked: 5,
			expectedStatus: network.StatusHealthy,
		},
		{
			name:           "warning when leaked ips exceed 20",
			nodeName:       "node-2",
			podCIDR:        "10.244.2.0/24",
			total:          254,
			allocated:      100,
			activePods:     75,
			expectedLeaked: 25,
			expectedStatus: network.StatusWarning,
		},
		{
			name:           "warning when usage exceeds 80 percent",
			nodeName:       "node-3",
			podCIDR:        "10.244.3.0/24",
			total:          200,
			allocated:      165,
			activePods:     155,
			expectedLeaked: 10,
			expectedStatus: network.StatusWarning,
		},
		{
			name:           "exhausted when usage exceeds 95 percent",
			nodeName:       "node-4",
			podCIDR:        "10.244.4.0/24",
			total:          200,
			allocated:      192,
			activePods:     190,
			expectedLeaked: 2,
			expectedStatus: network.StatusExhausted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			util := network.NewIPAMUtilization(tt.nodeName, tt.podCIDR, tt.total, tt.allocated, tt.activePods)
			if util.LeakedOrphanIPs != tt.expectedLeaked {
				t.Errorf("LeakedOrphanIPs = %d, want %d", util.LeakedOrphanIPs, tt.expectedLeaked)
			}
			if util.Status != tt.expectedStatus {
				t.Errorf("Status = %s, want %s (usage=%.2f%%, leaked=%d)", util.Status, tt.expectedStatus, util.UsagePercentage, util.LeakedOrphanIPs)
			}
		})
	}
}
