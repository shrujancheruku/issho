//go:build !e2e

package main

import (
	"context"
	"log/slog"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

// newClock is the production clock: the system's. The end-to-end build
// (e2e_on.go) replaces this file, so the image holds no test-only RPC and no
// clock a request can move (cmd/cmd_test.go and scripts/image.sh check it).
func newClock() clock.Clock { return clock.Real{} }

// e2eServices serves nothing beyond the API's own services.
func e2eServices(context.Context, *slog.Logger, clock.Clock, *accounts) ([]rpc.Mount, func(), error) {
	return nil, func() {}, nil
}
