BIN := bin/mbtiles-inspect
PKG := ./cmd/mbtiles-inspect

.PHONY: all build build-linux build-linux-amd64 build-linux-arm64 test vet lint fixtures clean tidy

all: build

build:
	go build -o $(BIN) $(PKG)

build-linux: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/mbtiles-inspect-linux-amd64 $(PKG)

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/mbtiles-inspect-linux-arm64 $(PKG)

test: fixtures
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

fixtures:
	go run ./testdata/gen.go

tidy:
	go mod tidy

clean:
	rm -rf bin testdata/*.mbtiles testdata/*.sqlite
