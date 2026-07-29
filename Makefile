.PHONY: fmt vet test check run

fmt:
	gofmt -w $$(find . -name '*.go' -type f)

vet:
	go vet ./...

test:
	go test ./...

check: fmt vet test

run:
	go run ./cmd/openjwc-api
