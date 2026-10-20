# Go tools are pinned in backend/go.mod via `tool` directives (go.work makes them usable from the root).
GOTOOL := go tool

.PHONY: gen gen-proto gen-sql lint test migrate-up migrate-status api seed

gen: gen-proto gen-sql

gen-proto:
	rm -rf backend/gen/issho packages/api-client/src/gen
	$(GOTOOL) buf generate

gen-sql:
	cd backend && go tool sqlc generate

lint:
	$(GOTOOL) buf lint proto
	cd backend && go vet ./... && go tool sqlc vet
	pnpm -r typecheck

test:
	cd backend && go test ./...

migrate-up:
	cd backend && go run ./cmd/tool migrate up

migrate-status:
	cd backend && go run ./cmd/tool migrate status

api:
	cd backend && go run ./cmd/api

seed:
	cd backend && go run ./cmd/tool seed ../data/citypacks/tokyo.json
