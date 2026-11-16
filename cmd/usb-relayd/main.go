// USB Relay desktop daemon uses private parent-owned stdio for control.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/adb"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/identity"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/manager"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/private"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"
)

type command struct {
	ID       int              `json:"id"`
	Method   string           `json:"method"`
	Serial   string           `json:"serial"`
	Remember bool             `json:"remember"`
	Settings manager.Settings `json:"settings"`
}

func main() {
	assets := flag.String("assets", "", "directory containing the verified companion APK")
	data := flag.String("data", filepath.Join(os.Getenv("LOCALAPPDATA"), "UsbReverseTether"), "per-user protected data directory")
	adbPath := flag.String("adb", "", "explicit adb.exe path")
	diagnosticPorts := flag.String("diagnostic-ports", "", "explicit isolated test-server ports; disabled in the desktop app")
	profilePath := flag.String("cpu-profile", "", "local CPU profile for isolated diagnostics only")
	companionPackage := flag.String("companion-package", adb.Package, "verified companion application ID")
	flag.Parse()
	var mutex sync.Mutex
	emit := func(value any) { mutex.Lock(); defer mutex.Unlock(); json.NewEncoder(os.Stdout).Encode(value) }
	fail := func(message string) { emit(map[string]any{"type": "fatal", "message": message}) }
	if *profilePath != "" {
		if *diagnosticPorts == "" {
			fail("CPU profiling requires isolated diagnostic mode")
			return
		}
		profile, e := os.OpenFile(*profilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if e != nil {
			fail("Could not create local CPU profile")
			return
		}
		defer profile.Close()
		if e = pprof.StartCPUProfile(profile); e != nil {
			fail("Could not start CPU profile")
			return
		}
		defer pprof.StopCPUProfile()
	}
	if *companionPackage != adb.Package && *companionPackage != adb.PublicPackage {
		fail("Unsupported companion package")
		return
	}
	host, err := identity.LoadOrCreate(*data)
	if err != nil {
		fail("Private identity could not be unlocked. No trust was reset.")
		return
	}
	config := manager.DefaultConfig()
	path := filepath.Join(*data, "settings.dpapi")
	if err := private.Load(path, &config); err != nil && !errors.Is(err, os.ErrNotExist) {
		fail("Protected settings could not be unlocked. No pairing was reset.")
		return
	}
	if *adbPath == "" {
		if config.Settings.ADBPath != "" {
			*adbPath = config.Settings.ADBPath
		} else {
			*adbPath = adb.Find()
		}
	}
	if *assets == "" {
		exe, _ := os.Executable()
		*assets = filepath.Dir(exe)
	}
	backend := &adb.Client{Path: *adbPath, AppPackage: *companionPackage, APK: filepath.Join(*assets, "usb-transfer-lab.apk")}
	controller := manager.New(backend, host, config, func(value manager.Config) error { return private.Save(path, value) })
	if *diagnosticPorts != "" {
		var ports []uint16
		values := strings.Split(*diagnosticPorts, ",")
		if len(values) > 3 {
			fail("Too many diagnostic ports")
			return
		}
		for _, value := range values {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1024 || n > 65535 {
				fail("Invalid diagnostic port")
				return
			}
			ports = append(ports, uint16(n))
		}
		controller.SetDiagnosticPorts(ports)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); controller.Run(ctx) }()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				emit(map[string]any{"type": "snapshot", "state": controller.Snapshot()})
			}
		}
	}()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 65536)
	for scanner.Scan() {
		var request command
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			emit(map[string]any{"type": "result", "id": 0, "ok": false, "error": "Invalid control request"})
			continue
		}
		var err error
		switch request.Method {
		case "connect":
			err = controller.Connect(request.Serial, request.Remember)
		case "disconnect":
			controller.Disconnect()
		case "forget":
			err = controller.Forget(request.Serial)
		case "settings":
			err = controller.SetSettings(request.Settings)
		case "snapshot":
		case "shutdown":
			cancel()
			<-done
			return
		default:
			err = errors.New("unsupported control request")
		}
		response := map[string]any{"type": "result", "id": request.ID, "ok": err == nil}
		if err != nil {
			response["error"] = err.Error()
		}
		emit(response)
		emit(map[string]any{"type": "snapshot", "state": controller.Snapshot()})
	}
	cancel()
	<-done
}
