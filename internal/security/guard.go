package security

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DeviceNetwork struct {
	DeviceID         string
	NetworkInterface string
	WDSStatus        string
}

type Finding struct {
	Code      string `json:"code"`
	Interface string `json:"interface,omitempty"`
	Detail    string `json:"detail"`
}

type GuardState struct {
	State     string    `json:"state"`
	CheckedAt time.Time `json:"checked_at"`
	WDSStatus string    `json:"wds_status"`
	Findings  []Finding `json:"findings"`
}

func (s GuardState) HasFinding(code string) bool {
	for _, finding := range s.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

// Check performs observation only. It never edits IP addresses, routes, DNS,
// processes, or any modem setting.
func Check(ctx context.Context, devices []DeviceNetwork) GuardState {
	_ = ctx
	state := GuardState{State: "safe", CheckedAt: time.Now().UTC()}
	interfaces := map[string]bool{}
	for _, d := range devices {
		if d.NetworkInterface != "" {
			interfaces[d.NetworkInterface] = true
		}
		if d.WDSStatus != "" && d.WDSStatus != "disconnected" && d.WDSStatus != "unknown" {
			state.Findings = append(state.Findings, Finding{Code: "WDSPacketServiceActive", Interface: d.NetworkInterface, Detail: "WDS packet service reports " + d.WDSStatus})
		}
		if d.WDSStatus != "" {
			state.WDSStatus = d.WDSStatus
		}
	}
	if len(interfaces) == 0 {
		if len(state.Findings) > 0 {
			state.State = "critical"
		}
		return state
	}
	for name := range interfaces {
		if hasGlobalAddress(name) {
			state.Findings = append(state.Findings, Finding{Code: "CellularGlobalIP", Interface: name, Detail: "cellular interface has a global IP address"})
		}
		if defaultRouteUses(name) {
			state.Findings = append(state.Findings, Finding{Code: "CellularDefaultRoute", Interface: name, Detail: "default route uses cellular interface"})
		}
	}
	if qmiNetworkProcessPresent() {
		state.Findings = append(state.Findings, Finding{Code: "QMINetworkProcess", Detail: "qmi-network style process detected"})
	}
	if len(state.Findings) > 0 {
		state.State = "critical"
	}
	return state
}

func hasGlobalAddress(name string) bool {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.IsGlobalUnicast() {
			return true
		}
	}
	return false
}

func defaultRouteUses(name string) bool {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return false
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 && fields[0] == name && fields[1] == "00000000" && fields[2] != "00000000" {
			return true
		}
	}
	return false
}

func qmiNetworkProcessPresent() bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() || !allDigits(entry.Name()) {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		text := strings.ToLower(string(cmdline))
		if strings.Contains(text, "qmi-network") || strings.Contains(text, "quectel-cm") {
			return true
		}
	}
	return false
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
