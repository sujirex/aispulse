package ais

import (
	"bufio"
	"log"
	"net"
	"time"

	"aispulse/store"
)

// Feeder connects to a live AIS TCP/UDP feed and decodes incoming sentences.
type Feeder struct {
	addr      string
	st        *store.Store
	broadcast BroadcastFn
}

// NewFeeder creates a TCP feeder pointing at addr (e.g. "153.44.253.27:5631").
func NewFeeder(addr string, st *store.Store, fn BroadcastFn) *Feeder {
	return &Feeder{addr: addr, st: st, broadcast: fn}
}

// Run connects and reads indefinitely, reconnecting on error.
func (f *Feeder) Run() {
	for {
		log.Printf("[feeder] connecting to %s", f.addr)
		if err := f.connect(); err != nil {
			log.Printf("[feeder] %v — retrying in 15s", err)
		}
		time.Sleep(15 * time.Second)
	}
}

func (f *Feeder) connect() error {
	conn, err := net.DialTimeout("tcp", f.addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("[feeder] connected to %s", f.addr)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		parsed, err := Decode(line)
		if err != nil || parsed == nil {
			continue
		}
		updated := f.st.Update(parsed.Vessel)
		if updated != nil && f.broadcast != nil {
			f.broadcast(updated, line)
		}
	}
	return scanner.Err()
}
