.PHONY: fmt vet test race check run build licenses bench

fmt:
	gofmt -w $$(find cmd internal migrations tools -name '*.go' -type f)

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

check: fmt vet test

licenses:
	go run ./tools/licenses

build: licenses
	CGO_ENABLED=0 go build -trimpath -o bin/openjwc ./cmd/openjwc

run:
	go run ./cmd/openjwc serve

bench:
	go test -run '^$$' -bench . -benchmem ./internal/infrastructure/sqlite
