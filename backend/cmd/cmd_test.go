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
	requests := make(chan string, 1)
	go func() {
		addr := regexp.MustCompile(`"addr":"([^"]+)"`)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if m := addr.FindStringSubmatch(sc.Text()); m != nil && strings.Contains(sc.Text(), "started") {
				addrs <- m[1]
				break
			}
		}
		for sc.Scan() {
			if strings.Contains(sc.Text(), `"msg":"request"`) {
				requests <- sc.Text()
			}
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

	// The process logs the request through the shared logger: the route and
	// nothing of the caller, whose address was 127.0.0.1.
	select {
	case line := <-requests:
		if !strings.Contains(line, `"route":"/issho.v1.HealthService/Check"`) {
			t.Errorf("request line has no route: %s", line)
		}
		if strings.Contains(line, "127.0.0.1") {
			t.Errorf("request line holds the caller's address: %s", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the API logged no request line")
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

// The API needs its database and its sign-in together: with only one, it says
// which is missing and exits 1 rather than serving accounts it can't keep or
// can't check.
func TestAPIWithHalfOfItsAccountsConfigExitsWithAnError(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "api")
	if out, err := exec.Command("go", "build", "-o", bin, "./api").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for name, tc := range map[string]struct{ env, missing string }{
		"no sign-in":  {"DATABASE_URL=postgres://localhost/x", "SUPABASE_AUTH_URL"},
		"no database": {"SUPABASE_AUTH_URL=http://127.0.0.1:1/auth/v1", "DATABASE_URL"},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(bin)
			cmd.Env = append(os.Environ(), "API_ADDR=127.0.0.1:0", "DATABASE_URL=", "SUPABASE_AUTH_URL=", tc.env)
			out, err := cmd.CombinedOutput()
			if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
				t.Fatalf("err = %v, want exit status 1; output:\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.missing) {
				t.Fatalf("output does not name %s:\n%s", tc.missing, out)
			}
		})
	}
}

// With its database and sign-in configured the API serves accounts, behind the
// token check: a call with no good token is refused as unauthenticated (an API
// that didn't serve accounts would answer unimplemented).
func TestAPIWithAccountsConfiguredServesThemBehindTheTokenCheck(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "api")
	if out, err := exec.Command("go", "build", "-o", bin, "./api").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	pool := testdb.NewDatabase(t)
	if err := db.Up(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin)
	// The key set's address is never reached: the token is refused first.
	cmd.Env = append(os.Environ(), "API_ADDR=127.0.0.1:0", "DATABASE_URL="+testdb.URL(pool), "SUPABASE_AUTH_URL=http://127.0.0.1:1/auth/v1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	addrs := make(chan string, 1)
	go func() {
		addr := regexp.MustCompile(`"addr":"([^"]+)"`)
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

	client := isshov1connect.NewAccountServiceClient(http.DefaultClient, "http://"+addr)
	req := connect.NewRequest(&isshov1.GetAccountRequest{})
	req.Header().Set("Authorization", "Bearer forged")
	_, err = client.GetAccount(context.Background(), req)
	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("GetAccount with a forged token: code %v (%v), want unauthenticated", got, err)
	}
}

const fixturesPkg = "github.com/shrujancheruku/issho/backend/fixtures"

// depsOf lists every package the commands are built from, tests excluded.
func depsOf(t *testing.T, commands ...string) string {
	t.Helper()
	args := []string{"list", "-deps"}
	for _, c := range commands {
		args = append(args, "./"+c)
	}
	out, err := exec.Command("go", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	return string(out)
}

// Nothing loads the fixture in production: the image holds api and worker, and
// neither is built from the fixtures. migrate is the third command the
// deploy might run, and it doesn't either. Only seed, which the image doesn't
// hold, imports them.
func TestProductionProcessesDoNotImportTheFixtures(t *testing.T) {
	for _, name := range []string{"api", "worker", "migrate"} {
		if strings.Contains(depsOf(t, name), fixturesPkg+"\n") {
			t.Errorf("cmd/%s is built from the fixtures package", name)
		}
	}
	if !strings.Contains(depsOf(t, "seed"), fixturesPkg+"\n") {
		t.Error("cmd/seed is not built from the fixtures package: this test checks nothing")
	}
	dockerfile, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dockerfile), "cmd/seed") {
		t.Error("the image builds cmd/seed")
	}
}

