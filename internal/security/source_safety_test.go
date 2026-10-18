package security

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func productionGoSources(t *testing.T) map[string]string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("find source root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	result := map[string]string{}
	for _, tree := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[path] = string(content)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func requireNoProductionToken(t *testing.T, tokens ...string) {
	t.Helper()
	for path, content := range productionGoSources(t) {
		for _, token := range tokens {
			if strings.Contains(content, token) {
				t.Fatalf("unsafe production token %q in %s", token, path)
			}
		}
	}
}

func TestNoDataSessionMethodsRegistered(t *testing.T) {
	requireNoProductionToken(t,
		"StartNetwork", "SetProfile", "CreateProfile", "ModifyProfile", "SetAutoconnect", "SetAPN",
	)
}

func TestNoWDSStartCalls(t *testing.T) {
	requireNoProductionToken(t, "WDSStart", "wds-start-network", "qmi-network start", "uqmi")
}

func TestNoDialRoutes(t *testing.T) {
	requireNoProductionToken(t, "ip route add default", "dhclient", "udhcpc", "MASQUERADE")
}

func TestNoAPNMethods(t *testing.T) {
	requireNoProductionToken(t, "CGDCONT", "APN", "PDP")
}

func TestNoATExecutor(t *testing.T) {
	requireNoProductionToken(t, "AT+", "os/exec", "exec.Command")
}

func TestSendingSMSDisabled(t *testing.T) {
	requireNoProductionToken(t,
		"/api/v1/sms/send", "SendRawMessage", "SendFromStorage", "RawWriteMessage", "DeleteMessage", "ModifyMessageTag", "SetRoutes", "SendAck", "SetSMSC", "usb_modeswitch",
	)
}
