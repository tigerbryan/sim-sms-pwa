package device

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanSystem discovers only real cdc-wdm character devices. It does not open,
// reset, reconfigure, or otherwise mutate them.
func ScanSystem(ctx context.Context, allowedVIDs []string, controlPaths ...string) ([]Device, error) {
	_ = ctx
	selected := ""
	if len(controlPaths) > 0 {
		selected = strings.TrimSpace(controlPaths[0])
	}
	var paths []string
	if selected != "" {
		clean := filepath.Clean(selected)
		if clean != selected || filepath.Dir(clean) != "/dev" || !strings.HasPrefix(filepath.Base(clean), "cdc-wdm") {
			return nil, fmt.Errorf("invalid QMI control path %q", selected)
		}
		paths = []string{clean}
	} else {
		var err error
		paths, err = filepath.Glob("/dev/cdc-wdm*")
		if err != nil {
			return nil, err
		}
	}
	allowed := normalizeVIDs(allowedVIDs)
	var found []Device
	for _, control := range paths {
		info, err := os.Lstat(control)
		if err != nil || info.Mode()&os.ModeDevice == 0 {
			continue
		}
		d := describeDevice(control)
		if len(allowed) > 0 && !allowed[d.USBVID] {
			continue
		}
		found = append(found, d)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].ControlPath < found[j].ControlPath })
	if len(found) == 0 {
		return []Device{}, nil
	}
	return found, nil
}

func describeDevice(control string) Device {
	base := filepath.Base(control)
	sysfs, _ := filepath.EvalSymlinks(filepath.Join("/sys/class/usbmisc", base, "device"))
	if sysfs == "" {
		sysfs = filepath.Join("/sys/class/usbmisc", base, "device")
	}
	root := findUSBParent(sysfs)
	vid := readTrim(filepath.Join(root, "idVendor"))
	pid := readTrim(filepath.Join(root, "idProduct"))
	driver := ""
	if target, err := filepath.EvalSymlinks(filepath.Join(sysfs, "driver")); err == nil {
		driver = filepath.Base(target)
	}
	network := findNetworkInterface(sysfs, root)
	serial := findSerialPorts(sysfs, root)
	sum := sha256.Sum256([]byte(control + "|" + sysfs))
	return Device{
		ID:               "qmi-" + hex.EncodeToString(sum[:8]),
		ControlPath:      control,
		Driver:           driver,
		USBVID:           strings.ToLower(vid),
		USBPID:           strings.ToLower(pid),
		Manufacturer:     readTrim(filepath.Join(root, "manufacturer")),
		Product:          readTrim(filepath.Join(root, "product")),
		NetworkInterface: network,
		SerialPorts:      serial,
		SysfsPath:        sysfs,
		Status:           "available",
	}
}

func findUSBParent(start string) string {
	current := start
	for i := 0; i < 8 && current != "" && current != filepath.Dir(current); i++ {
		if _, err := os.Stat(filepath.Join(current, "idVendor")); err == nil {
			return current
		}
		current = filepath.Dir(current)
	}
	return start
}

func findNetworkInterface(sysfs, root string) string {
	for _, candidate := range []string{sysfs, filepath.Dir(sysfs), root} {
		entries, err := os.ReadDir(filepath.Join(candidate, "net"))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			return entry.Name()
		}
	}
	return ""
}

func findSerialPorts(sysfs, root string) []string {
	seen := map[string]bool{}
	for _, candidate := range []string{sysfs, filepath.Dir(sysfs), root} {
		entries, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "ttyUSB") || strings.HasPrefix(entry.Name(), "ttyACM") {
				seen["/dev/"+entry.Name()] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for port := range seen {
		out = append(out, port)
	}
	sort.Strings(out)
	return out
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func normalizeVIDs(vids []string) map[string]bool {
	out := map[string]bool{}
	for _, vid := range vids {
		value := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(vid), "0x"))
		if len(value) == 4 {
			out[value] = true
		}
	}
	return out
}

func FindByID(devices []Device, id string) (Device, error) {
	for _, d := range devices {
		if d.ID == id {
			return d, nil
		}
	}
	return Device{}, fmt.Errorf("%w: %s", ErrNoDevice, id)
}
