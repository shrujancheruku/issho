package push

import (
	"context"
	"errors"
	"regexp"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
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

func (h *Handler) RegisterPushToken(ctx context.Context, req *isshov1.RegisterPushTokenRequest) (*isshov1.RegisterPushTokenResponse, error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if !expoToken.MatchString(req.Token) {
		return nil, errs.Invalid("token: not an Expo push token")
	}
	platform, ok := platforms[req.Platform]
	if !ok {
		return nil, errs.Invalid("platform: ios, android or web")
	}
	err = h.Q.UpsertPushToken(ctx, dbgen.UpsertPushTokenParams{Token: req.Token, UserID: c.UserID, Platform: platform})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		return nil, errs.Refused("no account: call IdentityService.GetMe first")
	}
	if err != nil {
		return nil, err
	}
	return &isshov1.RegisterPushTokenResponse{}, nil
}

func (h *Handler) UnregisterPushToken(ctx context.Context, req *isshov1.UnregisterPushTokenRequest) (*isshov1.UnregisterPushTokenResponse, error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.Q.DeletePushToken(ctx, dbgen.DeletePushTokenParams{Token: req.Token, UserID: c.UserID}); err != nil {
		return nil, err
	}
	return &isshov1.UnregisterPushTokenResponse{}, nil
}
