.PHONY: test test-race lint fmt outdated db-up db-down

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

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
