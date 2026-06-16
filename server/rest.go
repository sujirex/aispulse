package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleVessels(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	vessels := s.store.GetAll()
	json.NewEncoder(w).Encode(vessels)
}

func (s *Server) handleVessel(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	mmsiStr := strings.TrimPrefix(r.URL.Path, "/api/vessels/")
	mmsi64, err := strconv.ParseUint(mmsiStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid MMSI", http.StatusBadRequest)
		return
	}
	v, ok := s.store.Get(uint32(mmsi64))
	if !ok {
		http.NotFound(w, r)
		return
	}
	json.NewEncoder(w).Encode(v)
}

type statsResp struct {
	Total    int            `json:"total"`
	Underway int            `json:"underway"`
	Anchored int            `json:"anchored"`
	Moored   int            `json:"moored"`
	ByType   map[string]int `json:"byType"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	vessels := s.store.GetAll()
	resp := statsResp{
		Total:  len(vessels),
		ByType: make(map[string]int),
	}
	for _, v := range vessels {
		switch v.NavStatus {
		case 0, 8:
			resp.Underway++
		case 1:
			resp.Anchored++
		case 5:
			resp.Moored++
		}
		t := v.ShipTypeStr
		if t == "" {
			t = "Unknown"
		}
		resp.ByType[t]++
	}
	json.NewEncoder(w).Encode(resp)
}

func setCORSJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
