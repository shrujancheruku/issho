// Command policylint holds the backend to the rule in docs/spec/architecture.md
// ("The seams"): every number in the rules is a constant in
// backend/internal/policy, linked to the spec document that sets it, and no
// other package writes the number again. `make policy-lint` runs it, and
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
	fs := flag.NewFlagSet("policylint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "the repo root, which holds docs/spec and backend")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	findings, err := Check(*root)
	if err != nil {
		fmt.Fprintln(stderr, "policylint:", err)
		return 2
	}
	for _, f := range findings {
		fmt.Fprintln(stderr, f)
	}
	if len(findings) > 0 {
		fmt.Fprintf(stderr, "policylint: %d finding(s)\n", len(findings))
		return 1
	}
	fmt.Fprintln(stdout, "policylint: ok")
	return 0
}
