// Package adb owns hidden ADB processes and mappings created by this application.
package adb

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const Package = "dev.usbreverse.lab"

// PublicPackage is the independently signed public companion application.
const PublicPackage = "io.github.enesoglu.usbrelay"

// Device contains only the ADB fields required for device selection.
type Device struct {
	Transport string `json:"transport"`
	Serial    string `json:"serial"`
	Model     string `json:"model"`
	State     string `json:"state"`
}

// Provision contains the pinned transport configuration; its nonce is never put in a process argument.
type Provision struct {
	AttachID   string `json:"attachId"`
	Port       int    `json:"port"`
	Pin        string `json:"pin"`
	Nonce      string `json:"nonce"`
	DNS        string `json:"dns"`
	Background bool   `json:"background"`
}

// Client discovers ADB and never invokes a Windows shell.
type Client struct {
	AppPackage string
	Path       string
	APK        string
}

func Find() string {
	candidates := []string{filepath.Join(os.Getenv("LOCALAPPDATA"), "UsbRelay", "tools", "platform-tools-"+managedToolsVersion, "platform-tools", "adb.exe"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Android", "Sdk", "platform-tools", "adb.exe"), filepath.Join(os.Getenv("ANDROID_HOME"), "platform-tools", "adb.exe")}
	for _, p := range candidates {
		if !filepath.IsAbs(p) {
			continue
		}
		if info, e := os.Stat(p); e == nil && !info.IsDir() {
			return p
		}
	}
	p, err := exec.LookPath("adb.exe")
	if err != nil || !filepath.IsAbs(p) {
		return ""
	}
	return p
}
func (c *Client) run(ctx context.Context, input string, args ...string) (string, error) {
	path := c.Path
	if path == "" {
		path = Find()
	}
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Base(path), "adb.exe") {
		return "", errors.New("ADB not found; open Setup to download Google's platform tools")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	output, err := cmd.Output()
	if err != nil {
		return "", errors.New("ADB operation failed; check USB debugging and the device")
	}
	if len(output) > 2*1024*1024 {
		return "", errors.New("ADB response exceeded the size limit")
	}
	return string(output), nil
}

// ParseDevices is shared by discovery and deterministic selection tests.
func ParseDevices(output string) []Device {
	var result []Device
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] == "List" {
			continue
		}
		if f[1] != "device" && f[1] != "unauthorized" && f[1] != "offline" {
			continue
		}
		d := Device{Serial: f[0], Model: "Android phone", State: f[1]}
		for _, field := range f[2:] {
			if strings.HasPrefix(field, "transport_id:") {
				d.Transport = strings.TrimPrefix(field, "transport_id:")
			}
			if strings.HasPrefix(field, "model:") {
				d.Model = strings.ReplaceAll(strings.TrimPrefix(field, "model:"), "_", " ")
			}
		}
		result = append(result, d)
	}
	return result
}
func (c *Client) List(ctx context.Context) ([]Device, error) {
	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := c.run(timeout, "", "devices", "-l")
	if err != nil {
		return nil, err
	}
	return ParseDevices(output), nil
}

