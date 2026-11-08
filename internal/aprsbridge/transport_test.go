package aprsbridge

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ebarkie/aprs"
)

func TestAPRSSessionAuthenticatesAndSendsWithoutLoggingCredentials(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	client, server := net.Pipe()
	t.Cleanup(func() { server.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	frames := make(chan aprs.Frame, 1)
	frame := aprs.Frame{Src: aprs.Addr{Call: "TEST"}, Dst: aprs.Addr{Call: "APRS"}, Text: "test-position"}
	frames <- frame
	done := make(chan error, 1)
	go func() { done <- aprsSession(ctx, client, aprs.Addr{Call: "RELAY"}, 12345, frames) }()
	if err := server.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(server)
	fmt.Fprint(server, "# APRS server\r\n")
	login, err := reader.ReadString('\n')
	wantLogin := fmt.Sprintf("user RELAY pass 12345 vers %s %s\r\n", aprs.SwName, aprs.SwVers)
	if err != nil || login != wantLogin {
		t.Fatalf("login mismatch or read failed: %v", err)
	}
	// Send several lines at once to exercise buffered protocol reads.
	fmt.Fprint(server, "# logresp RELAY verified, server TEST\r\n# keepalive\r\n")
	wireFrame, err := reader.ReadString('\n')
	if err != nil || wireFrame != fmt.Sprintf("%s\r\n", frame) {
		t.Fatalf("frame=%q error=%v", wireFrame, err)
	}
	close(frames)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("session did not stop when input closed")
	}
	if strings.Contains(logs.String(), "12345") || strings.Contains(logs.String(), "user RELAY pass") {
		t.Fatal("APRS credentials appeared in logs")
	}
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		t.Fatalf("session did not close its connection: %v", err)
	}
}

func TestAPRSSessionStopsOnDisconnectOrCancellation(t *testing.T) {
	for _, stage := range []string{"banner", "login", "connected", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			client, server := net.Pipe()
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- aprsSession(ctx, client, aprs.Addr{Call: "TEST"}, 12345, make(chan aprs.Frame)) }()
			server.SetDeadline(time.Now().Add(5 * time.Second))
			if stage != "banner" {
				fmt.Fprint(server, "# server\r\n")
				if _, err := bufio.NewReader(server).ReadString('\n'); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "connected" || stage == "cancel" {
				fmt.Fprint(server, "# logresp TEST verified, server TEST\r\n")
			}
			if stage == "cancel" {
				cancel()
			} else {
				server.Close()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("disconnected session returned success")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("session did not stop")
			}
		})
	}
}
