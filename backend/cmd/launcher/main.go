// Package main implements the Antenna Studio process launcher.
// It starts both the Go backend server and the Vite frontend dev server
// as child processes, prefixes their output for easy identification, and
// manages their lifecycle via Unix signals.
//
// Signal handling behavior:
//   - SIGINT (Ctrl+C once):  gracefully restart both processes
//   - SIGINT twice within 2s: shut down completely
//   - SIGTERM:               shut down completely (used by systemd, Docker, etc.)
//   - SIGHUP:                restart both processes (conventional reload signal)
//
// Each child process runs in its own process group (Setpgid: true) so that
// killAll can terminate the entire tree (e.g. node + its children) with a
// single signal to the group leader.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Config holds the launcher's command-line configuration.
type Config struct {
	BackendPort  int    // TCP port for the Go backend server
	FrontendPort int    // TCP port for the Vite dev server
	CORSOrigins  string // Comma-separated CORS origins passed to the backend
	BackendDir   string // Filesystem path to the backend/ directory
	FrontendDir  string // Filesystem path to the frontend/ directory
}

// process pairs a human-readable name with a running exec.Cmd for lifecycle management.
type process struct {
	name string
	cmd  *exec.Cmd
}

func main() {
	cfg := Config{}
	flag.IntVar(&cfg.BackendPort, "port", 8080, "Backend server port")
	flag.IntVar(&cfg.FrontendPort, "frontend-port", 5173, "Frontend dev server port")
	flag.StringVar(&cfg.CORSOrigins, "cors", "", "CORS origins (default: http://localhost:<frontend-port>)")
	flag.StringVar(&cfg.BackendDir, "backend-dir", "", "Path to backend directory (auto-detected if empty)")
	flag.StringVar(&cfg.FrontendDir, "frontend-dir", "", "Path to frontend directory (auto-detected if empty)")
	flag.Parse()

	// Auto-detect project root by searching for sibling backend/ and frontend/ dirs
	root := detectRoot()
	if cfg.BackendDir == "" {
		cfg.BackendDir = filepath.Join(root, "backend")
	}
	if cfg.FrontendDir == "" {
		cfg.FrontendDir = filepath.Join(root, "frontend")
	}
	if cfg.CORSOrigins == "" {
		cfg.CORSOrigins = fmt.Sprintf("http://localhost:%d", cfg.FrontendPort)
	}

	// Fail fast if the required directories are missing
	for _, d := range []struct{ name, path string }{
		{"backend", cfg.BackendDir},
		{"frontend", cfg.FrontendDir},
	} {
		if _, err := os.Stat(d.path); os.IsNotExist(err) {
			log.Fatalf("%s directory not found: %s", d.name, d.path)
		}
	}

	log.Printf("Antenna Studio Launcher")
	log.Printf("  Backend:  http://localhost:%d  (dir: %s)", cfg.BackendPort, cfg.BackendDir)
	log.Printf("  Frontend: http://localhost:%d  (dir: %s)", cfg.FrontendPort, cfg.FrontendDir)
	log.Printf("  CORS:     %s", cfg.CORSOrigins)
	log.Println()
	log.Println("Ctrl+C      → restart both processes")
	log.Println("Ctrl+C ×2   → shutdown (within 2s)")
	log.Println()

	// Listen for signals on a buffered channel (buffer=1 so we don't miss signals
	// that arrive while we're processing the previous one)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	var procs []*process
	startAll := func() {
		procs = launch(cfg)
		log.Println("All processes started.")
	}
	stopAll := func() {
		killAll(procs)
		procs = nil
	}

	startAll()

	// Track when the last SIGINT arrived for double-Ctrl+C detection
	lastInterrupt := time.Time{}

	for sig := range sigCh {
		switch sig {
		case syscall.SIGTERM:
			// SIGTERM = clean shutdown (no restart)
			log.Println("Received SIGTERM, shutting down...")
			stopAll()
			os.Exit(0)

		case syscall.SIGHUP:
			// SIGHUP = conventional "reload config" signal; we restart both processes
			log.Println("Received SIGHUP, restarting...")
			stopAll()
			startAll()

		case syscall.SIGINT:
			// Single Ctrl+C = restart; double Ctrl+C within 2 seconds = quit
			now := time.Now()
			if now.Sub(lastInterrupt) < 2*time.Second {
				log.Println("Double Ctrl+C, shutting down...")
				stopAll()
				os.Exit(0)
			}
			lastInterrupt = now
			log.Println("Received Ctrl+C, restarting... (press again within 2s to quit)")
			stopAll()
			startAll()
		}
	}
}

