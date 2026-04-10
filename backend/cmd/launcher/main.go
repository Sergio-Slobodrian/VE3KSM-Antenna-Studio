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

type Config struct {
	BackendPort  int
	FrontendPort int
	CORSOrigins  string
	BackendDir   string
	FrontendDir  string
}

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

	// Auto-detect directories relative to this binary or cwd
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

	// Validate directories exist
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

	lastInterrupt := time.Time{}

	for sig := range sigCh {
		switch sig {
		case syscall.SIGTERM:
			log.Println("Received SIGTERM, shutting down...")
			stopAll()
			os.Exit(0)

		case syscall.SIGHUP:
			log.Println("Received SIGHUP, restarting...")
			stopAll()
			startAll()

		case syscall.SIGINT:
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

func startProcess(name, dir string, args []string, extraEnv []string) *process {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdout = &prefixWriter{prefix: fmt.Sprintf("[%s] ", name), w: os.Stdout}
	cmd.Stderr = &prefixWriter{prefix: fmt.Sprintf("[%s] ", name), w: os.Stderr}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Inherit environment, add extras
	cmd.Env = append(os.Environ(), extraEnv...)

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start %s: %v", name, err)
		return &process{name: name, cmd: nil}
	}

	log.Printf("Started %s (PID %d)", name, cmd.Process.Pid)

	// Reap the process in the background so it doesn't become a zombie
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("%s exited: %v", name, err)
		} else {
			log.Printf("%s exited cleanly", name)
		}
	}()

	return &process{name: name, cmd: cmd}
}

func killAll(procs []*process) {
	for _, p := range procs {
		if p.cmd == nil || p.cmd.Process == nil {
			continue
		}
		// Kill the entire process group so child processes (node, go) also die
		pgid, err := syscall.Getpgid(p.cmd.Process.Pid)
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
		} else {
			_ = p.cmd.Process.Kill()
		}
		log.Printf("Stopped %s", p.name)
	}
	// Brief pause for processes to clean up
	time.Sleep(500 * time.Millisecond)
}

// detectRoot finds the project root by looking for backend/ and frontend/ dirs
func detectRoot() string {
	// Try cwd first
	cwd, _ := os.Getwd()
	if hasSubdirs(cwd) {
		return cwd
	}
	// Try the directory containing the executable
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	if hasSubdirs(exeDir) {
		return exeDir
	}
	// Try parent of exe dir (in case binary is in cmd/launcher or bin/)
	parent := filepath.Dir(exeDir)
	if hasSubdirs(parent) {
		return parent
	}
	// Try grandparent (binary in backend/cmd/launcher)
	grandparent := filepath.Dir(parent)
	if hasSubdirs(grandparent) {
		return grandparent
	}
	return cwd
}

func hasSubdirs(dir string) bool {
	_, err1 := os.Stat(filepath.Join(dir, "backend"))
	_, err2 := os.Stat(filepath.Join(dir, "frontend"))
	return err1 == nil && err2 == nil
}

// prefixWriter prepends a tag to each line of output
type prefixWriter struct {
	prefix string
	w      *os.File
	buf    []byte
}

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
