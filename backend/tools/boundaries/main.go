// Command boundaries holds the backend to the rules in docs/spec/architecture.md
// ("The seams"): a package reaches another only through its exported Go API,
// and outside services are wired only in cmd/. `make boundaries` runs it, and
// `make lint` runs that.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run returns the exit code: 0 clean, 1 findings, 2 the lint couldn't run.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("boundaries", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "the repo root, which holds backend")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	findings, err := Check(*root)
	if err != nil {
		fmt.Fprintln(stderr, "boundaries:", err)
		return 2
	}
	for _, f := range findings {
		fmt.Fprintln(stderr, f)
	}
	if len(findings) > 0 {
		fmt.Fprintf(stderr, "boundaries: %d finding(s)\n", len(findings))
		return 1
	}
	fmt.Fprintln(stdout, "boundaries: ok")
	return 0
}
