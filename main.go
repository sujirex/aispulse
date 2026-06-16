package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aispulse/ais"
	"aispulse/server"
	"aispulse/store"
)

func main() {
	addr     := flag.String("addr", ":8080", "HTTP listen address (e.g. :8080)")
	tcpFeed  := flag.String("feed", "", "Live AIS TCP feed address (e.g. 153.44.253.27:5631)")
	simulate := flag.Bool("sim", true, "Enable built-in vessel simulator")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("AISPulse starting…")

	st  := store.New()
	hub := server.NewHub()

	// Broadcast function shared by simulator and feeder
	broadcast := func(v *store.Vessel, raw string) {
		hub.Broadcast(v, raw)
	}

	// Simulator
	if *simulate {
		sim := ais.NewSimulator(st, broadcast)
		go sim.Run()
		log.Println("[sim] vessel simulator started (20 vessels, Indian coastal waters)")
	}

	// Live TCP feed
	if *tcpFeed != "" {
		feeder := ais.NewFeeder(*tcpFeed, st, broadcast)
		go feeder.Run()
		log.Printf("[feeder] connecting to %s", *tcpFeed)
	}

	// Periodic store pruning (vessels not seen in 30 min)
	go func() {
		t := time.NewTicker(5 * time.Minute)
		for range t.C {
			n := st.Prune(30 * time.Minute)
			if n > 0 {
				log.Printf("[store] pruned %d stale vessel(s)", n)
			}
		}
	}()

	// HTTP server
	srv := server.New(*addr, st, hub)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("[server] %v", err)
		}
	}()

	log.Printf("Open http://localhost%s in your browser", *addr)
	<-quit
	log.Println("Shutting down…")
}
