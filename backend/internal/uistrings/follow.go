package uistrings

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/repoedit"
)

// HumanSource reads the human-version files on the default branch
// (*repoedit.Repo).
type HumanSource interface {
	List(ctx context.Context, dir string) ([]string, error)
	Read(ctx context.Context, path string) ([]byte, bool, error)
}

// LoadHuman replaces the human versions with the default branch's
// (HumanDir). On any error nothing changes.
func (c *Catalog) LoadHuman(ctx context.Context, src HumanSource) error {
	names, err := src.List(ctx, HumanDir)
	if err != nil {
		return err
	}
	files := map[string][]byte{}
	for _, name := range names {
		lang, ok := strings.CutSuffix(name, ".json")
		if !ok || !i18n.ValidDisplay(lang) {
			continue
		}
		raw, ok, err := src.Read(ctx, HumanDir+"/"+name)
		if err != nil {
			return err
		}
		if ok { // merged away since the listing: nothing to load
			files[lang] = raw
		}
	}
	return c.SetHuman(files)
}

// Start reads the human versions once, then follows the default branch in
// the background (Follow). A refused token (repoedit.TokenRefused) is a
// startup error when strict, which production sets, so a rotated or mistyped
// GITHUB_TOKEN is found by the deploy rather than after the fact; elsewhere,
// and for any other failure, it only warns and the built-in copies stay.
func (c *Catalog) Start(ctx context.Context, src HumanSource, every time.Duration, log *slog.Logger, strict bool) error {
	if err := c.LoadHuman(ctx, src); err != nil {
		if strict && repoedit.TokenRefused(err) {
			return fmt.Errorf("ui strings: GITHUB_TOKEN was refused: %w", err)
		}
		if ctx.Err() == nil {
			log.Warn("ui strings: couldn't read the human versions", "err", err)
		}
	}
	go c.follow(ctx, src, every, log, true)
	return nil
}

// Follow keeps the human versions in step with the default branch, reading it every
// interval until ctx ends, so a merged pull request from the translation
// editor reaches the app (whose check runs hourly) without a deploy. A
// failed read keeps what's loaded. Versions written for English newer than
// this build's are stale here (Current) and skipped, so following a branch
// ahead of the deployed API is safe.
func (c *Catalog) Follow(ctx context.Context, src HumanSource, every time.Duration, log *slog.Logger) {
	c.follow(ctx, src, every, log, false)
}

// follow is Follow; skipFirst waits an interval before the first read
// (Start has just made it).
func (c *Catalog) follow(ctx context.Context, src HumanSource, every time.Duration, log *slog.Logger, skipFirst bool) {
	for {
		if !skipFirst {
			if err := c.LoadHuman(ctx, src); err != nil && ctx.Err() == nil {
				log.Warn("ui strings: couldn't read the human versions", "err", err)
			}
		}
		skipFirst = false
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}
