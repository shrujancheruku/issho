package rpc_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

const throwaway = "/throwaway.v1.ThrowawayService/"

// fakeAuth knows a few tokens, standing in for the verifier of #608.
type fakeAuth map[string]rpc.Identity

func (f fakeAuth) Authenticate(_ context.Context, token string) (rpc.Identity, error) {
	id, ok := f[token]
	if !ok {
		return rpc.Identity{}, errors.New("unknown token")
	}
	return id, nil
}

var throwawayRules = map[string]isshov1.Access{
	"Public":      isshov1.Access_ACCESS_PUBLIC,
	"Member":      isshov1.Access_ACCESS_MEMBER,
	"Making":      isshov1.Access_ACCESS_MAKING,
	"Admin":       isshov1.Access_ACCESS_ADMIN,
	"Undeclared":  isshov1.Access_ACCESS_UNSPECIFIED,
	"Unspecified": isshov1.Access_ACCESS_UNSPECIFIED,
}

// throwawayFiles is the registry of a service that exists only in these tests,
// one RPC per access rule plus one that declares none. Its descriptor is built
// here with the same `access` option a .proto would set.
func throwawayFiles(t *testing.T) *protoregistry.Files {
	t.Helper()
	rules := throwawayRules
	var methods []*descriptorpb.MethodDescriptorProto
	for name, rule := range rules {
		m := &descriptorpb.MethodDescriptorProto{
			Name:       proto.String(name),
			InputType:  proto.String(".google.protobuf.Empty"),
			OutputType: proto.String(".google.protobuf.Empty"),
		}
		if name != "Undeclared" { // that one has no option at all
			m.Options = &descriptorpb.MethodOptions{}
			proto.SetExtension(m.Options, isshov1.E_Access, rule)
		}
		methods = append(methods, m)
	}
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       proto.String("throwaway/v1/throwaway.proto"),
		Package:    proto.String("throwaway.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/empty.proto", "issho/v1/access.proto"},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name:   proto.String("ThrowawayService"),
			Method: methods,
		}},
	}, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}
	files := new(protoregistry.Files)
	if err := files.RegisterFile(fd); err != nil {
		t.Fatal(err)
	}
	return files
}

// throwawayServer serves every throwaway RPC behind the real interceptor.
func throwawayServer(t *testing.T, auth rpc.Authenticator) *httptest.Server {
	t.Helper()
	files := throwawayFiles(t)
	ok := func(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[emptypb.Empty], error) {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}
	mux := http.NewServeMux()
	for name := range throwawayRules {
		mux.Handle(throwaway+name, connect.NewUnaryHandler(throwaway+name, ok,
			connect.WithInterceptors(rpc.NewAccessInterceptor(auth, files))))
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// call invokes one throwaway RPC, with a bearer token when token is not empty.
func call(srv *httptest.Server, name, token string) error {
	client := connect.NewClient[emptypb.Empty, emptypb.Empty](srv.Client(), srv.URL+throwaway+name)
	req := connect.NewRequest(&emptypb.Empty{})
	if token != "" {
		req.Header().Set("Authorization", "Bearer "+token)
	}
	_, err := client.CallUnary(context.Background(), req)
	return err
}

func TestAMemberRPCCalledWithNoTokenIsRefused(t *testing.T) {
	srv := throwawayServer(t, fakeAuth{})

	err := call(srv, "Member", "")

	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("code = %v (err %v), want unauthenticated", got, err)
	}
}

func TestAccessRules(t *testing.T) {
	auth := fakeAuth{
		"member":        {Subject: "m"},
		"making":        {Subject: "k", Making: true},
		"admin1":        {Subject: "a", Admin: true},                   // staff, first step only
		"admin2":        {Subject: "a", Admin: true, SecondStep: true}, // staff, both steps
		"making-admin2": {Subject: "o", Making: true, Admin: true, SecondStep: true},
	}
	srv := throwawayServer(t, auth)

	for _, tc := range []struct {
		name  string
		rpc   string
		token string
		want  connect.Code // 0 means the call succeeds
	}{
		{"public, no token", "Public", "", 0},
		{"public, a token the verifier refuses", "Public", "forged", 0},
		{"member, no token", "Member", "", connect.CodeUnauthenticated},
		{"member, a token the verifier refuses", "Member", "forged", connect.CodeUnauthenticated},
		{"member, a member", "Member", "member", 0},
		{"making, no token", "Making", "", connect.CodeUnauthenticated},
		{"making, a plain member", "Making", "member", connect.CodePermissionDenied},
		{"making, an insider or the owner", "Making", "making", 0},
		{"admin, no token", "Admin", "", connect.CodeUnauthenticated},
		{"admin, a plain member", "Admin", "member", connect.CodePermissionDenied},
		{"admin, staff without the second step", "Admin", "admin1", connect.CodePermissionDenied},
		{"admin, staff with the second step", "Admin", "admin2", 0},
		{"making, staff who is not an insider", "Making", "admin2", connect.CodePermissionDenied},
		// Deny by default: no rule means no one, however well they are signed in.
		{"no rule declared, a fully privileged caller", "Undeclared", "making-admin2", connect.CodeInternal},
		{"no rule declared, no token", "Undeclared", "", connect.CodeInternal},
		{"rule set to unspecified", "Unspecified", "making-admin2", connect.CodeInternal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := call(srv, tc.rpc, tc.token)
			if tc.want == 0 {
				if err != nil {
					t.Fatalf("err = %v, want success", err)
				}
				return
			}
			if got := connect.CodeOf(err); got != tc.want {
				t.Fatalf("code = %v (err %v), want %v", got, err, tc.want)
			}
		})
	}
}

// A token that isn't a bearer token is no token.
func TestAnAuthorizationHeaderThatIsNotBearerIsNoToken(t *testing.T) {
	srv := throwawayServer(t, fakeAuth{"member": {Subject: "m"}})
	client := connect.NewClient[emptypb.Empty, emptypb.Empty](srv.Client(), srv.URL+throwaway+"Member")
	req := connect.NewRequest(&emptypb.Empty{})
	req.Header().Set("Authorization", "Basic member")

	_, err := client.CallUnary(context.Background(), req)

	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("code = %v (err %v), want unauthenticated", got, err)
	}
}

// A handler behind the interceptor learns who is calling.
func TestAHandlerSeesTheCallersIdentity(t *testing.T) {
	var got rpc.Identity
	var seen bool
	auth := fakeAuth{"member": {Subject: "person-1"}}
	mux := http.NewServeMux()
	mux.Handle(throwaway+"Member", connect.NewUnaryHandler(throwaway+"Member",
		func(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[emptypb.Empty], error) {
			got, seen = rpc.IdentityFrom(ctx)
			return connect.NewResponse(&emptypb.Empty{}), nil
		}, connect.WithInterceptors(rpc.NewAccessInterceptor(auth, throwawayFiles(t)))))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	if err := call(srv, "Member", "member"); err != nil {
		t.Fatal(err)
	}
	if !seen || got.Subject != "person-1" {
		t.Fatalf("identity = %+v (seen %v), want subject person-1", got, seen)
	}
}
