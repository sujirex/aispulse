// Package ais provides NMEA AIS sentence parsing and decoding.
package ais

import (
	"fmt"
	"strings"
	"time"

	"aispulse/store"
)

// Parsed holds the decoded result of one AIS sentence.
type Parsed struct {
	MsgType uint8
	Vessel  *store.Vessel
}

// Decode parses a single NMEA !AIVDM / !AIVDO sentence.
// Multi-part messages (e.g. type 5) must be assembled before calling.
func Decode(sentence string) (*Parsed, error) {
	sentence = strings.TrimSpace(sentence)
	if !strings.HasPrefix(sentence, "!AIVDM") && !strings.HasPrefix(sentence, "!AIVDO") {
		return nil, fmt.Errorf("not AIS")
	}
	parts := strings.Split(sentence, ",")
	if len(parts) < 6 || parts[5] == "" {
		return nil, fmt.Errorf("malformed sentence")
	}

	payload := parts[5]
	bits := decodeBits(payload)
	if len(bits) == 0 {
		return nil, fmt.Errorf("empty payload")
	}

	msgType := uint8(extractBits(bits, 0, 6))
	switch msgType {
	case 1, 2, 3:
		return decodeType123(bits, msgType)
	case 5:
		return decodeType5(bits)
	case 18:
		return decodeType18(bits)
	case 24:
		return decodeType24(bits)
	default:
		return nil, fmt.Errorf("unsupported msg type %d", msgType)
	}
}

// ── bit helpers ────────────────────────────────────────────────────────────

// decodeBits converts an AIS payload string into a slice of 6-bit values.
func decodeBits(payload string) []byte {
	out := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		v := payload[i] - 48
		if v > 39 {
			v -= 8
		}
		out[i] = v & 0x3F
	}
	return out
}

// extractBits reads n unsigned bits starting at bit offset from 6-bit slice.
func extractBits(b []byte, offset, n int) uint64 {
	var r uint64
	for i := 0; i < n; i++ {
		bi := (offset + i) / 6
		shift := uint(5 - (offset+i)%6)
		if bi < len(b) {
			r = (r << 1) | uint64((b[bi]>>shift)&1)
		}
	}
	return r
}

// extractSigned reads n bits as a two's-complement signed integer.
func extractSigned(b []byte, offset, n int) int64 {
	v := int64(extractBits(b, offset, n))
	if v >= (1 << uint(n-1)) {
		v -= 1 << uint(n)
	}
	return v
}

// extractStr reads a 6-bit ASCII string of n bits.
func extractStr(b []byte, offset, n int) string {
	chars := n / 6
	r := make([]byte, chars)
	for i := 0; i < chars; i++ {
		c := byte(extractBits(b, offset+i*6, 6))
		if c < 32 {
			c += 64
		}
		r[i] = c
	}
	return strings.TrimRight(string(r), "@ \x00")
}

// ── message decoders ───────────────────────────────────────────────────────

func decodeType123(b []byte, msgType uint8) (*Parsed, error) {
	if len(b)*6 < 168 {
		return nil, fmt.Errorf("type 1/2/3 payload too short")
	}
	mmsi := uint32(extractBits(b, 8, 30))
	nav := uint8(extractBits(b, 38, 4))
	sog := float64(extractBits(b, 50, 10)) / 10.0
	lon := float64(extractSigned(b, 61, 28)) / 600000.0
	lat := float64(extractSigned(b, 89, 27)) / 600000.0
	cog := float64(extractBits(b, 116, 12)) / 10.0
	hdg := uint16(extractBits(b, 128, 9))

	// Sanity-clamp
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		lat, lon = 0, 0
	}
	if sog > 102 {
		sog = 0
	}
	if cog >= 360 {
		cog = 0
	}
	if hdg > 359 {
		hdg = 0
	}

	return &Parsed{
		MsgType: msgType,
		Vessel: &store.Vessel{
			MMSI:         mmsi,
			NavStatus:    nav,
			NavStatusStr: navStatusStr(nav),
			SOG:          sog,
			COG:          cog,
			Heading:      hdg,
			Lat:          lat,
			Lon:          lon,
			LastSeen:     time.Now(),
		},
	}, nil
}

