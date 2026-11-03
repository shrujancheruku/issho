// Package pgconv holds the small conversions between Go values and the pgx
// types sqlc generates for nullable columns.
package pgconv

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Deref returns what a nullable text column holds, or "" for NULL.
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Nullable is a uuid column value that is NULL for the zero uuid.
func Nullable(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil} }

// Time is a non-NULL timestamptz value.
func Time(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
