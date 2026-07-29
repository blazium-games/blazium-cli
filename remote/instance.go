package remote

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	instanceIDAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	instanceIDLength   = 6
	portPoolStart      = 6500
	portPoolEnd        = 6599
	maxActiveInstances = 20
	maxClosedHistory   = 50
)

// GenerateInstanceID returns a 6-char id from the unambiguous alphabet.
func GenerateInstanceID() (string, error) {
	buf := make([]byte, instanceIDLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, instanceIDLength)
	for i := range buf {
		out[i] = instanceIDAlphabet[int(buf[i])%len(instanceIDAlphabet)]
	}
	return string(out), nil
}

// GenerateToken returns a 32-byte hex auth token.
func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 64)
	for i, b := range buf {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out), nil
}

// IsProcessAlive reports whether pid appears to be running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return processAlive(pid)
}

// PortInUse reports whether something is already listening on host:port.
func PortInUse(host string, port int) bool {
	if host == "" {
		host = "127.0.0.1"
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

// ClaimedPorts returns ports reserved by active instances.
func ClaimedPorts(instances []RemoteInstance) map[int]bool {
	out := make(map[int]bool)
	for _, inst := range instances {
		if inst.RemotePort > 0 {
			out[inst.RemotePort] = true
		}
		if inst.MCPPort > 0 {
			out[inst.MCPPort] = true
		}
	}
	return out
}

// AllocatePorts finds n free ports in the pool, skipping claimed and in-use ports.
func AllocatePorts(host string, n int, claimed map[int]bool) ([]int, error) {
	if claimed == nil {
		claimed = map[int]bool{}
	}
	var out []int
	for port := portPoolStart; port <= portPoolEnd && len(out) < n; port++ {
		if claimed[port] || PortInUse(host, port) {
			continue
		}
		out = append(out, port)
		claimed[port] = true
	}
	if len(out) < n {
		return nil, fmt.Errorf("could not allocate %d free ports in %d-%d", n, portPoolStart, portPoolEnd)
	}
	return out, nil
}

// UniqueInstanceID generates an id not present in active instances.
func UniqueInstanceID(active []RemoteInstance) (string, error) {
	used := map[string]bool{}
	for _, inst := range active {
		used[strings.ToUpper(inst.ID)] = true
	}
	for i := 0; i < 64; i++ {
		id, err := GenerateInstanceID()
		if err != nil {
			return "", err
		}
		if !used[id] {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique instance id")
}

// SortNewestFirst sorts instances by StartedAt descending.
func SortNewestFirst(instances []RemoteInstance) []RemoteInstance {
	out := append([]RemoteInstance(nil), instances...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}

// LatestLiveInstance returns the newest active instance, if any.
func LatestLiveInstance(instances []RemoteInstance) *RemoteInstance {
	sorted := SortNewestFirst(instances)
	if len(sorted) == 0 {
		return nil
	}
	return &sorted[0]
}

// FindLiveByID returns an active instance by id (case-insensitive).
func FindLiveByID(instances []RemoteInstance, id string) *RemoteInstance {
	want := strings.ToUpper(strings.TrimSpace(id))
	for i := range instances {
		if strings.ToUpper(instances[i].ID) == want {
			return &instances[i]
		}
	}
	return nil
}

// FindLiveByProject returns active instances for a project path.
func FindLiveByProject(instances []RemoteInstance, projectPath string) []RemoteInstance {
	want := strings.ToLower(strings.TrimSpace(projectPath))
	var out []RemoteInstance
	for _, inst := range instances {
		if strings.ToLower(inst.ProjectPath) == want {
			out = append(out, inst)
		}
	}
	return SortNewestFirst(out)
}

// RetireInstance moves an active instance into closed history.
func RetireInstance(cfg *CLIFile, inst RemoteInstance, reason string) {
	if cfg == nil {
		return
	}
	remaining := make([]RemoteInstance, 0, len(cfg.Remote.Instances))
	for _, cur := range cfg.Remote.Instances {
		if cur.ID == inst.ID && cur.PID == inst.PID {
			continue
		}
		remaining = append(remaining, cur)
	}
	cfg.Remote.Instances = remaining
	cfg.Remote.Closed = append([]ClosedInstance{{
		ProjectPath: inst.ProjectPath,
		PID:         inst.PID,
		RemotePort:  inst.RemotePort,
		RetiredID:   inst.ID,
		Reason:      reason,
		EndedAt:     time.Now().UTC(),
	}}, cfg.Remote.Closed...)
	if len(cfg.Remote.Closed) > maxClosedHistory {
		cfg.Remote.Closed = cfg.Remote.Closed[:maxClosedHistory]
	}
}

// AddActiveInstance appends an instance and caps the active list.
func AddActiveInstance(cfg *CLIFile, inst RemoteInstance) {
	if cfg == nil {
		return
	}
	cfg.Remote.Instances = append(cfg.Remote.Instances, inst)
	if len(cfg.Remote.Instances) > maxActiveInstances {
		cfg.Remote.Instances = cfg.Remote.Instances[len(cfg.Remote.Instances)-maxActiveInstances:]
	}
}

// FindRetiredID looks up a closed history entry by retired id.
func FindRetiredID(closed []ClosedInstance, id string) *ClosedInstance {
	want := strings.ToUpper(strings.TrimSpace(id))
	for i := range closed {
		if strings.ToUpper(closed[i].RetiredID) == want {
			return &closed[i]
		}
	}
	return nil
}

// ProcessExists is a thin alias used by tests.
func ProcessExists(pid int) bool {
	return IsProcessAlive(pid)
}

// Dummy use of os for platforms that need it in process_alive files.
var _ = os.ErrNotExist
