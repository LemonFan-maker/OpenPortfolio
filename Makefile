.PHONY: dev-server dev-web test fmt

dev-server:
	cd apps/server && go run ./cmd/server

dev-web:
	npm --workspace apps/web run dev

test:
	cd apps/server && go test ./...
	npm --workspace apps/web run lint

fmt:
	cd apps/server && gofmt -w ./cmd ./internal
