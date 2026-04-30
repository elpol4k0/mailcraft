.PHONY: build test clean run vet lint frontend build-all docker-build

BINARY := mailcraft
CMD := ./cmd/mailcraft

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) $(CMD)

test:
	go test -race ./...

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	rm -f ui/app.js ui/app.js.map ui/app.css ui/app.css.map ui/index.html

lint:
	staticcheck ./...

frontend:
	cd web && npm ci && npm run build

build-all: frontend build

docker-build:
	docker build -t mailcraft:latest .