// launch starts both the backend and frontend child processes and returns
// them as a slice for lifecycle management. The backend is started with
// "go run ./cmd/server" and receives PORT and CORS_ORIGINS via environment
// variables. The frontend runs "npx vite" on the configured port.
func launch(cfg Config) []*process {
	backend := startProcess("backend", cfg.BackendDir,
		[]string{"go", "run", "./cmd/server"},
		[]string{
			fmt.Sprintf("PORT=%d", cfg.BackendPort),
			fmt.Sprintf("CORS_ORIGINS=%s", cfg.CORSOrigins),
		},
	)

	frontend := startProcess("frontend", cfg.FrontendDir,
		[]string{"npx", "vite", "--port", strconv.Itoa(cfg.FrontendPort)},
		nil,
	)

	return []*process{backend, frontend}
}

// startProcess spawns a child process with the given arguments, working
// directory, and extra environment variables. It sets Setpgid=true so the
// child gets its own process group, enabling killAll to terminate the entire
// subtree. Stdout and stderr are piped through prefixWriter to tag each
// line with "[name] " for easy identification in the launcher's output.
// A background goroutine calls cmd.Wait() to reap the child and prevent zombies.
func startProcess(name, dir string, args []string, extraEnv []string) *process {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdout = &prefixWriter{prefix: fmt.Sprintf("[%s] ", name), w: os.Stdout}
	cmd.Stderr = &prefixWriter{prefix: fmt.Sprintf("[%s] ", name), w: os.Stderr}
	// Setpgid puts the child in its own process group so we can kill the
	// entire group (including grandchildren like node workers) at once
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Inherit the launcher's environment and overlay any extra vars
	cmd.Env = append(os.Environ(), extraEnv...)

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start %s: %v", name, err)
		return &process{name: name, cmd: nil}
	}

	log.Printf("Started %s (PID %d)", name, cmd.Process.Pid)

	// Reap the child in the background to prevent zombie processes.
	// This goroutine runs until the child exits.
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("%s exited: %v", name, err)
		} else {
			log.Printf("%s exited cleanly", name)
		}
	}()

	return &process{name: name, cmd: cmd}
}

// killAll sends SIGTERM to the process group of each managed process.
// Using the negative PGID (-pgid) sends the signal to every process in
// the group, ensuring that child processes spawned by go or node are also
// terminated. Falls back to Process.Kill() if the PGID lookup fails.
// A 500ms pause gives processes time to flush output and clean up.
func killAll(procs []*process) {
	for _, p := range procs {
		if p.cmd == nil || p.cmd.Process == nil {
			continue
		}
		pgid, err := syscall.Getpgid(p.cmd.Process.Pid)
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
		} else {
			_ = p.cmd.Process.Kill()
		}
		log.Printf("Stopped %s", p.name)
	}
	// Brief pause for processes to flush and clean up
	time.Sleep(500 * time.Millisecond)
}

// detectRoot finds the project root directory by searching for a directory
// that contains both backend/ and frontend/ subdirectories. It checks:
//  1. The current working directory
//  2. The directory containing the launcher executable
//  3. Up to 4 parent directories from each of those starting points
//
// This lets the launcher work regardless of where it's invoked from:
// from the project root, from backend/, or from a build output directory.
func detectRoot() string {
	candidates := []string{}

	cwd, _ := os.Getwd()
	candidates = append(candidates, cwd)

	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	candidates = append(candidates, exeDir)

	// Walk up to 4 levels from each starting point
	for _, start := range []string{cwd, exeDir} {
		dir := start
		for range 4 {
			dir = filepath.Dir(dir)
			candidates = append(candidates, dir)
		}
	}

	for _, dir := range candidates {
		if hasSubdirs(dir) {
			return dir
		}
	}
	return cwd
}

// hasSubdirs checks whether a directory contains both backend/ and frontend/
// subdirectories, which is the signature of the Antenna Studio project root.
func hasSubdirs(dir string) bool {
	_, err1 := os.Stat(filepath.Join(dir, "backend"))
	_, err2 := os.Stat(filepath.Join(dir, "frontend"))
	return err1 == nil && err2 == nil
}

// prefixWriter is an io.Writer that prepends a tag (e.g. "[backend] ") to
// each line of output. It buffers partial lines until a newline is received,
// ensuring the prefix is only inserted at line boundaries and not mid-line.
type prefixWriter struct {
	prefix string
	w      *os.File
	buf    []byte
}

// Write implements io.Writer. It appends data to an internal buffer and
// flushes complete lines (terminated by '\n') with the prefix prepended.
// Partial lines remain buffered until the next Write delivers a newline.
func (pw *prefixWriter) Write(p []byte) (int, error) {
	pw.buf = append(pw.buf, p...)
	for {
		idx := -1
		for i, b := range pw.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		line := pw.buf[:idx+1]
		pw.buf = pw.buf[idx+1:]
		fmt.Fprintf(pw.w, "%s%s", pw.prefix, line)
	}
	return len(p), nil
}
