# syntax=docker/dockerfile:1
#
# Multi-stage build for Antenna Studio.
#
# Stage 1 (deps): Use a Node image ONLY to run `npm install` so that
# esbuild (running later, inside the Go process) can resolve React,
# Three.js, Recharts, Zustand, etc. from disk.  Node is NOT present in
# the final image — only the files it fetched.
#
# Stage 2 (go-build): Build the single Go binary.
#
# Stage 3 (runtime): Minimal alpine image with the Go binary plus the
# frontend source tree + node_modules.  The binary bundles TS/TSX at
# startup using esbuild's Go library; no Node, Vite, or nginx is ever
# executed at runtime.

FROM node:20-alpine AS deps
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --no-audit --no-fund

FROM golang:1.22-alpine AS go-build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 go build -o /out/antenna-studio ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=go-build /out/antenna-studio /app/antenna-studio
COPY frontend/index.html      /app/frontend/index.html
COPY frontend/tsconfig.json   /app/frontend/tsconfig.json
COPY frontend/src             /app/frontend/src
COPY frontend/public          /app/frontend/public
COPY --from=deps /frontend/node_modules /app/frontend/node_modules
EXPOSE 8080
ENV PORT=8080
CMD ["/app/antenna-studio", "-frontend-dir", "/app/frontend"]
