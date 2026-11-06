// Package server provides the HTTP server, WebSocket hub and REST API.
package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"

	"aispulse/store"
)

//go:embed web
var webFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Server is the main HTTP server.
type Server struct {
	addr  string
	store *store.Store
	hub   *Hub
	mux   *http.ServeMux
}

// New creates a Server and registers all routes.
func New(addr string, st *store.Store, hub *Hub) *Server {
	s := &Server{addr: addr, store: st, hub: hub, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/vessels/", s.routeVessel)
	s.mux.HandleFunc("/api/vessels", s.handleVessels)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/ws", s.handleWS)
	s.mux.HandleFunc("/config.js", handleConfigJS)

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	s.mux.Handle("/", http.FileServer(http.FS(web)))
}

// routeVessel dispatches /api/vessels/{mmsi} vs /api/vessels
func (s *Server) routeVessel(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/vessels/" {
		s.handleVessels(w, r)
		return
	}
	s.handleVessel(w, r)
}

// handleWS upgrades the connection and registers it with the hub.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 256)}
	s.hub.register(c)
	go c.writePump()

	// Read loop — keeps connection open and detects close
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			s.hub.unregister(c)
			return
		}
	}
}

// Start begins listening (blocking).
func (s *Server) Start() error {
	log.Printf("[server] listening on http://localhost%s", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

// handleConfigJS exposes public client settings. CARTO_KEY is a browser-side
// basemap key (restrict it to your domains in the CARTO dashboard).
func handleConfigJS(w http.ResponseWriter, r *http.Request) {
	cfg, _ := json.Marshal(map[string]string{"cartoKey": os.Getenv("CARTO_KEY")})
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("window.AIS_CONFIG=" + string(cfg) + ";\n"))
}
