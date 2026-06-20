<div align="center">

# WebSDR

**Software-defined radio in your browser — spectrum, demod, satellite LRPT**

[![Build](https://github.com/iamoumeng/websdr/actions/workflows/build.yml/badge.svg)](https://github.com/iamoumeng/websdr/actions/workflows/build.yml)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js&logoColor=white)](https://vuejs.org/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../LICENSE)

[简体中文](../README.md)

<table align="center" border="0" cellspacing="16" cellpadding="0">
  <tr>
    <td align="center"><img src="images/radio.png" width="420" alt=""></td>
    <td align="center"><img src="images/satellite.png" width="420" alt=""></td>
  </tr>
  <tr>
    <td align="center"><img src="images/ads-b.png" width="420" alt=""></td>
    <td align="center"><img src="images/protocol.png" width="420" alt=""></td>
  </tr>
</table>

</div>

---

## Overview

WebSDR is a **Go + RTL-SDR + Vue 3** web-based software-defined radio. Plug in a USB dongle, open your browser, and enjoy live spectrum & waterfall, audio demodulation, **real-time Meteor-M LRPT weather image decoding**, ~~NOAA APT~~, ADS-B, AIS, and multi-protocol auto-detection.

## Supported Hardware

WebSDR uses [librtlsdr](https://github.com/osmocom/rtl-sdr) to drive **RTL2832U + R820T/R820T2** USB receivers. **The author's personal device for development and testing is the [RTL-SDR Blog V3](https://www.rtl-sdr.com/)**; screenshots and feature validation in this project are based on that model.

Other compatible dongles include:

| Device | Notes |
|--------|-------|
| [RTL-SDR Blog V3 / V4](https://www.rtl-sdr.com/) | Recommended; **V3 is the author's daily driver**, bias-tee & improved HF |
| Nooelec NESDR Smart / SMArt | Compact, portable |
| Generic RTL2832U TV sticks | Must use RTL2832U + R820T2 chipset |

**Tips**

- **FM / aviation / ham bands** — stock antenna is fine for testing
- **137 MHz weather satellites (LRPT ~~/ APT~~)** — use a **137 MHz QFH or V-dipole**; an LNA helps
- **1090 MHz ADS-B** — vertical or dedicated ADS-B antenna
- **162 MHz AIS** — marine VHF antenna
- HF (&lt; 24 MHz) uses automatic Q-channel direct sampling (no manual `-direct` flag)

> One RTL-SDR tunes one band at a time. Switching pages (Radio / Satellite / ADS-B, etc.) reconfigures the hardware automatically.

## LRPT Decoding

**LRPT** (Low Resolution Picture Transmission) is the **OQPSK digital downlink** broadcast by Russian **Meteor-M** weather satellites, typically at **137.900 MHz** with a symbol rate of **72–80 k sym/s**.

WebSDR provides a full LRPT pipeline:

1. **Auto-detection** — protocol scanner finds LRPT carriers on the 137 MHz band
2. **LRPT listen page** — live spectrum, symbol-rate lock status, and signal strength
3. **Satellite decoder** — OQPSK demod for **Meteor-M2 / M2-2 / M2-3 / M2-4**, streaming **6 MSU-MR channels** (visible, NIR, SWIR, MWIR, thermal) with TLE pass prediction and Doppler tracking

| Satellite | NORAD | Modulation | Downlink |
|-----------|-------|------------|----------|
| Meteor-M2 | 40069 | LRPT (OQPSK) | 137.900 MHz |
| Meteor-M2-2 | 44387 | LRPT (OQPSK) | 137.900 MHz |
| Meteor-M2-3 | 57190 | LRPT (OQPSK) | 137.900 MHz |
| Meteor-M2-4 | 59051 | LRPT (OQPSK) | 137.900 MHz |

> LRPT is a digital image link. ~~NOAA APT used an analog FM subcarrier; the APT page remains in the app, but NOAA satellites no longer broadcast APT — this feature is effectively unavailable.~~

## Features

| Module | Capabilities |
|--------|-------------|
| **Radio** | Live spectrum / waterfall; WFM / NFM / AM / USB / LSB / DSB / CW / RAW; dial tuning; low-latency WebSocket audio |
| **Satellite** | Meteor-M LRPT live 6-channel imagery; TLE passes; Doppler tracking |
| ~~**APT**~~ | ~~NOAA APT analog downlink, live cloud image rebuild~~ (NOAA APT broadcasts discontinued) |
| **LRPT** | Meteor LRPT digital link monitoring & symbol-rate lock |
| **ADS-B** | 1090 MHz Mode S decode & map tracking |
| **AIS** | 162 MHz vessel AIS decode |
| **Protocol scan** | Auto-detect FM RDS, POCSAG, LoRa, DMR, and more |

## Quick Start

**Requirements:** Go 1.22+ · Node.js 20+ · [librtlsdr](https://github.com/osmocom/rtl-sdr) · RTL-SDR USB dongle · `CGO_ENABLED=1` (on by default)

### Build from Source

**1. Build frontend (all platforms)**

```bash
cd web && npm ci && npm run build && cd ..
```

**2. Install librtlsdr & compile Go**

<details>
<summary><b>Linux</b> (Debian / Ubuntu)</summary>

```bash
sudo apt update
sudo apt install -y librtlsdr-dev libusb-1.0-0-dev pkg-config golang-go

cd /path/to/websdr
CGO_ENABLED=1 go build -o websdr ./cmd/websdr
./websdr
```

Fedora / RHEL:

```bash
sudo dnf install -y rtl-sdr-devel libusb1-devel pkgconfig golang
CGO_ENABLED=1 go build -o websdr ./cmd/websdr
```

</details>

<details>
<summary><b>macOS</b></summary>

```bash
brew install librtlsdr go node

cd /path/to/websdr
CGO_ENABLED=1 go build -o websdr ./cmd/websdr
./websdr
```

</details>

<details>
<summary><b>Windows</b> (MSYS2 MinGW64)</summary>

1. Install [MSYS2](https://www.msys2.org/) and [Go for Windows](https://go.dev/dl/)
2. Open an **MSYS2 MINGW64** terminal and install dependencies:

```bash
pacman -S --needed mingw-w64-x86_64-gcc mingw-w64-x86_64-pkg-config mingw-w64-x86_64-rtl-sdr
```

3. Build in the same terminal (ensure `go` is on PATH):

```bash
cd /c/path/to/websdr
CGO_ENABLED=1 go build -o websdr.exe ./cmd/websdr
./websdr.exe
```

> Build on Windows inside MSYS2 MINGW64 so `pkg-config` can find `librtlsdr`. Install the [Zadig WinUSB driver](https://zadig.akeo.ie/) for your RTL-SDR before running.

</details>

### Runtime dependencies

Build requires `-dev` packages; to **run** `websdr` or a Release binary you also need shared libraries (`.so` / `.dylib` / `.dll`). Otherwise you may see errors like `error while loading shared libraries: librtlsdr.so.0`.

<details>
<summary><b>Linux</b> runtime libraries</summary>

Debian / Ubuntu:

```bash
sudo apt update
sudo apt install -y librtlsdr0 libusb-1.0-0
```

Fedora / RHEL:

```bash
sudo dnf install -y rtl-sdr libusb1
```

Verify:

```bash
ldd ./websdr | grep -E 'rtlsdr|usb'
```

</details>

<details>
<summary><b>macOS</b></summary>

```bash
brew install librtlsdr
```

</details>

<details>
<summary><b>Windows</b></summary>

The GitHub **Release** `websdr-windows-amd64.zip` bundles `websdr.exe` with required DLLs — unzip and run (still needs the [Zadig WinUSB driver](https://zadig.akeo.ie/) for your RTL-SDR).

When building locally, add MSYS2 `mingw64\bin` to PATH or copy DLLs next to the exe.

</details>

Open **http://127.0.0.1:8080** locally or **http://&lt;your-ip&gt;:8080** on the LAN (default bind: `0.0.0.0`). Plug in the RTL-SDR and hit Play.

**CLI flags**

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | (empty, all interfaces) | Listen address; empty = dual-stack `:port` |
| `-port` | 8080 | HTTP port |
| `-freq` | 100000000 | Initial center frequency (Hz) |
| `-gain` | 30 | Gain (dB) |
| `-agc` | false | Auto gain control |
| `-device` | 0 | RTL-SDR device index |

**Frontend dev**

```bash
./websdr               # terminal 1: backend on :8080
cd web && npm run dev  # terminal 2: Vite on :5173 (WebSocket proxied)
```

## Stack

Go · librtlsdr · Vue 3 · Vite · WebSocket · AudioWorklet · Leaflet

---

<div align="center">

Apache License 2.0 · [iamoumeng/websdr](https://github.com/iamoumeng/websdr)

</div>