var validSerial = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,160}$`)

// Prepare returns a lease that removes only its own reverse mapping.
func (c *Client) Prepare(ctx context.Context, device Device, config Provision, automatic bool) (func(), error) {
	if !validSerial.MatchString(device.Serial) {
		return nil, errors.New("invalid device identifier")
	}
	timeout, cancel := context.WithTimeout(ctx, 70*time.Second)
	defer cancel()
	base := []string{"-s", device.Serial}
	run := func(input string, args ...string) (string, error) {
		return c.run(timeout, input, append(append([]string{}, base...), args...)...)
	}
	path, err := run("", "get-devpath")
	if err != nil || !usbSelection(path, device.Serial, func() (string, error) { return c.run(timeout, "", "-d", "get-serialno") }) {
		return nil, errors.New("only a directly attached USB device can be paired")
	}
	// An exact installed-file hash avoids unnecessary package replacement. A
	// version string is never signature proof; changed APKs go through Android's
	// installer, which enforces signing-key compatibility.
	if automatic {
		state, err := run("", "shell", "dumpsys", "package", c.packageName())
		if err != nil {
			return nil, err
		}
		if regexp.MustCompile(`User 0:[^\n]*stopped=true`).MatchString(state) {
			return nil, errors.New("phone app was force-stopped; open it or connect manually")
		}
	}
	verified := false
	if location, e := run("", "shell", "pm", "path", c.packageName()); e == nil {
		if path := installedAPKPath(location); path != "" {
			if remote, e := run("", "shell", "sha256sum", path); e == nil {
				if file, e := os.Open(c.APK); e == nil {
					hash := sha256.New()
					_, copyErr := io.Copy(hash, file)
					file.Close()
					fields := strings.Fields(remote)
					verified = copyErr == nil && len(fields) == 2 && fields[1] == path && fields[0] == hex.EncodeToString(hash.Sum(nil))
				}
			}
		}
	}
	if !verified {
		output, err := run("", "install", "-r", c.APK)
		if err != nil || !strings.Contains(output, "Success") {
			return nil, errors.New("could not verify/install the signed companion app; a different signing key or newer app may be installed")
		}
	}
	mapping := "tcp:" + strconv.Itoa(config.Port)
	existing, err := run("", "reverse", "--list")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(existing, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[len(f)-2] == mapping {
			return nil, errors.New("the requested ADB mapping is already owned by another session")
		}
	}
	if _, err = run("", "reverse", mapping, mapping); err != nil {
		return nil, err
	}
	cleanup := func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		c.run(cleanupCtx, "", append(base, "reverse", "--remove", mapping)...)
	}
	raw, _ := json.Marshal(config)
	encoded := base64.StdEncoding.EncodeToString(raw)
	// Only base64 enters the device shell. No session secret appears in host argv.
	script := fmt.Sprintf("am start -W -f 0x18000000 -n %s --es config '%s'\n", c.provisionComponent(), encoded)
	output, err := run(script, "shell", "sh")
	if err != nil || strings.Contains(output, "Exception") || strings.Contains(output, "Error:") || strings.Contains(output, "Warning: Activity not started") {
		cleanup()
		return nil, errors.New("companion provisioning failed; update the Android app")
	}
	return cleanup, nil
}

// Stop requests a graceful service disconnect without force-stopping the package.
func (c *Client) Stop(serial string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.run(ctx, "", "-s", serial, "shell", "am", "start", "-W", "-f", "0x18000000", "-n", c.provisionComponent(), "--ez", "stop", "true")
}

// usbSelection authenticates a selected serial against ADB's USB transport.
func usbSelection(path, selected string, usbSerial func() (string, error)) bool {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "usb:") {
		return true
	}
	if path != "unknown" || selected == "" {
		return false
	}
	// Windows' native USB backend may not expose get-devpath. The -d selector
	// is enforced by ADB itself and fails closed on multiple attached USB devices.
	actual, err := usbSerial()
	return err == nil && strings.TrimSpace(actual) == selected
}

// installedAPKPath accepts a single base APK path with no device-shell metacharacters.
func installedAPKPath(output string) string {
	output = strings.TrimSpace(output)
	if strings.ContainsAny(output, "\r\n") || !strings.HasPrefix(output, "package:") {
		return ""
	}
	path := strings.TrimPrefix(output, "package:")
	if !regexp.MustCompile(`^/data/app/[A-Za-z0-9_./=+~\-]+/base\.apk$`).MatchString(path) || strings.Contains(path, "/../") {
		return ""
	}
	return path
}

func (c *Client) packageName() string {
	if c.AppPackage == PublicPackage {
		return PublicPackage
	}
	return Package
}
func (c *Client) provisionComponent() string {
	return c.packageName() + "/" + Package + ".ProvisionActivity"
}
