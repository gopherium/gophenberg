.PHONY: peers dev seed test test-race cover cover-html lint fmt generate outdated db-up db-down \
	e2e e2e-build e2e-theme e2e-serve e2e-db-reset e2e-seed e2e-reset

COVERPKGS = $(shell go list ./... | grep -v -e /internal/postgres/db -e /internal/testdb)

dev: db-up
	go run ./cmd/gophenberg

seed: db-up
	go run ./cmd/gophenberg seed

test:
	go test ./...

test-race:
	go test -race ./...

peers:
	pnpm peers check

lint:
	golangci-lint run
	go run ./cmd/doclint

fmt:
	golangci-lint fmt

generate:
	go run ./cmd/pluginwire
	go tool sqlc generate

COVERDATA = .covdata

cover:
	rm -rf $(COVERDATA)
	mkdir -p $(COVERDATA)/bin $(COVERDATA)/counters
	go build -cover -coverpkg=./cmd/... -o $(COVERDATA)/bin ./cmd/gophenberg ./cmd/doclint ./cmd/pluginwire
	GOPHENBERG_COVER_BINDIR=$(CURDIR)/$(COVERDATA)/bin \
	GOPHENBERG_COVER_GOCOVERDIR=$(CURDIR)/$(COVERDATA)/counters \
	go test -cover $(COVERPKGS) -args -test.gocoverdir=$(CURDIR)/$(COVERDATA)/counters
	@echo "=== merged unit + binary coverage ==="
	go tool covdata percent -i=$(COVERDATA)/counters
	@echo
	@go tool covdata textfmt -i=$(COVERDATA)/counters -o $(COVERDATA)/cover.out
	@go tool cover -func=$(COVERDATA)/cover.out | tail -1

cover-html: cover
	go tool cover -html=$(COVERDATA)/cover.out

E2E_DB ?= gophenberg_e2e
E2E_DATABASE_URL ?= postgres://postgres:gophenberg@localhost:5435/$(E2E_DB)?sslmode=disable
E2E_EMAIL ?= e2e@example.com
E2E_NAME ?= Grace Hopper
E2E_PASSWORD ?= correct horse battery
E2E_THEMES_DIR ?= $(CURDIR)/.e2e-themes
E2E_THEME ?= starter

e2e-build:
	pnpm --filter @gophenberg/frontend build
	pnpm --filter @gophenberg/theme-starter build
	go build -o gophenberg ./cmd/gophenberg

e2e-theme: e2e-build
	rm -rf $(E2E_THEMES_DIR)
	mkdir -p $(E2E_THEMES_DIR)/$(E2E_THEME)
	cp -R test/theme/dist/server test/theme/dist/client test/theme/theme.json \
		$(E2E_THEMES_DIR)/$(E2E_THEME)/

e2e-serve: db-up e2e-theme
	GOPHENBERG_WEB_DIR=frontend/dist GOPHENBERG_DATABASE_URL="$(E2E_DATABASE_URL)" \
		GOPHENBERG_THEMES_DIR="$(E2E_THEMES_DIR)" GOPHENBERG_THEME="$(E2E_THEME)" \
		./gophenberg

e2e-db-reset: db-up
	docker compose exec -T postgres psql -U postgres -v ON_ERROR_STOP=1 \
		-c "DROP DATABASE IF EXISTS $(E2E_DB) WITH (FORCE)" \
		-c "CREATE DATABASE $(E2E_DB)"

e2e-seed: db-up e2e-build
	printf '%s\n' "$(E2E_PASSWORD)" | \
		GOPHENBERG_DATABASE_URL="$(E2E_DATABASE_URL)" ./gophenberg createadmin \
		-email "$(E2E_EMAIL)" -name "$(E2E_NAME)"
	GOPHENBERG_DATABASE_URL="$(E2E_DATABASE_URL)" ./gophenberg seed

e2e-reset: e2e-db-reset e2e-seed

e2e:
	pnpm --filter @gophenberg/e2e exec playwright test

outdated:
	@echo "=== direct Go modules with updates ==="
	@go list -m -u -f '{{if and (not .Indirect) .Update}}  {{.Path}}: {{.Version}} -> {{.Update.Version}}{{end}}' all 2>/dev/null | grep . || echo "  (all current)"
	@echo "=== pinned tools to review by hand ==="
	@echo "  go directive / installed:  $$(sed -n 's/^go //p' go.mod) / $$(go env GOVERSION)"
	@echo "  also: golangci-lint-action + setup-go, the postgres docker image,"
	@echo "        and @wordpress/* (bleeding-edge train, whole-train batch bumps)"

db-up:
	docker compose up -d --wait

db-down:
	docker compose down
