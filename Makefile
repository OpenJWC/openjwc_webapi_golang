.PHONY: fmt fmt-check vet test race check run build licenses bench

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then echo "以下文件未格式化，请先运行 make fmt："; echo "$$files"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

check: fmt-check vet test

licenses:
	go run ./tools/licenses

build: licenses
	CGO_ENABLED=0 go build -trimpath -o bin/openjwc ./cmd/openjwc

run:
	go run ./cmd/openjwc serve

bench:
	go test -run '^$$' -bench . -benchmem ./internal/infrastructure/sqlite
