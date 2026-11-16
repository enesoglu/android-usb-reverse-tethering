// USB Relay is a loopback-only userspace reverse-tethering server.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/relay"
	"io"
	"net"
	"os"
	"os/signal"
	"time"
)

func main() {
	port := flag.Int("port", 31417, "loopback TCP port")
	insecureLab := flag.Bool("insecure-lab", false, "explicitly enable the deprecated plaintext lab server")
	flag.Parse()
	if !*insecureLab {
		fmt.Fprintln(os.Stderr, "Use the secure desktop app; plaintext is available only with --insecure-lab for isolated tests")
		os.Exit(1)
	}
	token, err := hex.DecodeString(os.Getenv("URT_TOKEN"))
	if err != nil || len(token) != 32 {
		fmt.Fprintln(os.Stderr, "URT_TOKEN must be a 64-character hex session token")
		os.Exit(1)
	}
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot listen on loopback")
		os.Exit(1)
	}
	defer listener.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() { <-ctx.Done(); listener.Close() }()
	fmt.Println("USB relay listening on loopback")
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// One session at a time bounds stack and flow resources.
		metrics := &relay.Metrics{}
		sessionCtx, cancel := context.WithCancel(ctx)
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-sessionCtx.Done():
					return
				case <-ticker.C:
					json.NewEncoder(os.Stdout).Encode(map[string]any{"in_packets": metrics.InPackets.Load(), "out_packets": metrics.OutPackets.Load(), "in_bytes": metrics.InBytes.Load(), "out_bytes": metrics.OutBytes.Load(), "active_flows": metrics.ActiveFlows.Load(), "link_queue_rejections": metrics.QueueRejections.Load(), "flow_rejections": metrics.FlowRejections.Load(), "dial_failures": metrics.DialFailures.Load()})
				}
			}
		}()
		sessionErr := relay.Serve(sessionCtx, conn, token, metrics)
		reason := "io_error"
		switch {
		case sessionErr == nil:
			reason = "completed"
		case errors.Is(sessionErr, io.EOF):
			reason = "peer_eof"
		case errors.Is(sessionErr, context.Canceled):
			reason = "canceled"
		case sessionErr.Error() == "invalid ipv4 packet":
			reason = "invalid_ipv4"
		case sessionErr.Error() == "invalid packet size":
			reason = "invalid_frame_size"
		case sessionErr.Error() == "unauthorized tunnel":
			reason = "unauthorized"
		}

		json.NewEncoder(os.Stdout).Encode(map[string]any{"event": "session_ended", "reason": reason, "in_packets": metrics.InPackets.Load(), "out_packets": metrics.OutPackets.Load(), "unsupported_ipv6": metrics.UnsupportedIPv6.Load(), "queue_rejections": metrics.QueueRejections.Load()})
		cancel()
		fmt.Println("USB tunnel disconnected")
	}
}
