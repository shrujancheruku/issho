package retention

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tables lists every table in the database as "schema.table", across every
// schema but Postgres's own: the app's, River's and goose's, and anything
// `public` should never hold (a table there is a finding too). Views and
// sequences hold no rows of their own.
func Tables(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `
		select n.nspname || '.' || c.relname
		from pg_class c join pg_namespace n on n.oid = c.relnamespace
		where c.relkind in ('r', 'p')
		  and n.nspname not in ('pg_catalog', 'information_schema')
		  and n.nspname not like 'pg\_toast%'
		order by 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
