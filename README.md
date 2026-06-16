# AISPulse

Real-time AIS vessel tracking dashboard for Indian coastal waters — built with Go and Leaflet.js.

![Go](https://img.shields.io/badge/Go-1.21-00ADD8?style=flat&logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-0DC8C8?style=flat)
![Live](https://img.shields.io/badge/Demo-Render-46E3B7?style=flat&logo=render&logoColor=white)

---

## Features

- **Live vessel map** — Leaflet.js with CartoDB dark/light tiles; vessel markers rotate to heading and are coloured by ship type
- **WebSocket feed** — real-time position updates pushed to all connected clients
- **Vessel type filter** — filter map and sidebar by Cargo, Tanker, Tug, Fishing, Passenger, Pilot, SAR
- **Port boundary alerts** — monitors 12 major Indian ports; toast notifications fire when a vessel enters or leaves a zone
- **Speed history chart** — inline SVG sparkline of the last 40 SOG readings per vessel
- **Export track** — download any vessel's breadcrumb trail as CSV or KML (Google Earth)
- **Fleet stats** — live counts of total, underway, anchored and moored vessels
- **NMEA log** — scrolling feed of raw AIS sentences as they arrive
- **Dark / light mode** — persisted in localStorage
- **Built-in simulator** — 20 named Indian coastal vessels wander the coast when no live feed is connected

---

## Screenshots

> Dark mode · vessel detail panel · port zone circles

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.21, `gorilla/websocket` |
| AIS decoding | Custom 6-bit ASCII decoder (types 1/2/3, 5, 18, 24) |
| Frontend | Vanilla JS, Leaflet.js, CSS custom properties |
| Fonts | Roboto + Roboto Mono (Google Fonts) |
| Deployment | Docker, Render (free tier) |

The entire web frontend is embedded in the Go binary via `//go:embed` — no separate file server needed.

---

## Quick Start

### Prerequisites

- Go 1.21+

### Run locally

```bash
git clone https://github.com/sujirex/aispulse.git
cd aispulse
go mod tidy
go run .
```

Open [http://localhost:8080](http://localhost:8080)

The built-in simulator starts automatically with 20 vessels.

### Command-line flags

| Flag | Default | Description |
|---|---|---|
| `-addr` | `:8080` | HTTP listen address |
| `-sim` | `true` | Enable vessel simulator |
| `-feed` | _(empty)_ | Live AIS TCP feed (e.g. `data.aishub.net:5631`) |

### Connect a live AIS feed

```bash
go run . -sim=false -feed data.aishub.net:5631
```

Register at [aishub.net](https://www.aishub.net) (free, non-commercial) to get access to the global AIS network.

### Build a single binary

```bash
go build -o aispulse .
./aispulse -addr :8080
```

---

## Live Data Sources

AISPulse connects to any NMEA-over-TCP source:

- **[AISHub](https://www.aishub.net)** — free community network, register for TCP access
- **RTL-SDR + AIS-catcher** — own VHF receiver for local coverage (~20–40 NM)
- **MarineTraffic / VesselFinder** — commercial APIs (requires adapter)
- **Indian VTMS feeds** — port authority feeds via DGLL (authorised access)

---

## Monitored Port Zones

Kandla · Bhavnagar · Dahej · Hazira · Mumbai/JNPT · Mormugao · Kochi · Chennai · Visakhapatnam · Paradip · Haldia · Port Blair

---

## Deployment

### Docker

```bash
docker build -t aispulse .
docker run -p 8080:8080 aispulse
```

### Render (free)

1. Fork or push this repo to GitHub
2. New Web Service → connect repo → Runtime: **Docker** → Instance: **Free**
3. Deploy — live in ~5 minutes at `https://your-app.onrender.com`

> **Note:** Render's free tier sleeps after 15 minutes of inactivity. Suitable for demos. For always-on hosting use Oracle Cloud Always Free + Cloudflare Tunnel.

---

## Project Structure

```
aispulse/
├── main.go              # Entry point, flags, goroutines
├── go.mod
├── Dockerfile
├── ais/
│   ├── decoder.go       # NMEA 6-bit AIS decoder (types 1/2/3/5/18/24)
│   ├── simulator.go     # Built-in vessel simulator (Indian coastal waters)
│   └── feeder.go        # TCP AIS feed connector
├── store/
│   └── store.go         # Thread-safe in-memory vessel registry + track history
└── server/
    ├── server.go        # HTTP server, routes, embedded static files
    ├── hub.go           # WebSocket hub (broadcast to all clients)
    ├── rest.go          # REST API (/api/vessels, /api/stats)
    └── web/
        ├── index.html   # Full dashboard SPA
        └── guide.html   # User guide
```

---

## API

| Endpoint | Method | Description |
|---|---|---|
| `/api/vessels` | GET | All vessels (no track) |
| `/api/vessels/{mmsi}` | GET | Single vessel with full track |
| `/api/stats` | GET | Fleet statistics |
| `/ws` | WS | Real-time vessel update stream |

### WebSocket message format

```json
{
  "type": "vessel_update",
  "vessel": {
    "mmsi": 419000001,
    "name": "MUMBAI STAR",
    "lat": 18.94,
    "lon": 72.88,
    "sog": 12.4,
    "cog": 245.0,
    "shipTypeStr": "Cargo",
    "navStatusStr": "Under way using engine"
  },
  "raw": "!AIVDM,1,1,,B,15M2TR..."
}
```

---

## Built by

**Suji Kumar C** · [sujikumar.com](https://sujikumar.com)

---

## License

MIT