func buildSeed(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "seed")
	if out, err := exec.Command("go", "build", "-o", bin, "./seed").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// `seed` loads the fixture city and plans into a migrated local database, and
// can be run again.
func TestSeedLoadsTheFixtureIntoALocalDatabase(t *testing.T) {
	bin := buildSeed(t)
	pool := testdb.NewDatabase(t)
	if err := db.Up(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		cmd := exec.Command(bin)
		cmd.Env = append(os.Environ(), "DATABASE_URL="+testdb.URL(pool))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seed: %v\n%s", err, out)
		}
	}
	var cities, plans int
	if err := pool.QueryRow(context.Background(), `select (select count(*) from app.cities), (select count(*) from app.plans)`).Scan(&cities, &plans); err != nil {
		t.Fatal(err)
	}
	if cities != 1 || plans != 2 {
		t.Fatalf("cities, plans = %d, %d; want 1, 2", cities, plans)
	}
}

// `seed` is for development: it refuses a database that isn't on this machine,
// before it connects, and says so without printing the URL (it holds the
// password).
func TestSeedRefusesADatabaseThatIsNotLocal(t *testing.T) {
	bin := buildSeed(t)
	for _, url := range []string{
		"postgresql://postgres:hunter2@db.abcdefghij.supabase.co:5432/postgres",
		"postgresql://postgres.abcdefghij:hunter2@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres",
		"postgresql://postgres:hunter2@10.0.0.5/postgres",
		"postgresql://postgres:hunter2@127.0.0.1/postgres?host=db.abcdefghij.supabase.co",
		"not a url",
		"",
	} {
		cmd := exec.Command(bin)
		cmd.Env = append(os.Environ(), "DATABASE_URL="+url)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("seed accepted %q", url)
		}
		if strings.Contains(string(out), "hunter2") {
			t.Errorf("seed printed the password:\n%s", out)
		}
	}
}

// The hosted-dev opt-in does not open the door to production: with the flag
// and a dev ref set, a target that is not that project's is refused before it
// connects, and the password is not printed. Without SEED_DEV_PROJECT_REF the
// flag alone reaches nothing.
func TestSeedHostedDevOptInRefusesAProductionLikeTarget(t *testing.T) {
	bin := buildSeed(t)
	const devRef = "abcdefghijklmnopqrst"
	for _, tc := range []struct{ url, ref string }{
		{"postgresql://postgres:hunter2@db.zzzzzzzzzzzzzzzzzzzz.supabase.co:5432/postgres", devRef},
		{"postgresql://postgres.zzzzzzzzzzzzzzzzzzzz:hunter2@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres", devRef},
		{"postgresql://postgres:hunter2@db." + devRef + ".supabase.co:5432/postgres", ""},
	} {
		cmd := exec.Command(bin, "--hosted-dev")
		cmd.Env = append(os.Environ(), "DATABASE_URL="+tc.url, "SEED_DEV_PROJECT_REF="+tc.ref)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("seed --hosted-dev accepted %q with ref %q", tc.url, tc.ref)
		}
		if strings.Contains(string(out), "hunter2") {
			t.Errorf("seed printed the password:\n%s", out)
		}
	}
}

