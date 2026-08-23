.PHONY: peers dev seed test test-race cover cover-html lint fmt generate outdated db-up db-down pot catalogs translations \
	translations-retire \
	e2e e2e-build e2e-theme e2e-serve e2e-db-reset e2e-seed e2e-reset bump bump-kit \
	brick-link brick-sync brick-pack brick-unlink

COVERPKGS = $(shell go list ./... | grep -v -e /internal/postgres/db -e /internal/testdb)

# Where a linked package's restore point is kept while it is linked.
BRICK_HELD = .brick-held

brick-link:
	@test -n "$(BRICK)" || { echo "usage: make brick-link BRICK=../some-package"; exit 1; }
	@test -f "$(BRICK)/package.json" || { echo "$(BRICK) holds no package.json"; exit 1; }
	@test ! -d $(BRICK_HELD) || { echo "a package is already linked, run make brick-unlink first"; exit 1; }
	mkdir -p $(BRICK_HELD)
	cp frontend/package.json pnpm-lock.yaml $(BRICK_HELD)/
	printf '%s\n' "$(abspath $(BRICK))" > $(BRICK_HELD)/path
	cd "$(BRICK)" && pnpm run build
	cd frontend && pnpm add "link:$(abspath $(BRICK))"
	@echo "linked $(abspath $(BRICK)), run make brick-sync after editing it"

brick-sync:
	@test -d $(BRICK_HELD) || { echo "no package is linked"; exit 1; }
	cd "$$(cat $(BRICK_HELD)/path)" && pnpm run build
	@echo "rebuilt, the consumer reads it without reinstalling"

brick-pack:
	@test -n "$(BRICK)" || { echo "usage: make brick-pack BRICK=../some-package"; exit 1; }
	@test ! -d $(BRICK_HELD) || { echo "a package is already linked, run make brick-unlink first"; exit 1; }
	mkdir -p $(BRICK_HELD)
	cp frontend/package.json pnpm-lock.yaml $(BRICK_HELD)/
	printf '%s\n' "$(abspath $(BRICK))" > $(BRICK_HELD)/path
	cd "$(BRICK)" && pnpm run build && npm pack --pack-destination "$(CURDIR)/$(BRICK_HELD)"
	cd frontend && pnpm add "file:$$(ls $(CURDIR)/$(BRICK_HELD)/*.tgz | head -1)"
	@echo "installed the packed package, which is what a publish would ship"

brick-unlink:
	@test -d $(BRICK_HELD) || { echo "no package is linked"; exit 1; }
	cp $(BRICK_HELD)/package.json frontend/package.json
	cp $(BRICK_HELD)/pnpm-lock.yaml pnpm-lock.yaml
	rm -rf $(BRICK_HELD)
	pnpm install
	@echo "restored the published package"

bump:
	@test -n "$(V)" || (echo "usage: make bump V=0.2.0" && exit 1)
	printf '%s\n' "$(V)" > internal/version/VERSION

bump-kit:
	@test -n "$(V)" || (echo "usage: make bump-kit V=0.2.0" && exit 1)
	cd sdk/astro && npm version "$(V)" --no-git-tag-version --allow-same-version

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

pot:
	node frontend/scripts/write-pot.ts

translations:
	node --env-file-if-exists=.env frontend/scripts/sync-translations.ts

translations-retire:
	node --env-file-if-exists=.env frontend/scripts/retire-translations.ts

catalogs:
	node frontend/scripts/write-catalogs.ts

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
E2E_PROOF_EMAIL ?= proof@example.com
E2E_PROOF_NAME ?= Ada Lovelace
E2E_PASSWORD ?= correct horse battery
E2E_THEMES_DIR ?= $(CURDIR)/.e2e-themes
E2E_THEME ?= starter
E2E_ARCHIVE_DIR ?= $(CURDIR)/.e2e-archive
E2E_UPLOAD_THEME ?= driftwood
E2E_UPLOAD_VERSION ?= 9.9.9
KIT_VERSION = $(shell node -p "require('./sdk/astro/package.json').version")
E2E_MEDIA_DIR ?= $(CURDIR)/.e2e-media

e2e-build:
	pnpm --filter @gophenberg/frontend build
	pnpm --filter @gophenberg/theme-starter build
	go build -o gophenberg ./cmd/gophenberg

e2e-theme: e2e-build
	rm -rf $(E2E_THEMES_DIR)
	mkdir -p $(E2E_THEMES_DIR)/$(E2E_THEME)
	cp -R test/theme/dist/server test/theme/dist/client test/theme/theme.json \
		$(E2E_THEMES_DIR)/$(E2E_THEME)/

e2e-archive: e2e-build
	rm -rf $(E2E_ARCHIVE_DIR)
	mkdir -p $(E2E_ARCHIVE_DIR)/$(E2E_UPLOAD_THEME)
	cp -R test/theme/dist/server test/theme/dist/client \
		$(E2E_ARCHIVE_DIR)/$(E2E_UPLOAD_THEME)/
	printf '{"name":"%s","version":"%s","kit":"%s"}\n' \
		"$(E2E_UPLOAD_THEME)" "$(E2E_UPLOAD_VERSION)" "$(KIT_VERSION)" \
		> $(E2E_ARCHIVE_DIR)/$(E2E_UPLOAD_THEME)/theme.json
	cd $(E2E_ARCHIVE_DIR)/$(E2E_UPLOAD_THEME) && \
		zip -qr ../$(E2E_UPLOAD_THEME).zip theme.json server client

e2e-media:
	rm -rf $(E2E_MEDIA_DIR)
	mkdir -p $(E2E_MEDIA_DIR)

e2e-serve: db-up e2e-theme e2e-media
	GOPHENBERG_WEB_DIR=frontend/dist GOPHENBERG_DATABASE_URL="$(E2E_DATABASE_URL)" \
		GOPHENBERG_THEMES_DIR="$(E2E_THEMES_DIR)" \
		GOPHENBERG_MEDIA_DIR="$(E2E_MEDIA_DIR)" \
		./gophenberg

e2e-db-reset: db-up
	docker compose exec -T postgres psql -U postgres -v ON_ERROR_STOP=1 \
		-c "DROP DATABASE IF EXISTS $(E2E_DB) WITH (FORCE)" \
		-c "CREATE DATABASE $(E2E_DB)"

e2e-seed: db-up e2e-build
	printf '%s\n' "$(E2E_PASSWORD)" | \
		GOPHENBERG_DATABASE_URL="$(E2E_DATABASE_URL)" ./gophenberg createadmin \
		-email "$(E2E_EMAIL)" -name "$(E2E_NAME)" -role admin
	printf '%s\n' "$(E2E_PASSWORD)" | \
		GOPHENBERG_DATABASE_URL="$(E2E_DATABASE_URL)" ./gophenberg createadmin \
		-email "$(E2E_PROOF_EMAIL)" -name "$(E2E_PROOF_NAME)" -role admin
	GOPHENBERG_DATABASE_URL="$(E2E_DATABASE_URL)" ./gophenberg seed

e2e-reset: e2e-db-reset e2e-seed

e2e: e2e-reset e2e-archive
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
