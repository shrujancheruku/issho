package rpcauth_test

import (
	"reflect"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/rpc/rpcauth"
)

// The fixtures import issho/v1/access.proto, which lives in the real proto/.
const protoRoot = "../../../../proto"

func TestAnRPCWithNoDeclaredRuleIsReported(t *testing.T) {
	got, err := rpcauth.Check("testdata/undeclared", protoRoot)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"fixture.v1.UnruledService.Forgotten",
		"fixture.v1.UnruledService.OtherOption",
		"fixture.v1.UnruledService.Unspecified",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("undeclared = %v, want %v", got, want)
	}
}

func TestEveryRPCWithARuleIsAccepted(t *testing.T) {
	got, err := rpcauth.Check("testdata/declared", protoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("undeclared = %v, want none", got)
	}
}

// The API's own proto is held to the same rule.
func TestEveryRPCInTheRepoDeclaresARule(t *testing.T) {
	got, err := rpcauth.Check(protoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("undeclared = %v, want none", got)
	}
}

func TestAProtoThatDoesNotCompileIsAnError(t *testing.T) {
	if _, err := rpcauth.Check("testdata/does-not-exist", protoRoot); err == nil {
		t.Fatal("want an error for a directory that isn't there")
	}
}

// A directory with no .proto files is a mistyped path, not a clean bill.
func TestADirectoryWithNoProtoFilesIsAnError(t *testing.T) {
	if _, err := rpcauth.Check(t.TempDir(), protoRoot); err == nil {
		t.Fatal("want an error for a directory with no .proto files")
	}
}
