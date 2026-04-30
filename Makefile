.PHONY: build test clean run vet lint frontend build-all docker-build

BINARY := mailcraftmc
CMD := ./cmd/mailcraft

ifeq ($(OS),Windows_NT)
    NPM := npm.cmd
else
    NPM := npm
endif

build:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY).exe $(CMD)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY)     $(CMD)

test:
	go test -race ./...

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -f ui/app.js ui/app.js.map ui/app.css ui/app.css.map ui/index.html

lint:
	staticcheck ./...

frontend:
	cd web && $(NPM) ci && $(NPM) run build

build-all: frontend build

docker-build:
	docker build -t mailcraft:latest .
