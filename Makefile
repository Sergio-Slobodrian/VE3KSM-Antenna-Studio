.PHONY: dev-backend dev-frontend dev build-backend build-frontend build clean test

# Development
dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

dev:
	@echo "Run 'make dev-backend' and 'make dev-frontend' in separate terminals"

# Build
build-backend:
	cd backend && go build -o ../bin/antenna-server ./cmd/server

build-frontend:
	cd frontend && npm run build

build: build-backend build-frontend

# Test
test:
	cd backend && go test ./...

# Clean
clean:
	rm -rf bin/ frontend/dist/

# Docker
docker-up:
	docker-compose up --build

docker-down:
	docker-compose down
