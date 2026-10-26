package push

import (
	"context"
	"errors"
	"regexp"

	"connectrpc.com/connect"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// Handler serves PushService.
type Handler struct {
	Q *dbgen.Queries
}

// expoToken matches Expo push tokens: ExponentPushToken[…] (older) or
// ExpoPushToken[…]. Checked here as well as in the database so a bad token
// is an invalid argument, not an internal error.
var expoToken = regexp.MustCompile(`^Expo(nent)?PushToken\[[A-Za-z0-9_-]{1,200}\]$`)

var platforms = map[isshov1.PushPlatform]string{
	isshov1.PushPlatform_PUSH_PLATFORM_IOS:     "ios",
	isshov1.PushPlatform_PUSH_PLATFORM_ANDROID: "android",
	isshov1.PushPlatform_PUSH_PLATFORM_WEB:     "web",
}

func (h *Handler) RegisterPushToken(ctx context.Context, req *connect.Request[isshov1.RegisterPushTokenRequest]) (*connect.Response[isshov1.RegisterPushTokenResponse], error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if !expoToken.MatchString(req.Msg.Token) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token: not an Expo push token"))
	}
	platform, ok := platforms[req.Msg.Platform]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("platform: ios, android or web"))
	}
	err = h.Q.UpsertPushToken(ctx, dbgen.UpsertPushTokenParams{Token: req.Msg.Token, UserID: c.UserID, Platform: platform})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no account: call IdentityService.GetMe first"))
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.RegisterPushTokenResponse{}), nil
}

func (h *Handler) UnregisterPushToken(ctx context.Context, req *connect.Request[isshov1.UnregisterPushTokenRequest]) (*connect.Response[isshov1.UnregisterPushTokenResponse], error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.Q.DeletePushToken(ctx, dbgen.DeletePushTokenParams{Token: req.Msg.Token, UserID: c.UserID}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.UnregisterPushTokenResponse{}), nil
}
