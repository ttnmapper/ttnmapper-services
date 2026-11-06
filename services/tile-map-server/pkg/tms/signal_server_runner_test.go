package tms

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSignalServerBudget(t *testing.T) {
	for _, tt := range []struct{ cpus, processes, threads, wantProcesses, wantThreads int }{
		{8, 1, 0, 1, 8}, {8, 2, 0, 2, 4}, {8, 3, 0, 3, 2},
		{8, 8, 4, 2, 4}, {8, 2, 20, 1, 8}, {1, 20, 0, 1, 1}, {0, 0, 0, 1, 1},
	} {
		p, threads := signalServerBudget(tt.cpus, tt.processes, tt.threads)
		if p != tt.wantProcesses || threads != tt.wantThreads {
			t.Errorf("budget(%d,%d,%d) = (%d,%d), want (%d,%d)", tt.cpus, tt.processes, tt.threads, p, threads, tt.wantProcesses, tt.wantThreads)
		}
	}
}

func TestSignalServerRunnerLimitsProcessesAndPassesThreads(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "signalserver")
	// Every subprocess records its lifetime and arguments in a shared log.
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'start %s\\n' \"$*\" >> \"$1\"\nsleep 0.02\nprintf 'end\\n' >> \"$1\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "calls")
	runner := newSignalServerRunner(Configuration{SignalServerBin: bin, SignalServerMaxProcesses: 2}, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := runner.run([]string{logPath}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	active, peak, starts := 0, 0, 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasPrefix(line, "start ") {
			if !strings.HasSuffix(line, "-number_threads 4") {
				t.Errorf("missing thread budget: %s", line)
			}
			starts++
			active++
			peak = max(peak, active)
		} else if line == "end" {
			active--
		} else {
			t.Fatalf("unexpected subprocess output: %s", line)
		}
		if active < 0 || active > 2 {
			t.Fatalf("active subprocesses = %d", active)
		}
	}
	if starts != 8 || active != 0 || peak == 0 {
		t.Fatalf("starts=%d, active=%d, peak=%d", starts, active, peak)
	}
}

func TestSignalServerRunnerTimeoutReleasesSlot(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "signalserver")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nif [ \"$1\" = hang ]; then exec sleep 30; fi\nprintf ok\n"), 0755); err != nil {
		t.Fatal(err)
	}
	runner := newSignalServerRunner(Configuration{SignalServerBin: bin}, 1)
	runner.timeout = 50 * time.Millisecond
	if _, err := runner.run([]string{"hang"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}
	if out, err := runner.run(nil); err != nil || string(out) != "ok" {
		t.Fatalf("subsequent invocation: %q, %v", out, err)
	}
}
