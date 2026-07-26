.PHONY: test test-race cover cover-html lint fmt generate outdated db-up db-down

COVERPKGS = $(shell go list ./... | grep -v -e /internal/postgres/db -e /internal/testdb)

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

generate:
	go run ./cmd/pluginwire

COVERDATA = .covdata

cover:
	rm -rf $(COVERDATA)
	mkdir -p $(COVERDATA)/bin $(COVERDATA)/counters
	go build -cover -coverpkg=./cmd/... -o $(COVERDATA)/bin ./cmd/gophenberg ./cmd/pluginwire
	GOPHENBERG_COVER_BINDIR=$(CURDIR)/$(COVERDATA)/bin \
	GOPHENBERG_COVER_GOCOVERDIR=$(CURDIR)/$(COVERDATA)/counters \
	go test -cover $(COVERPKGS) -args -test.gocoverdir=$(CURDIR)/$(COVERDATA)/counters
	@echo "=== merged unit + binary coverage ==="
	go tool covdata percent -i=$(COVERDATA)/counters
	@go tool covdata textfmt -i=$(COVERDATA)/counters -o $(COVERDATA)/cover.out
	@go tool cover -func=$(COVERDATA)/cover.out | tail -1

cover-html: cover
	go tool cover -html=$(COVERDATA)/cover.out

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
