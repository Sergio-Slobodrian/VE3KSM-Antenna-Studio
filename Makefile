.PHONY: run dev deps build test clean docker-up docker-down sync update-itu-zones

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

# -----------------------------------------------------------------------------
# ITU-R P.832 zone data import.
#
# Rewrites frontend/src/data/itu_r_p832.json from an external GeoJSON source
# (ITU atlas conversion, per-region dataset, Natural Earth overlay, ...).
# Each feature is classified into zone 1-6 by properties.zone (preferred) or
# properties.sigma (fallback, via canonical σ thresholds).  Polygon rings are
# simplified with Douglas-Peucker; MultiPolygons are split per outer ring;
# holes are discarded.
#
# Required:  SRC=/path/to/source.geojson
# Optional:  MERGE=1         append to existing out file (dedup by id)
#            EPS=0.05        simplification epsilon in degrees (0 = off)
#
# Examples:
#   make update-itu-zones SRC=/tmp/itu-atlas.geojson
#   make update-itu-zones SRC=/tmp/north-america.geojson MERGE=1
#   make update-itu-zones SRC=/tmp/source.geojson EPS=0.1
#   make update-itu-zones SRC=/tmp/hires.geojson EPS=0
# -----------------------------------------------------------------------------
SRC ?=
MERGE ?=
EPS ?= 0.05
ITU_OUT := frontend/src/data/itu_r_p832.json

update-itu-zones:
	@if [ -z "$(SRC)" ]; then \
		echo "error: SRC=<path/to/source.geojson> is required"; \
		echo "example: make update-itu-zones SRC=/tmp/itu-atlas.geojson"; \
		exit 1; \
	fi
	cd backend && go run ./cmd/ituimport \
		-src "$(abspath $(SRC))" \
		-out "$(abspath $(ITU_OUT))" \
		-eps "$(EPS)" \
		$(if $(MERGE),-merge,)
