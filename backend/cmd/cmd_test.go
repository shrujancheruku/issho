package cmd_test

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/db"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
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
			if name == "api" {
				// Port 0: the OS picks a free one, so parallel tests and a
				// running dev server never clash.
				cmd.Env = append(os.Environ(), "API_ADDR=127.0.0.1:0")
			}
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

// The api serves ConnectRPC, and its health check reports the version the binary
// was built with (`-ldflags "-X main.version=..."`, which a release build sets).
func TestAPIServesTheHealthCheckWithItsBuildVersion(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "api")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=9.9.9-test", "-o", bin, "./api")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "API_ADDR=127.0.0.1:0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// The started line says which port the OS gave it.
	addrs := make(chan string, 1)
	go func() {
		addr := regexp.MustCompile(`addr=(\S+)`)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if m := addr.FindStringSubmatch(sc.Text()); m != nil && strings.Contains(sc.Text(), "started") {
				addrs <- m[1]
				break
			}
		}
		for sc.Scan() {
		}
	}()
	var addr string
	select {
	case addr = <-addrs:
	case <-time.After(10 * time.Second):
		t.Fatal("did not report its address")
	}

	client := isshov1connect.NewHealthServiceClient(http.DefaultClient, "http://"+addr)
	res, err := client.Check(context.Background(), connect.NewRequest(&isshov1.CheckRequest{}))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got := res.Msg.GetVersion(); got != "9.9.9-test" {
		t.Fatalf("version = %q, want 9.9.9-test", got)
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
	// goose records each applied migration as a row; `down` rolls back one, so
	// this holds however many migrations there are.
	applied := func() int {
		var n int
		if err := pool.QueryRow(context.Background(),
			`select count(*) from goose.goose_db_version where version_id > 0 and is_applied`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	run("up")
	before := applied()
	if before == 0 {
		t.Fatal("no migration applied after migrate up")
	}
	run("down")
	if after := applied(); after != before-1 {
		t.Fatalf("migrations applied after migrate down = %d, want %d", after, before-1)
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

// `make rpc-auth` runs this command: it exits 1 and names the RPC that has no
// access rule, and exits 0 when every RPC has one.
func TestRPCAuthFailsOnAnRPCWithNoRuleAndPassesOtherwise(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "rpcauth")
	if out, err := exec.Command("go", "build", "-o", bin, "./rpcauth").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	fixtures := filepath.Join("..", "internal", "rpc", "rpcauth", "testdata")
	protoRoot := filepath.Join("..", "..", "proto")

	out, err := exec.Command(bin, "-I", protoRoot, filepath.Join(fixtures, "undeclared")).CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("undeclared fixture: err = %v, want exit status 1; output:\n%s", err, out)
	}
	if !strings.Contains(string(out), "fixture.v1.UnruledService.Forgotten") {
		t.Fatalf("output does not name the RPC:\n%s", out)
	}

	if out, err := exec.Command(bin, "-I", protoRoot, filepath.Join(fixtures, "declared")).CombinedOutput(); err != nil {
		t.Fatalf("declared fixture: %v\n%s", err, out)
	}
	if out, err := exec.Command(bin, protoRoot).CombinedOutput(); err != nil {
		t.Fatalf("the repo's proto: %v\n%s", err, out)
	}
}
