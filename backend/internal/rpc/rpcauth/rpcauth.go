// Package rpcauth is the check behind `make rpc-auth`: every RPC in the .proto
// files must declare an access rule.
package rpcauth

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// Check compiles every .proto file under dir (imports resolve from dir and from
// imports) and returns the full names of the RPCs that declare no access rule,
// sorted. An RPC that names ACCESS_UNSPECIFIED declares none. An empty result
// means every RPC is covered.
func Check(dir string, imports ...string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".proto") {
			rel, err := filepath.Rel(dir, path)
			files = append(files, filepath.ToSlash(rel))
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// A path typed wrong must not read as "every RPC is covered".
	if len(files) == 0 {
		return nil, fmt.Errorf("no .proto files under %s", dir)
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: append([]string{dir}, imports...),
		}),
	}
	compiled, err := compiler.Compile(context.Background(), files...)
	if err != nil {
		return nil, err
	}

	var missing []string
	for _, file := range compiled {
		services := file.Services()
		for i := 0; i < services.Len(); i++ {
			methods := services.Get(i).Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				rule, err := accessOf(m)
				if err != nil {
					return nil, err
				}
				if rule == isshov1.Access_ACCESS_UNSPECIFIED {
					missing = append(missing, string(m.FullName()))
				}
			}
		}
	}
	sort.Strings(missing)
	return missing, nil
}

// accessOf reads the access option of a method. The compiler may hold a custom
// option as an unknown field, so the options are read back through a round trip
// that resolves it against the generated extension.
func accessOf(m protoreflect.MethodDescriptor) (isshov1.Access, error) {
	raw, err := proto.Marshal(m.Options())
	if err != nil {
		return 0, err
	}
	opts := &descriptorpb.MethodOptions{}
	if err := proto.Unmarshal(raw, opts); err != nil {
		return 0, err
	}
	rule, ok := proto.GetExtension(opts, isshov1.E_Access).(isshov1.Access)
	if !ok {
		return 0, errors.New("access option has an unexpected type")
	}
	return rule, nil
}
