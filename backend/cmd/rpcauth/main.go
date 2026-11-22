// Command rpcauth is `make rpc-auth`: it exits 1 and names each RPC under the
// proto directory that declares no access rule.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/shrujancheruku/issho/backend/internal/rpc/rpcauth"
)

type dirs []string

func (d *dirs) String() string     { return strings.Join(*d, ",") }
func (d *dirs) Set(v string) error { *d = append(*d, v); return nil }

func main() {
	var imports dirs
	flag.Var(&imports, "I", "extra directory to resolve imports from (repeatable)")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: rpcauth [-I importdir]... protodir")
		os.Exit(2)
	}

	missing, err := rpcauth.Check(flag.Arg(0), imports...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rpcauth:", err)
		os.Exit(2)
	}
	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, "rpcauth: these RPCs declare no access rule; add `option (issho.v1.access) = ACCESS_...;` (see proto/README.md):")
		for _, name := range missing {
			fmt.Fprintln(os.Stderr, "  "+name)
		}
		os.Exit(1)
	}
}