// With --hosted-dev the seed also reads the database's own environment marker
// (`issho.environment`) before writing anything, and goes on only if it is
// exactly `dev`: the operator supplies the ref, but not what is stored in the
// database. The test database is local, so the target check passes and only the
// marker decides. No marker and `prod` are refused with nothing written; `dev`
// seeds. Without the flag the marker is never read: a local seed needs none.
func TestSeedHostedDevNeedsTheDevEnvironmentMarker(t *testing.T) {
	bin := buildSeed(t)
	for _, tc := range []struct {
		name   string
		marker string // "" sets none
		seeds  bool
	}{
		{"no marker", "", false},
		{"prod", "prod", false},
		{"dev with other case", "Dev", false},
		{"dev", "dev", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := testdb.NewDatabase(t)
			ctx := context.Background()
			if err := db.Up(ctx, pool); err != nil {
				t.Fatal(err)
			}
			if tc.marker != "" {
				if _, err := pool.Exec(ctx, `do $$ begin execute format('alter database %I set issho.environment = %L', current_database(), '`+tc.marker+`'); end $$`); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(bin, "--hosted-dev")
			cmd.Env = append(os.Environ(), "DATABASE_URL="+testdb.URL(pool))
			out, err := cmd.CombinedOutput()
			var cities int
			if qerr := pool.QueryRow(ctx, `select count(*) from app.cities`).Scan(&cities); qerr != nil {
				t.Fatal(qerr)
			}
			if tc.seeds {
				if err != nil || cities != 1 {
					t.Fatalf("seed: err %v, cities %d\n%s", err, cities, out)
				}
				return
			}
			if err == nil || cities != 0 {
				t.Fatalf("seed went ahead without the dev marker: err %v, cities %d\n%s", err, cities, out)
			}
			if !strings.Contains(string(out), "issho.environment") {
				t.Errorf("refusal does not name the marker:\n%s", out)
			}
		})
	}
}

// A session can set `issho.environment` itself: a URL's `options` parameter, or
// PGOPTIONS in the environment, sends `-c issho.environment=dev` at connect. The
// marker that counts is the one stored on the database, so none of these may
// seed an unmarked database. And a --hosted-dev URL that sets any runtime
// parameter is refused before it connects, even on a database that is marked,
// so nothing the session says is ever in play. The refusal never prints the URL.
func TestSeedHostedDevRefusesASessionThatSetsTheMarker(t *testing.T) {
	bin := buildSeed(t)
	const options = "&options=-c%20issho.environment%3Ddev"
	for _, tc := range []struct {
		name   string
		query  string   // appended to the database's URL
		env    []string // extra environment
		marked bool
	}{
		{"options in the URL, database unmarked", options, nil, false},
		{"options in the URL, database marked", options, nil, true},
		{"another runtime parameter, database marked", "&application_name=seed", nil, true},
		{"PGOPTIONS, database unmarked", "", []string{"PGOPTIONS=-c issho.environment=dev"}, false},
		{"PGOPTIONS, database marked", "", []string{"PGOPTIONS=-c issho.environment=dev"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := testdb.NewDatabase(t)
			ctx := context.Background()
			if err := db.Up(ctx, pool); err != nil {
				t.Fatal(err)
			}
			if tc.marked {
				if _, err := pool.Exec(ctx, `do $$ begin execute format('alter database %I set issho.environment = %L', current_database(), 'dev'); end $$`); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(bin, "--hosted-dev")
			cmd.Env = append(append(os.Environ(), "DATABASE_URL="+testdb.URL(pool)+tc.query), tc.env...)
			out, err := cmd.CombinedOutput()
			var cities int
			if qerr := pool.QueryRow(ctx, `select count(*) from app.cities`).Scan(&cities); qerr != nil {
				t.Fatal(qerr)
			}
			if err == nil || cities != 0 {
				t.Fatalf("seed went ahead with a session that sets parameters: err %v, cities %d\n%s", err, cities, out)
			}
			if strings.Contains(string(out), "issho.environment=dev") && strings.Contains(string(out), "options") {
				t.Errorf("refusal echoes the URL:\n%s", out)
			}
		})
	}
}
