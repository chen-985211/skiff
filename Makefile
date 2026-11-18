.PHONY: run dev test test-integration format-check check package

run:
	go run ./cmd/skiff

dev:
	go tool mygo dev -sign -

test:
	go test ./tests/unit/... ./tests/contract/...

test-integration:
	go test -tags integration ./tests/integration/... -v

format-check:
	@test -z "$$(gofmt -l cmd internal tests)" || (gofmt -l cmd internal tests; exit 1)

check: format-check
	go vet ./...
	go test -race -tags integration ./...
	go build -o build/skiff ./cmd/skiff

package:
	go tool mygo build -skip-dmg
