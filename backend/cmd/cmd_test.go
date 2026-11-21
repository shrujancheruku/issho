package cmd_test

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Each command starts, reports it is running, and exits 0 on SIGTERM.
func TestCommandsStartAndExitCleanlyOnSignal(t *testing.T) {
	for _, name := range []string{"api", "worker"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bin := filepath.Join(t.TempDir(), name)
			if out, err := exec.Command("go", "build", "-o", bin, "./"+name).CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}

			cmd := exec.Command(bin)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })

			started := make(chan struct{})
			go func() {
				sc := bufio.NewScanner(stdout)
				for sc.Scan() {
					if strings.Contains(sc.Text(), "started") {
						close(started)
						break
					}
				}
				for sc.Scan() { // drain so the child never blocks on a full pipe
				}
			}()
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("did not report started")
			}

			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("exit after SIGTERM: %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("did not exit after SIGTERM")
			}
		})
	}
}
