.PHONY: run dev deps build test clean docker-up docker-down sync

# -----------------------------------------------------------------------------
# sync: pull changes from the Windows-side copy of this repo into the WSL
# filesystem we're currently in.  Working through a symlink into /mnt/c is
# painfully slow for Go builds, npm, and esbuild, so we keep a WSL-native
# working tree and rsync the Windows copy into it on demand.
#
# Override the source path with:  make sync WIN_SRC=/mnt/c/path/to/repo
# Dry-run before syncing for real:  make sync DRY=1
# -----------------------------------------------------------------------------
WIN_SRC ?= /mnt/c/Users/User/Documents/WSLProjects/AntennaDesigner
RSYNC_FLAGS := -av --delete \
	--exclude=.git/ \
	--exclude=node_modules/ \
	--exclude=bin/ \
	--exclude=frontend/dist/ \
	--exclude='*.exe' \
	--exclude='.DS_Store'
ifdef DRY
RSYNC_FLAGS += --dry-run
endif

sync:
	@if [ ! -d "$(WIN_SRC)" ]; then \
		echo "error: WIN_SRC='$(WIN_SRC)' not found. Override with: make sync WIN_SRC=/mnt/c/..."; \
		exit 1; \
	fi
	rsync $(RSYNC_FLAGS) "$(WIN_SRC)/" ./

# -----------------------------------------------------------------------------
# One Go binary serves the API and the compiled TypeScript frontend.  There
# is no Node.js server in the runtime path; esbuild (Go library) bundles the
# TS/TSX source inside the backend process before any byte reaches the browser.
# -----------------------------------------------------------------------------

# Fetch frontend npm dependencies (React, Three, Recharts, Zustand).  These
# populate frontend/node_modules/ which esbuild resolves against.  Node is
# NOT required at runtime — only during this one-time install.
deps:
	cd frontend && npm install --no-audit --no-fund

# Run the server in dev mode: the frontend is re-bundled on every request.
run: deps
	cd backend && go run ./cmd/server -dev

# Alias so old muscle-memory still works.
dev: run

# Build a single production binary at ./bin/antenna-studio.
build: deps
	cd backend && go build -o ../bin/antenna-studio ./cmd/server

# Test
test:
	cd backend && go test ./...

clean:
	rm -rf bin/

# Docker
docker-up:
	docker-compose up --build

docker-down:
	docker-compose down
