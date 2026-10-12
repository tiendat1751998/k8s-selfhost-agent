package hardware

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ProtectedOOMScore defines the Linux kernel OOM killer exemption score (-999).
const ProtectedOOMScore = -999

// DefaultProtectedDaemons lists critical system daemons that must not be OOM killed.
var DefaultProtectedDaemons = []string{"kubelet", "containerd", "dockerd"}

// OOMShield manages verifying and setting /proc/$PID/oom_score_adj for critical system daemons.
type OOMShield struct {
	procRoot string
}

// NewOOMShield constructs an OOMShield targeting host /proc filesystem.
func NewOOMShield() *OOMShield {
	return &OOMShield{procRoot: "/proc"}
}

// NewOOMShieldWithRoot constructs an OOMShield with a custom proc root directory (used in tests).
func NewOOMShieldWithRoot(procRoot string) *OOMShield {
	return &OOMShield{procRoot: procRoot}
}

// isProcSupported returns whether the proc filesystem exists at the configured path.
func (s *OOMShield) isProcSupported() bool {
	info, err := os.Stat(s.procRoot)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FindDaemonPIDs searches for all PIDs matching a given daemon executable name.
func (s *OOMShield) FindDaemonPIDs(daemonName string) ([]int, error) {
	if !s.isProcSupported() {
		if runtime.GOOS != "linux" {
			return nil, nil // Portable fallback for non-Linux platforms
		}
		return nil, fmt.Errorf("proc directory %s inaccessible", s.procRoot)
	}

	entries, err := os.ReadDir(s.procRoot)
	if err != nil {
		return nil, fmt.Errorf("reading proc root %s: %w", s.procRoot, err)
	}

	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // Not a PID directory
		}

		commPath := filepath.Join(s.procRoot, entry.Name(), "comm")
		commBytes, err := os.ReadFile(commPath)
		if err != nil {
			continue
		}

		comm := strings.TrimSpace(string(commBytes))
		if strings.EqualFold(comm, daemonName) {
			pids = append(pids, pid)
			continue
		}

		// Also check cmdline for process matching if comm is truncated
		cmdlinePath := filepath.Join(s.procRoot, entry.Name(), "cmdline")
		cmdlineBytes, err := os.ReadFile(cmdlinePath)
		if err == nil && len(cmdlineBytes) > 0 {
			args := bytes.Split(cmdlineBytes, []byte{0})
			if len(args) > 0 {
				execName := filepath.Base(string(args[0]))
				if strings.EqualFold(execName, daemonName) {
					pids = append(pids, pid)
				}
			}
		}
	}

	return pids, nil
}

// VerifyDaemonOOMScore checks the current oom_score_adj for the specified daemon.
func (s *OOMShield) VerifyDaemonOOMScore(daemonName string) (int, bool, error) {
	if !s.isProcSupported() && runtime.GOOS != "linux" {
		// Portable non-Linux fallback: report protected
		return ProtectedOOMScore, true, nil
	}

	pids, err := s.FindDaemonPIDs(daemonName)
	if err != nil {
		return 0, false, err
	}
	if len(pids) == 0 {
		return 0, false, fmt.Errorf("daemon %s not found in running processes", daemonName)
	}

	// Verify primary PID
	pid := pids[0]
	scorePath := filepath.Join(s.procRoot, strconv.Itoa(pid), "oom_score_adj")
	content, err := os.ReadFile(scorePath)
	if err != nil {
		return 0, false, fmt.Errorf("reading oom_score_adj for PID %d: %w", pid, err)
	}

	scoreStr := strings.TrimSpace(string(content))
	score, err := strconv.Atoi(scoreStr)
	if err != nil {
		return 0, false, fmt.Errorf("parsing oom_score_adj %q for PID %d: %w", scoreStr, pid, err)
	}

	return score, score == ProtectedOOMScore, nil
}

// SetPIDScore writes the target score to /proc/$PID/oom_score_adj.
func (s *OOMShield) SetPIDScore(pid int, score int) error {
	scorePath := filepath.Join(s.procRoot, strconv.Itoa(pid), "oom_score_adj")
	scoreStr := fmt.Sprintf("%d\n", score)
	if err := os.WriteFile(scorePath, []byte(scoreStr), 0644); err != nil {
		return fmt.Errorf("writing oom_score_adj for PID %d: %w", pid, err)
	}
	return nil
}

// ShieldDaemon sets /proc/$PID/oom_score_adj to -999 for all instances of a daemon.
func (s *OOMShield) ShieldDaemon(daemonName string) error {
	if !s.isProcSupported() && runtime.GOOS != "linux" {
		return nil
	}

	pids, err := s.FindDaemonPIDs(daemonName)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return fmt.Errorf("daemon %s not found to shield", daemonName)
	}

	for _, pid := range pids {
		if err := s.SetPIDScore(pid, ProtectedOOMScore); err != nil {
			return err
		}
	}
	return nil
}

// ShieldProtectedDaemons sets -999 for kubelet, containerd, and dockerd.
func (s *OOMShield) ShieldProtectedDaemons() (map[string]bool, error) {
	results := make(map[string]bool)
	for _, daemon := range DefaultProtectedDaemons {
		score, isShielded, err := s.VerifyDaemonOOMScore(daemon)
		if err != nil {
			// Daemon may not be installed on this host; skip if not found
			if strings.Contains(err.Error(), "not found") {
				continue
			}
			return results, err
		}

		if !isShielded {
			if err := s.ShieldDaemon(daemon); err != nil {
				return results, fmt.Errorf("shielding daemon %s: %w", daemon, err)
			}
			results[daemon] = true
		} else {
			results[daemon] = (score == ProtectedOOMScore)
		}
	}
	return results, nil
}