func decodeType5(b []byte) (*Parsed, error) {
	if len(b)*6 < 426 {
		return nil, fmt.Errorf("type 5 payload too short")
	}
	mmsi := uint32(extractBits(b, 8, 30))
	imo := uint32(extractBits(b, 40, 30))
	cs := extractStr(b, 70, 42)
	name := extractStr(b, 112, 120)
	stype := uint8(extractBits(b, 232, 8))
	dest := extractStr(b, 302, 120)

	return &Parsed{
		MsgType: 5,
		Vessel: &store.Vessel{
			MMSI:        mmsi,
			IMO:         imo,
			CallSign:    cs,
			Name:        name,
			ShipType:    stype,
			ShipTypeStr: shipTypeStr(stype),
			Destination: dest,
			LastSeen:    time.Now(),
		},
	}, nil
}

func decodeType18(b []byte) (*Parsed, error) {
	if len(b)*6 < 168 {
		return nil, fmt.Errorf("type 18 payload too short")
	}
	mmsi := uint32(extractBits(b, 8, 30))
	sog := float64(extractBits(b, 46, 10)) / 10.0
	lon := float64(extractSigned(b, 57, 28)) / 600000.0
	lat := float64(extractSigned(b, 85, 27)) / 600000.0
	cog := float64(extractBits(b, 112, 12)) / 10.0
	hdg := uint16(extractBits(b, 124, 9))

	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		lat, lon = 0, 0
	}
	if hdg > 359 {
		hdg = 0
	}

	return &Parsed{
		MsgType: 18,
		Vessel: &store.Vessel{
			MMSI:     mmsi,
			SOG:      sog,
			COG:      cog,
			Heading:  hdg,
			Lat:      lat,
			Lon:      lon,
			LastSeen: time.Now(),
		},
	}, nil
}

func decodeType24(b []byte) (*Parsed, error) {
	if len(b)*6 < 40 {
		return nil, fmt.Errorf("type 24 too short")
	}
	mmsi := uint32(extractBits(b, 8, 30))
	part := uint8(extractBits(b, 38, 2))
	v := &store.Vessel{MMSI: mmsi, LastSeen: time.Now()}

	if part == 0 && len(b)*6 >= 160 {
		v.Name = extractStr(b, 40, 120)
	} else if part == 1 && len(b)*6 >= 168 {
		v.ShipType = uint8(extractBits(b, 40, 8))
		v.ShipTypeStr = shipTypeStr(v.ShipType)
		v.CallSign = extractStr(b, 90, 42)
	}
	return &Parsed{MsgType: 24, Vessel: v}, nil
}

// ── lookup tables ──────────────────────────────────────────────────────────

func navStatusStr(s uint8) string {
	m := map[uint8]string{
		0: "Underway", 1: "At Anchor", 2: "Not Under Command",
		3: "Restricted Maneuv.", 4: "Constrained Draft",
		5: "Moored", 6: "Aground", 7: "Fishing", 8: "Sailing",
	}
	if s, ok := m[s]; ok {
		return s
	}
	return "Unknown"
}

func shipTypeStr(t uint8) string {
	switch {
	case t == 0:
		return "Unknown"
	case t == 30:
		return "Fishing"
	case t == 31 || t == 32:
		return "Towing"
	case t == 33:
		return "Dredger"
	case t == 35:
		return "Military"
	case t == 36:
		return "Sailing"
	case t == 37:
		return "Pleasure"
	case t == 50:
		return "Pilot"
	case t == 51:
		return "SAR"
	case t == 52:
		return "Tug"
	case t == 53:
		return "Port Tender"
	case t >= 60 && t < 70:
		return "Passenger"
	case t >= 70 && t < 80:
		return "Cargo"
	case t >= 80 && t < 90:
		return "Tanker"
	default:
		return "Other"
	}
}
