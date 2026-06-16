package ais

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"aispulse/store"
)

// simVessel is the mutable state for a simulated vessel.
type simVessel struct {
	mmsi      uint32
	name      string
	callSign  string
	shipType  uint8
	imo       uint32
	dest      string
	navStatus uint8
	lat       float64
	lon       float64
	sog       float64 // knots
	cog       float64 // degrees
	heading   float64
}

// BroadcastFn is called whenever a vessel update is ready.
type BroadcastFn func(vessel *store.Vessel, raw string)

// Simulator generates realistic AIS traffic around Indian coastal waters.
type Simulator struct {
	vessels   []*simVessel
	st        *store.Store
	broadcast BroadcastFn
}

// seed vessels — Indian coastal fleet
var seedFleet = []simVessel{
	{419001001, "MV BHARUCH", "VTBR1", 70, 9001001, "KANDLA", 0, 21.71, 72.53, 8.5, 120, 120},
	{419001002, "MT RELIANCE", "VTRL2", 80, 9001002, "MUMBAI", 0, 22.50, 69.80, 12.0, 50, 50},
	{419001003, "MV COASTAL STAR", "VTCS3", 70, 9001003, "MORMUGAO", 0, 18.93, 72.83, 7.2, 185, 185},
	{419001004, "TUG SHAKTIMAN", "VTSK4", 52, 9001004, "HAZIRA", 0, 21.00, 72.60, 4.5, 270, 270},
	{419001005, "MV OCEAN PRIDE", "VTOP5", 70, 9001005, "KOCHI", 0, 15.41, 73.79, 10.0, 355, 355},
	{419001006, "MT NARMADA", "VTNA6", 80, 9001006, "CHENNAI", 0, 9.96, 76.27, 6.0, 90, 90},
	{419001007, "MV KONKAN", "VTKK7", 70, 9001007, "PARADIP", 0, 13.08, 80.27, 9.5, 22, 22},
	{419001008, "MV SAGAR", "VTSG8", 70, 9001008, "KOLKATA", 0, 17.68, 83.21, 8.0, 130, 130},
	{419001009, "PILOT VENUS", "VTPV9", 50, 9001009, "PARADIP", 0, 20.31, 86.61, 12.0, 200, 200},
	{419001010, "MV HOOGHLY", "VTHG0", 70, 9001010, "HALDIA", 0, 21.57, 88.36, 5.5, 340, 340},
	{419001011, "MT KUTCH", "VTKU1", 80, 9001011, "KANDLA", 0, 23.00, 70.21, 11.0, 80, 80},
	{419001012, "MV ANDAMAN", "VTAM2", 70, 9001012, "CHENNAI", 0, 11.62, 92.72, 9.0, 275, 275},
	{419001013, "FV DEEP SEA", "VTDP3", 30, 9001013, "MORMUGAO", 7, 14.00, 74.50, 3.0, 180, 180},
	{419001014, "TUG HERCULES", "VTHC4", 52, 9001014, "BHAVNAGAR", 1, 22.50, 72.00, 0.0, 0, 0},
	{419001015, "MV EASTERN STAR", "VTES5", 70, 9001015, "VIZAG", 0, 12.00, 80.50, 14.0, 45, 45},
	{419001016, "MT GUJARAT", "VTGJ6", 80, 9001016, "HAZIRA", 0, 20.50, 71.50, 10.5, 140, 140},
	{419001017, "MV KOCHI PRIDE", "VTKP7", 70, 9001017, "MUMBAI", 0, 10.50, 76.00, 9.0, 340, 340},
	{419001018, "SAR VESSEL 01", "VTSR8", 51, 9001018, "MUMBAI", 0, 19.20, 72.80, 15.0, 60, 60},
	{419001019, "MV DECCAN", "VTDC9", 70, 9001019, "CHENNAI", 0, 16.00, 81.50, 11.0, 95, 95},
	{419001020, "PORT TENDER 1", "VTPT0", 53, 9001020, "KOCHI", 5, 9.95, 76.25, 1.0, 0, 0},
}

// NewSimulator creates and initialises a vessel simulator.
func NewSimulator(st *store.Store, fn BroadcastFn) *Simulator {
	vessels := make([]*simVessel, len(seedFleet))
	for i, v := range seedFleet {
		cp := v
		// Small random scatter so vessels don't all start on the same spot
		cp.lat += (rand.Float64() - 0.5) * 0.8
		cp.lon += (rand.Float64() - 0.5) * 0.8
		cp.cog += (rand.Float64() - 0.5) * 30
		if cp.cog < 0 {
			cp.cog += 360
		}
		cp.heading = cp.cog
		vessels[i] = &cp
	}
	return &Simulator{vessels: vessels, st: st, broadcast: fn}
}

