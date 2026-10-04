package main

import (
	"log"
	"net/http"
)

func main() {
	hub := NewHub()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	addr := ":8080"
	log.Printf("Farkle server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
