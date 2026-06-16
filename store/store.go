package store

import (
	"sync"
	"time"
)

// Vessel holds all known data for a tracked vessel.
type Vessel struct {
	MMSI         uint32       `json:"mmsi"`
	Name         string       `json:"name"`
	CallSign     string       `json:"callSign"`
	IMO          uint32       `json:"imo,omitempty"`
	ShipType     uint8        `json:"shipType"`
	ShipTypeStr  string       `json:"shipTypeStr"`
	NavStatus    uint8        `json:"navStatus"`
	NavStatusStr string       `json:"navStatusStr"`
	Lat          float64      `json:"lat"`
	Lon          float64      `json:"lon"`
	SOG          float64      `json:"sog"`
	COG          float64      `json:"cog"`
	Heading      uint16       `json:"heading"`
	Destination  string       `json:"destination,omitempty"`
	LastSeen     time.Time    `json:"lastSeen"`
	Track        []TrackPoint `json:"track,omitempty"`
}

// TrackPoint is a historical position entry.
type TrackPoint struct {
	Lat  float64   `json:"lat"`
	Lon  float64   `json:"lon"`
	Time time.Time `json:"time"`
}

const maxTrackLen = 120

// Store is a thread-safe in-memory vessel registry.
type Store struct {
	mu      sync.RWMutex
	vessels map[uint32]*Vessel
}

func New() *Store {
	return &Store{vessels: make(map[uint32]*Vessel)}
}

// Update merges an incoming vessel update into the store.
func (s *Store) Update(v *Vessel) *Vessel {
	s.mu.Lock()
	defer s.mu.Unlock()

	ex, ok := s.vessels[v.MMSI]
	if !ok {
		// First time — store as-is (deep copy)
		cp := *v
		cp.Track = nil
		s.vessels[v.MMSI] = &cp
		return &cp
	}

	// Merge position data
	if v.Lat != 0 || v.Lon != 0 {
		if ex.Lat != 0 {
			ex.Track = append(ex.Track, TrackPoint{ex.Lat, ex.Lon, ex.LastSeen})
			if len(ex.Track) > maxTrackLen {
				ex.Track = ex.Track[len(ex.Track)-maxTrackLen:]
			}
		}
		ex.Lat = v.Lat
		ex.Lon = v.Lon
		ex.SOG = v.SOG
		ex.COG = v.COG
		ex.Heading = v.Heading
		ex.NavStatus = v.NavStatus
		ex.NavStatusStr = v.NavStatusStr
		ex.LastSeen = v.LastSeen
	}

	// Merge static data (only overwrite if non-empty)
	if v.Name != "" {
		ex.Name = v.Name
	}
	if v.CallSign != "" {
		ex.CallSign = v.CallSign
	}
	if v.ShipType != 0 {
		ex.ShipType = v.ShipType
		ex.ShipTypeStr = v.ShipTypeStr
	}
	if v.Destination != "" {
		ex.Destination = v.Destination
	}
	if v.IMO != 0 {
		ex.IMO = v.IMO
	}

	cp := *ex
	return &cp
}

// GetAll returns a snapshot of all vessels (no track data for performance).
func (s *Store) GetAll() []Vessel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Vessel, 0, len(s.vessels))
	for _, v := range s.vessels {
		cp := *v
		cp.Track = nil
		out = append(out, cp)
	}
	return out
}

// Get returns a full vessel copy including track.
func (s *Store) Get(mmsi uint32) (Vessel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vessels[mmsi]
	if !ok {
		return Vessel{}, false
	}
	cp := *v
	return cp, true
}

// Count returns the number of tracked vessels.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.vessels)
}

// Prune removes vessels not seen within maxAge.
func (s *Store) Prune(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	n := 0
	for mmsi, v := range s.vessels {
		if v.LastSeen.Before(cutoff) {
			delete(s.vessels, mmsi)
			n++
		}
	}
	return n
}
