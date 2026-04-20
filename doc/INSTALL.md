# Antenna Studio — Installation Guide

This guide walks you through every step needed to install, build, and run
**VE3KSM Antenna Studio** on Windows using the Windows Subsystem for Linux
(WSL2) with Ubuntu 24.04.

By the end you will have a browser pointing to `http://localhost:8080` and a
working antenna simulator.

---

## Table of Contents

1. [System Requirements](#1-system-requirements)
2. [Part 1 — Install WSL2 with Ubuntu 24.04](#2-part-1--install-wsl2-with-ubuntu-2404)
3. [Part 2 — Install Build Prerequisites](#3-part-2--install-build-prerequisites)
4. [Part 3 — Clone, Build, and Test](#4-part-3--clone-build-and-test)
5. [Part 4 — Run and Verify](#5-part-4--run-and-verify)
6. [Troubleshooting](#6-troubleshooting)

---

## 1. System Requirements

| Requirement | Minimum |
|---|---|
| OS | Windows 10 (build 19041+) or Windows 11 |
| RAM | 4 GB |
| Disk space | ~2 GB (WSL image + Go toolchain + npm cache) |
| Internet | Required for downloads |

> **Tip:** To find your Windows build number, press **Win + R**, type `winver`,
> and press Enter.

---

## 2. Part 1 — Install WSL2 with Ubuntu 24.04

### Check if WSL is already installed

Open **PowerShell** (no Administrator needed for this check) and run:

```powershell
wsl --list --verbose
```

If you see `Ubuntu-24.04` with `VERSION 2` in the output, WSL is already set
up correctly — skip ahead to [Part 2](#3-part-2--install-build-prerequisites).

```
  NAME            STATE           VERSION
* Ubuntu-24.04    Stopped         2        ← already good
```

### Option A — Automated (recommended)

The script `doc/scripts/setup-wsl.ps1` handles everything: it checks your
Windows version, ensures WSL2 is the default, installs Ubuntu 24.04 if
missing, and upgrades any existing installation to WSL version 2.

1. Open **PowerShell as Administrator**
   (right-click the Start button → *Windows PowerShell (Admin)*)

2. Allow local scripts to run (one-time setting):
   ```powershell
   Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
   ```

3. Navigate to the repo root, then run the script:
   ```powershell
   .\doc\scripts\setup-wsl.ps1
   ```

4. If the script prints a reboot message, reboot and re-run it.

### Option B — Manual steps

1. Open **PowerShell as Administrator**.

2. Install WSL2 with Ubuntu 24.04:
   ```powershell
   wsl --install -d Ubuntu-24.04
   ```

3. If prompted to reboot, do so, then continue.

4. Launch **Ubuntu 24.04** from the Start Menu. On the first launch it will
   ask you to create a Linux username and password — choose anything you like.

5. Confirm the distribution is using WSL version 2:
   ```powershell
   wsl --list --verbose
   ```
   If Ubuntu-24.04 shows `VERSION 1`, upgrade it:
   ```powershell
   wsl --set-version Ubuntu-24.04 2
   ```

---

## 3. Part 2 — Install Build Prerequisites

Antenna Studio's backend is written in Go and the frontend uses TypeScript
bundled in-process by the Go binary. You need:

| Tool | Minimum version | Purpose |
|---|---|---|
| Go | 1.22 | Compile the backend and bundle the frontend |
| Node.js | 18 | Run `npm install` once to populate `node_modules` |
| npm | bundled with Node.js | Install frontend packages |
| git | any recent | Clone the repository |
| make | any recent | Build automation |

> Node.js is **not** used at runtime — only to resolve npm package references
> that the Go esbuild bundler needs.

### Option A — Automated (recommended)

Inside your Ubuntu terminal:

```bash
# If you don't have the repo yet, skip to Part 3 and run this script afterwards.
# If you already have the repo cloned:
chmod +x doc/scripts/setup-ubuntu.sh
./doc/scripts/setup-ubuntu.sh
```

The script installs git, make, curl, Go 1.24 (from the official tarball),
and Node.js 20 LTS (via NodeSource), then prints a version summary.

### Option B — Manual steps

Open the Ubuntu terminal and run each block in order.

#### Update packages

```bash
sudo apt update && sudo apt upgrade -y
```

#### Install git and make

```bash
sudo apt install -y git make curl
```

#### Install Go 1.24

> **Do not use `apt install golang`** — Ubuntu 24.04's apt repository ships
> an older version that may be below the required minimum.

```bash
# Download the official tarball
curl -fsSL https://go.dev/dl/go1.24.0.linux-amd64.tar.gz -o /tmp/go.tar.gz

# Remove any existing Go installation and extract the new one
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tar.gz

# Add Go to your PATH (permanent — applies to new shells)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# Apply immediately in the current shell
source ~/.bashrc
```

Verify:
```bash
go version
# Expected: go version go1.24.0 linux/amd64
```

#### Install Node.js 20 LTS

```bash
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs
```

Verify:
```bash
node --version   # Expected: v20.x.x
npm --version    # Expected: 10.x.x or similar
```

#### Verify all tools

```bash
go version && node --version && npm --version && git --version && make --version
```

All four commands should print version strings without errors.

---

## 4. Part 3 — Clone, Build, and Test

All commands below run inside the **Ubuntu terminal**.

### Clone the repository

```bash
git clone https://github.com/Sergio-Slobodrian/VE3KSM-Antenna-Studio.git
cd VE3KSM-Antenna-Studio
```

### Install frontend dependencies (one-time)

```bash
make deps
```

This runs `npm install` inside `frontend/` and populates `frontend/node_modules/`
so the Go bundler can resolve React, Three.js, Recharts, and other packages.
You only need to run this once (and again if `frontend/package.json` changes).

### Build the production binary

```bash
make build
```

This compiles the Go backend and the TypeScript frontend together into a single
self-contained binary at `./bin/antenna-studio`.

Expected output (last line):
```
go build -o ../bin/antenna-studio ./cmd/server
```

### Run the test suite

```bash
make test
```

This runs all 86+ unit tests across the backend packages. All tests must pass
before using the binary in production.

Expected output (last line):
```
ok      antenna-studio/internal/...   x.xxxs
```

No `FAIL` lines should appear.

---

## 5. Part 4 — Run and Verify

### Start the server

```bash
./bin/antenna-studio
```

The binary bundles the frontend once at startup (takes ~1 second), then listens
on port 8080.

Expected output:
```
Antenna Studio listening on :8080
```

### Open in a browser

Open your **Windows browser** and go to:

```
http://localhost:8080
```

WSL2 automatically forwards ports to Windows, so no extra configuration is needed.

### Verify the simulation engine

1. Click **Templates** in the toolbar and select **Half-Wave Dipole**.
2. Set the frequency to **300 MHz**.
3. Click **Simulate**.
4. In the **Impedance** results tab, confirm the feedpoint impedance is
   approximately **Z ≈ 73.1 + j42.5 Ω**.

This is the classic textbook result for a half-wave dipole in free space.
If your value matches, the solver is working correctly.

### Stop the server

Press **Ctrl+C** in the Ubuntu terminal.

If the port is ever stuck in use, run:
```bash
./kill-all.sh
```

---

## 6. Troubleshooting

### WSL fails to install — virtualisation error

**Symptom:** `wsl --install` prints an error about virtualisation or Hyper-V.

**Fix:** Enable the required Windows features and BIOS settings:
- In BIOS/UEFI: enable **Intel VT-x** or **AMD-V** (SVM).
- In Windows Features (`optionalfeatures.exe`): enable
  **Virtual Machine Platform** and **Windows Subsystem for Linux**.
- Reboot, then retry.

### `go: command not found` after running the setup script

**Symptom:** Typing `go version` returns "command not found" in a new terminal.

**Fix:** The PATH change was written to `~/.bashrc` but hasn't been loaded yet.
```bash
source ~/.bashrc
go version
```

### `make deps` fails — Node.js version error

**Symptom:** `npm install` aborts with an unsupported engine warning or syntax error.

**Fix:** Check your Node.js version:
```bash
node --version
```
If it shows v16 or older, re-run the Node.js install step from Part 2 using
the NodeSource script, which installs Node.js 20 LTS.

### Port 8080 is already in use

**Symptom:** The server prints `listen tcp :8080: bind: address already in use`.

**Fix:**
```bash
./kill-all.sh
```
Or override the port:
```bash
PORT=9090 ./bin/antenna-studio
# Then open http://localhost:9090
```

### Slow first build

The first `make build` downloads Go module dependencies from the internet and
may take 1–3 minutes depending on your connection. Subsequent builds are fast
(seconds) because the module cache is warm.

### WSL2 network issues inside Ubuntu

If `curl` or `apt` cannot reach the internet from inside WSL2:
```bash
# Check DNS resolution
ping -c 1 google.com
```
If it fails, try restarting the WSL network:
```powershell
# In Windows PowerShell (Administrator)
wsl --shutdown
wsl
```
