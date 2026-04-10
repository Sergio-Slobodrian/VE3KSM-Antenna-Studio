.PHONY: dev-backend dev-frontend dev run build-backend build-frontend build-launcher build clean test

# Development — single command
run:
	cd backend && go run ./cmd/launcher

# Development — separate terminals
dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

dev:
	@echo "Run 'make dev-backend' and 'make dev-frontend' in separate terminals"
	@echo "Or just 'make run' to start both at once."

# Build
build-launcher:
	cd backend && go build -o ../bin/antenna-studio ./cmd/launcher

build-backend:
	cd backend && go build -o ../bin/antenna-server ./cmd/server

build-frontend:
	cd frontend && npm run build

build: build-backend build-frontend build-launcher

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
