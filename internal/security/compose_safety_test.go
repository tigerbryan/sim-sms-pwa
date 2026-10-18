package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func deploymentCompose(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("find source root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

func requireLeastPrivilegeCompose(t *testing.T, name string) string {
	t.Helper()
	text := deploymentCompose(t, name)
	for _, required := range []string{
		"user: \"65532:65532\"", "cap_add: []", "cap_drop:", "- ALL",
		"read_only: true", "no-new-privileges:true", "privileged: false", "network_mode: bridge",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("%s missing required safety setting %q", name, required)
		}
	}
	for _, forbidden := range []string{"user: \"0:0\"", "/dev:/dev", "ttyUSB", "ttyACM", "docker.sock", "c *:*", ":* rw", " rwm"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s contains forbidden setting %q", name, forbidden)
		}
	}
	return text
}

func TestNoDeviceComposeCannotOpenModem(t *testing.T) {
	text := requireLeastPrivilegeCompose(t, "compose.no-device.yaml")
	for _, forbidden := range []string{"QMI_WEB_DEVICE", "QMI_WEB_BACKEND: qmi", "device_cgroup_rules:", "devices:"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("no-device compose contains modem access setting %q", forbidden)
		}
	}
}

func TestHardwareComposeUsesOneExactQMINode(t *testing.T) {
	text := requireLeastPrivilegeCompose(t, "compose.hardware.yaml")
	for _, required := range []string{"QMI_WEB_DEVICE", "type: bind", "device_cgroup_rules:", " rw\""} {
		if !strings.Contains(text, required) {
			t.Fatalf("hardware compose missing exact-device setting %q", required)
		}
	}
	for _, forbidden := range []string{"devices:", "build:", "QMI_WEB_BACKEND: mock"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("hardware compose contains forbidden setting %q", forbidden)
		}
	}
}
