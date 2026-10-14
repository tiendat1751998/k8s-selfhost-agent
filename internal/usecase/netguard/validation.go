package netguard

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/datdt/k8sselfhost/internal/domain/netguard"
)

// ifaceNameRegex enforces Linux IFNAMSIZ rules (letters, numbers, underscores, dots, colons, hyphens).
var ifaceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.:-]+$`)

// ValidateInterfaceName validates interface names per Linux IFNAMSIZ constraints (1..15 chars,
// allowed characters only, no leading hyphen) to prevent argument/flag injection in iptables/ip CLI calls.
func ValidateInterfaceName(iface string) error {
	trimmed := strings.TrimSpace(iface)
	if trimmed == "" {
		return netguard.ErrInvalidInterface
	}
	if len(trimmed) > 15 {
		return fmt.Errorf("%w: interface name exceeds 15 characters (Linux IFNAMSIZ)", netguard.ErrInvalidInterface)
	}
	if strings.HasPrefix(trimmed, "-") {
		return fmt.Errorf("%w: interface name cannot start with '-'", netguard.ErrInvalidInterface)
	}
	if !ifaceNameRegex.MatchString(trimmed) {
		return fmt.Errorf("%w: interface name contains invalid characters", netguard.ErrInvalidInterface)
	}
	return nil
}
