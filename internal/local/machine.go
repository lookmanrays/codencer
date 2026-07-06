package local

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const MachineVersion = 1

var hostLabelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

type MachineIdentity struct {
	Version   int       `json:"version"`
	MachineID string    `json:"machine_id"`
	Hostname  string    `json:"hostname"`
	HostLabel string    `json:"host_label"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func EnsureMachine(path string, now time.Time) (MachineIdentity, bool, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	machine, err := LoadMachine(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return MachineIdentity{}, false, err
		}
		next, genErr := NewMachineIdentity(now)
		if genErr != nil {
			return MachineIdentity{}, false, genErr
		}
		if err := SaveMachine(path, next); err != nil {
			return MachineIdentity{}, false, err
		}
		return next, true, nil
	}
	normalized, changed := normalizeMachine(machine, now)
	if changed {
		if err := SaveMachine(path, normalized); err != nil {
			return MachineIdentity{}, false, err
		}
	}
	return normalized, false, nil
}

func LoadMachine(path string) (MachineIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MachineIdentity{}, err
	}
	var machine MachineIdentity
	if err := json.Unmarshal(data, &machine); err != nil {
		return MachineIdentity{}, fmt.Errorf("decode machine identity: %w", err)
	}
	return machine, nil
}

func SaveMachine(path string, machine MachineIdentity) error {
	machine, _ = normalizeMachine(machine, time.Now().UTC())
	return writeJSONAtomic(path, machine, 0600)
}

func SetMachineHostLabel(path, label string, now time.Time) (MachineIdentity, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	label = SanitizeHostLabel(label)
	if label == "" || !hostLabelPattern.MatchString(label) {
		return MachineIdentity{}, fmt.Errorf("host label must match [a-z0-9][a-z0-9._-]{0,62}")
	}
	machine, _, err := EnsureMachine(path, now)
	if err != nil {
		return MachineIdentity{}, err
	}
	machine.HostLabel = label
	machine.UpdatedAt = now
	if err := SaveMachine(path, machine); err != nil {
		return MachineIdentity{}, err
	}
	return machine, nil
}

func NewMachineIdentity(now time.Time) (MachineIdentity, error) {
	id, err := randomMachineID()
	if err != nil {
		return MachineIdentity{}, err
	}
	hostname, _ := os.Hostname()
	label := SanitizeHostLabel(hostname)
	if label == "" {
		label = "local"
	}
	return MachineIdentity{
		Version:   MachineVersion,
		MachineID: id,
		Hostname:  hostname,
		HostLabel: label,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func SanitizeHostLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteRune(r)
		lastDash = r == '-'
	}
	label := strings.Trim(b.String(), ".-_")
	if len(label) > 63 {
		label = strings.Trim(label[:63], ".-_")
	}
	return label
}

func normalizeMachine(machine MachineIdentity, now time.Time) (MachineIdentity, bool) {
	changed := false
	if machine.Version == 0 {
		machine.Version = MachineVersion
		changed = true
	}
	if machine.MachineID == "" {
		if id, err := randomMachineID(); err == nil {
			machine.MachineID = id
			changed = true
		}
	}
	hostname, _ := os.Hostname()
	if machine.Hostname != hostname {
		machine.Hostname = hostname
		changed = true
	}
	if machine.HostLabel == "" {
		machine.HostLabel = SanitizeHostLabel(hostname)
		if machine.HostLabel == "" {
			machine.HostLabel = "local"
		}
		changed = true
	}
	if machine.OS != runtime.GOOS {
		machine.OS = runtime.GOOS
		changed = true
	}
	if machine.Arch != runtime.GOARCH {
		machine.Arch = runtime.GOARCH
		changed = true
	}
	if machine.CreatedAt.IsZero() {
		machine.CreatedAt = now
		changed = true
	}
	if machine.UpdatedAt.IsZero() || changed {
		machine.UpdatedAt = now
		changed = true
	}
	return machine, changed
}

func randomMachineID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate machine id: %w", err)
	}
	return "mach_" + hex.EncodeToString(buf), nil
}

func MachinePath(home string) string {
	return filepath.Join(home, "machine.json")
}
