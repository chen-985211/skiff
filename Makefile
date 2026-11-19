.PHONY: run dev test test-integration test-race format-check check-ui check package

run:
	go run ./cmd/skiff

dev:
	go tool mygo dev -sign -

test:
	go test ./tests/unit/... ./tests/contract/...

test-integration:
	go test -tags integration ./tests/integration/... -v

test-race:
	go test -race -timeout=30m -tags integration ./...

format-check:
	@test -z "$$(gofmt -l cmd internal tests)" || (gofmt -l cmd internal tests; exit 1)

check-ui:
	go test -count=1 ./tests/contract -run '^TestInterfacePolicy'
	go test ./tests/unit/transfer

check: format-check
	go vet -tags integration ./...
	go test -run '^TestRender' -tags integration ./...
	go test -race -skip '^TestRender' -tags integration ./...
	go build -o build/skiff ./cmd/skiff

package:
	go tool mygo build -skip-dmg
