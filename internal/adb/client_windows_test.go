package adb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceParsingRetainsAuthorizationAndTransport(t *testing.T) {
	devices := ParseDevices("List of devices attached\nsynthetic-one device model:Synthetic_Phone transport_id:7\nsynthetic-two unauthorized transport_id:8\n")
	if len(devices) != 2 || devices[0].Model != "Synthetic Phone" || devices[0].Transport != "7" || devices[1].State != "unauthorized" {
		t.Fatalf("unexpected parse: %#v", devices)
	}
}

func TestWindowsUnknownPathRequiresUSBOnlySerialMatch(t *testing.T) {
	for _, test := range []struct {
		name, path, selected, usb string
		err                       error
		want                      bool
	}{
		{"windows usb", "unknown", "synthetic-phone", "synthetic-phone\n", nil, true},
		{"different network device", "unknown", "192.0.2.5:5555", "synthetic-phone", nil, false},
		{"ambiguous USB", "unknown", "synthetic-phone", "", errors.New("multiple USB devices"), false},
		{"missing serial", "unknown", "", "", nil, false},
		{"reported USB", "usb:1-2", "synthetic-phone", "", nil, true},
		{"network path", "192.0.2.5:5555", "network-phone", "network-phone", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := usbSelection(test.path, test.selected, func() (string, error) { return test.usb, test.err })
			if got != test.want {
				t.Fatalf("USB transport accepted=%v, want %v", got, test.want)
			}
		})
	}
}

func TestInstalledAPKPathRejectsAmbiguityAndShellSyntax(t *testing.T) {
	for _, test := range []struct {
		input string
		want  bool
	}{
		{"package:/data/app/~~synthetic==/dev.usbreverse.lab-example==/base.apk\n", true},
		{"package:/data/app/example/base.apk;id", false},
		{"package:/data/app/example/$(id)/base.apk", false},
		{"package:/data/app/../example/base.apk", false},
		{"package:/data/app/a/base.apk\npackage:/data/app/b/base.apk", false},
		{"package:/sdcard/hostile/base.apk", false},
	} {
		if got := installedAPKPath(test.input); (got != "") != test.want {
			t.Errorf("path accepted=%v, want %v", got != "", test.want)
		}
	}
}

func TestFindRejectsRelativeSDKAndCurrentDirectoryExecutable(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("ANDROID_HOME", "")
	t.Setenv("PATH", "")
	t.Setenv("GODEBUG", "execerrdot=1")
	if err := os.MkdirAll(filepath.Join("Android", "Sdk", "platform-tools"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"adb.exe", filepath.Join("Android", "Sdk", "platform-tools", "adb.exe")} {
		if err := os.WriteFile(path, []byte("synthetic executable discovery fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got := Find(); got != "" {
		t.Fatalf("relative executable discovery must fail closed, got %q", got)
	}
}

func TestPublicCompanionUsesSeparateAppIDAndExistingClassNamespace(t *testing.T) {
	client := &Client{AppPackage: PublicPackage}
	if got := client.provisionComponent(); got != "io.github.enesoglu.usbrelay/dev.usbreverse.lab.ProvisionActivity" {
		t.Fatalf("wrong public component: %s", got)
	}
	development := &Client{}
	if development.packageName() != Package {
		t.Fatal("development identity changed")
	}
}