// Run starts the simulation loop (blocking — call in a goroutine).
func (s *Simulator) Run() {
	// Pre-populate static data (type 5 equivalent)
	for _, v := range s.vessels {
		static := &store.Vessel{
			MMSI:        v.mmsi,
			Name:        v.name,
			CallSign:    v.callSign,
			ShipType:    v.shipType,
			ShipTypeStr: shipTypeStr(v.shipType),
			IMO:         v.imo,
			Destination: v.dest,
			LastSeen:    time.Now(),
		}
		s.st.Update(static)
	}

	positionTick := time.NewTicker(4 * time.Second)
	staticTick := time.NewTicker(60 * time.Second)

	for {
		select {
		case <-positionTick.C:
			// Update 4-7 random vessels per tick
			perm := rand.Perm(len(s.vessels))
			count := 4 + rand.Intn(4)
			for i := 0; i < count && i < len(perm); i++ {
				v := s.vessels[perm[i]]
				if v.navStatus == 1 || v.navStatus == 5 {
					continue // anchored / moored — don't move
				}
				s.move(v)
				sentence := s.encodeType1(v)
				parsed, err := Decode(sentence)
				if err != nil || parsed == nil {
					continue
				}
				updated := s.st.Update(parsed.Vessel)
				if updated != nil && s.broadcast != nil {
					s.broadcast(updated, sentence)
				}
			}

		case <-staticTick.C:
			// Re-broadcast static info for all vessels
			for _, v := range s.vessels {
				static := &store.Vessel{
					MMSI:        v.mmsi,
					Name:        v.name,
					CallSign:    v.callSign,
					ShipType:    v.shipType,
					ShipTypeStr: shipTypeStr(v.shipType),
					IMO:         v.imo,
					Destination: v.dest,
					LastSeen:    time.Now(),
				}
				s.st.Update(static)
			}
		}
	}
}

// move applies one timestep of physics to a vessel.
func (s *Simulator) move(v *simVessel) {
	// Wander heading
	v.cog += (rand.Float64() - 0.5) * 8
	v.cog = normAngle(v.cog)
	v.heading = v.cog

	// Vary speed
	v.sog += (rand.Float64() - 0.5) * 0.3
	if v.sog < 1 {
		v.sog = 1
	}
	if v.sog > 18 {
		v.sog = 18
	}

	// Advance position (dt=4s, sog in knots → nm/s → degrees)
	dt := 4.0 / 3600.0
	dist := v.sog * dt
	rad := v.cog * math.Pi / 180.0
	v.lat += dist * math.Cos(rad) / 60.0
	v.lon += dist * math.Sin(rad) / (60.0 * math.Cos(v.lat*math.Pi/180.0))

	// Bounce off bounding box (rough Indian Ocean region)
	if v.lat < 6 || v.lat > 24 {
		v.cog = normAngle(360 - v.cog)
		v.heading = v.cog
	}
	if v.lon < 65 || v.lon > 94 {
		v.cog = normAngle(180 - v.cog)
		v.heading = v.cog
	}
}

func normAngle(a float64) float64 {
	for a < 0 {
		a += 360
	}
	for a >= 360 {
		a -= 360
	}
	return a
}

// ── AIS encoding (type 1) ──────────────────────────────────────────────────

func (s *Simulator) encodeType1(v *simVessel) string {
	buf := make([]byte, 28) // 168 bits ÷ 6 = 28 chars

	writeBits(buf, 0, 6, 1)                        // msg type
	writeBits(buf, 6, 2, 0)                        // repeat
	writeBits(buf, 8, 30, uint64(v.mmsi))          // MMSI
	writeBits(buf, 38, 4, uint64(v.navStatus))     // nav status
	writeBits(buf, 42, 8, 128)                     // ROT = unavailable
	writeBits(buf, 50, 10, uint64(v.sog*10))       // SOG ×10
	writeBits(buf, 60, 1, 1)                       // position accuracy
	writeSignedBits(buf, 61, 28, int64(v.lon*600000)) // longitude
	writeSignedBits(buf, 89, 27, int64(v.lat*600000)) // latitude
	writeBits(buf, 116, 12, uint64(v.cog*10))      // COG ×10
	writeBits(buf, 128, 9, uint64(v.heading))      // true heading
	writeBits(buf, 137, 6, 60)                     // timestamp (60=unavail)

	payload := bitsToPayload(buf)
	cs := nmeaChecksum("AIVDM,1,1,,A," + payload + ",0")
	return fmt.Sprintf("!AIVDM,1,1,,A,%s,0*%02X", payload, cs)
}

// ── bit-packing helpers ────────────────────────────────────────────────────

func writeBits(buf []byte, offset, n int, val uint64) {
	for i := 0; i < n; i++ {
		charIdx := (offset + i) / 6
		bitShift := uint(5 - (offset+i)%6)
		bit := uint8((val >> uint(n-1-i)) & 1)
		if charIdx < len(buf) {
			buf[charIdx] = (buf[charIdx] &^ (1 << bitShift)) | (bit << bitShift)
		}
	}
}

func writeSignedBits(buf []byte, offset, n int, val int64) {
	if val < 0 {
		val += 1 << uint(n)
	}
	writeBits(buf, offset, n, uint64(val))
}

func bitsToPayload(bits []byte) string {
	out := make([]byte, len(bits))
	for i, b := range bits {
		c := b + 48
		if c > 87 {
			c += 8
		}
		out[i] = c
	}
	return string(out)
}

func nmeaChecksum(s string) byte {
	var cs byte
	for i := 0; i < len(s); i++ {
		cs ^= s[i]
	}
	return cs
}
