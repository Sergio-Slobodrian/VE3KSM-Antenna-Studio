# syntax=docker/dockerfile:1
#
# Multi-stage build for VE3KSM Antenna Studio.
#
# Stage 1 (deps): Use a Node image ONLY to run `npm install` so that
# esbuild (running later, inside the Go process) can resolve React,
# Three.js, Recharts, Zustand, etc. from disk.  Node is NOT present in
# the final image — o