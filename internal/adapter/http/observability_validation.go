package http

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/netguard"
)

// ObservabilityOption configures an ObservabilityHandler.
type ObservabilityOption func(*ObservabilityHandler)

// WithComputeHostRepo sets the compute host repository on the handler.
func WithComputeHostRepo(repo docker.ComputeHostRepository) ObservabilityOption {
	return func(h *ObservabilityHandler) {
		h.computeHostRepo = repo
	}
}

// SetComputeHostRepo dynamically configures the compute host repository.
func (h *ObservabilityHandler) SetComputeHostRepo(repo docker.ComputeHostRepository) {
	h.computeHostRepo = repo
}

// RegisterSLORoutes mounts top-level SLO endpoints on a router (e.g. /slo and /slos).
func (h *ObservabilityHandler) RegisterSLORoutes(r chi.Router) {
	r.Get("/", h.ListSLOSnapshots)
	r.Post("/", h.CreateSLODefinition)
	r.Get("/definitions", h.ListSLODefinitions)
	r.Post("/definitions", h.CreateSLODefinition)
	r.Get("/definitions/{id}", h.GetSLODefinition)
	r.Put("/definitions/{id}", h.UpdateSLODefinition)
	r.Delete("/definitions/{id}", h.DeleteSLODefinition)
	r.Get("/{id}", h.GetSLODefinition)
	r.Put("/{id}", h.UpdateSLODefinition)
	r.Delete("/{id}", h.DeleteSLODefinition)
	r.Post("/{id}/trigger-alert", h.TriggerBurnAlert)
}

// validateCustomSLOQueryURL validates user-supplied custom query URLs to prevent SSRF while allowlisting registered fleet nodes.
func (h *ObservabilityHandler) validateCustomSLOQueryURL(ctx context.Context, query string) error {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	isHTTP := strings.HasPrefix(strings.ToLower(q), "http://") || strings.HasPrefix(strings.ToLower(q), "https://")
	isTCP := strings.HasPrefix(strings.ToLower(q), "tcp://")
	if !isHTTP && !isTCP {
		return nil
	}

	var host string
	if isHTTP {
		u, err := url.Parse(q)
		if err != nil {
			return fmt.Errorf("invalid query URL: %w", err)
		}
		host = u.Hostname()
	} else {
		addr := strings.TrimPrefix(q, "tcp://")
		var portStr string
		var err error
		host, portStr, err = net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("invalid tcp target '%s', expected host:port: %w", addr, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("invalid tcp port '%s'", portStr)
		}
	}

	if host == "" {
		return fmt.Errorf("query host cannot be empty")
	}

	// 1. Strict hard blocks: loopback and link-local / cloud metadata are NEVER allowed
	if isBlockedHostOrIP(host) {
		return fmt.Errorf("query host is not permitted: %s", host)
	}

	// 2. Allowlist registered fleet nodes
	if h.computeHostRepo != nil && h.isFleetNode(ctx, host) {
		return nil
	}

	// 3. Fallback to Netguard SSRF validation for external addresses
	g := netguard.New(netguard.WithAllowPrivate(false))

	if isHTTP {
		u, err := g.ValidateURL(q, "http", "https")
		if err != nil {
			return fmt.Errorf("invalid query URL: %w", err)
		}
		validatedHost := u.Hostname()
		if ip := net.ParseIP(validatedHost); ip != nil {
			if err := g.ValidateIP(ip); err != nil {
				return fmt.Errorf("query URL host is not permitted: %w", err)
			}
		} else {
			ips, err := net.LookupIP(validatedHost)
			if err != nil {
				return fmt.Errorf("resolving query host %s: %w", validatedHost, err)
			}
			if len(ips) == 0 {
				return fmt.Errorf("no IP address found for %s", validatedHost)
			}
			for _, ip := range ips {
				if err := g.ValidateIP(ip); err != nil {
					return fmt.Errorf("query host %s resolves to restricted address: %w", validatedHost, err)
				}
			}
		}
		return nil
	}

	// TCP validation
	if ip := net.ParseIP(host); ip != nil {
		if err := g.ValidateIP(ip); err != nil {
			return fmt.Errorf("query tcp host is not permitted: %w", err)
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("resolving tcp host %s: %w", host, err)
		}
		if len(ips) == 0 {
			return fmt.Errorf("no IP address found for %s", host)
		}
		for _, ip := range ips {
			if err := g.ValidateIP(ip); err != nil {
				return fmt.Errorf("tcp host %s resolves to restricted address: %w", host, err)
			}
		}
	}

	return nil
}

// isBlockedHostOrIP checks for loopback, cloud metadata, and unspecified addresses.
func isBlockedHostOrIP(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// AWS / GCP / Azure metadata service
	if ip.String() == "169.254.169.254" {
		return true
	}
	return false
}

// isFleetNode returns true if hostOrIP matches a registered compute host in the database.
func (h *ObservabilityHandler) isFleetNode(ctx context.Context, hostOrIP string) bool {
	if h.computeHostRepo == nil || strings.TrimSpace(hostOrIP) == "" {
		return false
	}

	hosts, err := h.computeHostRepo.List(ctx, "")
	if err != nil || len(hosts) == 0 {
		return false
	}

	target := strings.TrimSpace(hostOrIP)
	for _, host := range hosts {
		if strings.EqualFold(host.Name, target) {
			return true
		}
		if host.Endpoint != "" {
			u, err := url.Parse(host.Endpoint)
			if err == nil && strings.EqualFold(u.Hostname(), target) {
				return true
			}
			hostEndpointOnly := host.Endpoint
			if strings.Contains(hostEndpointOnly, "://") {
				parts := strings.Split(hostEndpointOnly, "://")
				if len(parts) > 1 {
					hostEndpointOnly = parts[1]
				}
			}
			if h, _, err := net.SplitHostPort(hostEndpointOnly); err == nil && strings.EqualFold(h, target) {
				return true
			}
		}
	}

	return false
}
