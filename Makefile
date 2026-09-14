BINARY := terraform-provider-xcloud
VERSION ?= dev

.PHONY: build test lint
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o dist/$(BINARY) .

test:
	go test ./... -race -count=1

lint:
	@test -z "$$(gofmt -l .)" || (gofmt -l .; exit 1)
	go vet ./...
	terraform fmt -check -recursive examples
