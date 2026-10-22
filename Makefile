.PHONY: build run test web clean

build: web
	go build -o ace ./cmd/ace

web:
	cd web && npm run build

run: build
	./ace

test:
	go test -v ./internal/... ./cmd/...

clean:
	rm -f ace
	rm -rf web/build web/.svelte-kit
