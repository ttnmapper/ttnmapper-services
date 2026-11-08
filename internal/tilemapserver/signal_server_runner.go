package tms

import (
	"context"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// Configuration is loaded before the first invocation. All Signal-Server calls,
// including elevation queries and offline optimization, share this budget.
var signalServerRunnerOnce sync.Once
var sharedSignalServerRunner *signalServerRunner

type signalServerRunner struct {
	bin     string
	threads int
	slots   chan struct{}
	// Bound the complete simulation pipeline too: decoding a large raster must
	// not accumulate behind fast subprocesses and exhaust memory.
	simulations chan struct{}
	timeout     time.Duration
}

func signalServerBudget(cpus, processes, threads int) (int, int) {
	cpus = max(1, cpus)
	processes = min(cpus, max(1, processes))
	if threads <= 0 {
		threads = max(1, cpus/processes)
	} else {
		threads = min(cpus, threads)
		processes = min(processes, cpus/threads)
	}
	return processes, threads
}

func newSignalServerRunner(config Configuration, cpus int) *signalServerRunner {
	processes, threads := signalServerBudget(cpus, config.SignalServerMaxProcesses, config.SignalServerThreads)
	timeout := config.SignalServerTimeoutSeconds
	if timeout <= 0 {
		timeout = 300
	}
	return &signalServerRunner{
		bin:         config.SignalServerBin,
		threads:     threads,
		slots:       make(chan struct{}, processes),
		simulations: make(chan struct{}, processes),
		timeout:     time.Duration(timeout) * time.Second,
	}
}

func getSignalServerRunner() *signalServerRunner {
	signalServerRunnerOnce.Do(func() {
		sharedSignalServerRunner = newSignalServerRunner(MyConfiguration, min(runtime.NumCPU(), runtime.GOMAXPROCS(0)))
		log.Printf("Signal-Server budget: %d processes, %d threads each", cap(sharedSignalServerRunner.slots), sharedSignalServerRunner.threads)
	})
	return sharedSignalServerRunner
}

func runSignalServer(args []string) ([]byte, error) {
	return getSignalServerRunner().run(args)
}

func (runner *signalServerRunner) run(args []string) ([]byte, error) {
	runner.slots <- struct{}{}
	defer func() { <-runner.slots }()
	ctx, cancel := context.WithTimeout(context.Background(), runner.timeout)
	defer cancel()
	// Copy before appending: callers may reuse their arguments.
	commandArgs := append(append([]string(nil), args...), "-number_threads", strconv.Itoa(runner.threads))
	out, err := exec.CommandContext(ctx, runner.bin, commandArgs...).CombinedOutput()
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	return out, err
}
