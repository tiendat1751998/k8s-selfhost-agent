//go:build !windows && !linux
package metrics

import "context"

type platformHostProber struct{}

func newPlatformHostProber() HostProber {
	return &platformHostProber{}
}

func (p *platformHostProber) ProbeHost(ctx context.Context) HostTelemetry {
	return probeFallbackHostTelemetry()
}
