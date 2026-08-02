.PHONY: fmt vet test race bench frontend cli build desktop verify

GOCACHE ?= /tmp/sheetproof-gocache

fmt:
	go fmt ./...

vet:
	GOCACHE=$(GOCACHE) go vet ./...

test:
	GOCACHE=$(GOCACHE) go test ./...

race:
	GOCACHE=$(GOCACHE) go test -race ./...

bench:
	GOCACHE=$(GOCACHE) go test -bench=. -benchmem ./...

frontend:
	cd frontend && npm run lint && npm run typecheck && npm run test && npm run build

cli:
	GOCACHE=$(GOCACHE) go build -o build/bin/SheetProof .

desktop:
	GOCACHE=$(GOCACHE) go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build

build: desktop

verify: fmt vet test race frontend cli desktop
