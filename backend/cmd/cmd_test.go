package cmd_test

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// Each command starts, reports it is running, and exits 0 on SIGTERM. The
// worker runs River, so it gets a migrated database.
func TestCommandsStartAndExitCleanlyOnSignal(t *testing.T) {
	for _, name := range []string{"api", "worker"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bin := filepath.Join(t.TempDir(), name)
			if out, err := exec.Command("go", "build", "-o", bin, "./"+name).CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}

			cmd := exec.Command(bin)
			if name == "worker" {
				pool := testdb.NewDatabase(t)
				if err := db.Up(context.Background(), pool); err != nil {
					t.Fatal(err)
				}
				cmd.Env = append(os.Environ(), "WORKER_DATABASE_URL="+testdb.URL(pool))
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })

			started := make(chan struct{})
			go func() {
				sc := bufio.NewScanner(stdout)
				for sc.Scan() {
					t.Log(sc.Text())
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

// The worker needs its database; without one it says so and exits 1 instead of
// looking healthy and working nothing.
func TestWorkerWithoutADatabaseExitsWithAnError(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "worker")
	if out, err := exec.Command("go", "build", "-o", bin, "./worker").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "WORKER_DATABASE_URL=")
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit status 1; output:\n%s", err, out)
	}
	if !strings.Contains(string(out), "WORKER_DATABASE_URL") {
		t.Fatalf("output does not name WORKER_DATABASE_URL:\n%s", out)
	}
}

// `migrate up` and `migrate down` are what make migrate-up and migrate-down run.
func TestMigrateCommandAppliesAndRollsBack(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "migrate")
	if out, err := exec.Command("go", "build", "-o", bin, "./migrate").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	pool := testdb.NewDatabase(t)
	run := func(arg string) {
		t.Helper()
		cmd := exec.Command(bin, arg)
		cmd.Env = append(os.Environ(), "DATABASE_URL="+testdb.URL(pool))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("migrate %s: %v\n%s", arg, err, out)
		}
	}
	appExists := func() bool {
		var ok bool
		if err := pool.QueryRow(context.Background(),
			`select exists (select 1 from pg_namespace where nspname = 'app')`).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}

	run("up")
	if !appExists() {
		t.Fatal("schema app missing after migrate up")
	}
	run("down")
	if appExists() {
		t.Fatal("schema app still there after migrate down")
	}
}

func TestMigrateCommandRejectsAnUnknownDirection(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "migrate")
	if out, err := exec.Command("go", "build", "-o", bin, "./migrate").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "sideways").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "usage") {
		t.Fatalf("err = %v, output %q; want a usage error", err, out)
	}
}
