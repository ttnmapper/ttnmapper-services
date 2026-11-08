package aprsbridge

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/ebarkie/aprs"
)

// Own the transport so the APRS library never logs the login command.
func sendAPRSFrames(ctx context.Context, address string, user aprs.Addr, password int, frames <-chan aprs.Frame) {
	for ctx.Err() == nil {
		conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
		if err == nil {
			log.Print("APRS connected")
			err = aprsSession(ctx, conn, user, password, frames)
			if err == nil {
				return // Input channel closed.
			}
		}
		if ctx.Err() != nil {
			return
		}
		log.Printf("APRS connection failed: %v; retrying in one minute", err)
		timer := time.NewTimer(time.Minute)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// aprsSession owns conn. Protocol traffic, including authentication, is never
// written to logs. A single buffered reader retains data received with the banner.
func aprsSession(ctx context.Context, conn net.Conn, user aprs.Addr, password int, frames <-chan aprs.Frame) error {
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	reader := bufio.NewReader(conn)
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if _, err := reader.ReadString('\n'); err != nil {
		return fmt.Errorf("read APRS banner: %w", err)
	}
	if _, err := fmt.Fprintf(conn, "user %s pass %d vers %s %s\r\n", user, password, aprs.SwName, aprs.SwVers); err != nil {
		return fmt.Errorf("write APRS login: %w", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		return fmt.Errorf("read APRS login response: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return err
	}
	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, reader)
		if err == nil {
			err = io.EOF
		}
		readDone <- err
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readDone:
			return fmt.Errorf("read APRS connection: %w", err)
		case frame, ok := <-frames:
			if !ok {
				return nil
			}
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(conn, "%s\r\n", frame); err != nil {
				return fmt.Errorf("write APRS frame: %w", err)
			}
		}
	}
}
