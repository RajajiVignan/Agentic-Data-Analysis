.PHONY: help install build frontend backend test vet lint typecheck fmt ci docker clean

help:
	@echo "Targets: install build frontend backend test vet lint typecheck fmt ci docker clean"

install:
	npm --prefix frontend ci

frontend:
	npm --prefix frontend run build

backend:
	go build -o server_bin ./cmd/server

build: frontend backend

test:
	go test ./...

vet:
	go vet ./...

lint:
	npm --prefix frontend run lint

typecheck:
	cd frontend && npx tsc --noEmit

fmt:
	gofmt -w .

ci: vet test typecheck lint build

docker:
	docker compose build

clean:
	rm -f server_bin
	rm -rf frontend/out
