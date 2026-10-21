GO     ?= go
BINARY := bin/tspdctl

.PHONY: all build test race vet fmt bench clean

all: test build

build:
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/tspdctl

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

bench:
	$(GO) test -run '^$$' -bench . -benchmem ./internal/securemsg ./internal/answer

clean:
	rm -rf bin
