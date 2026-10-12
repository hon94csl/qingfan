package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(`{"status":"ok","service":"qingfan-server"}`))
	})
	return mux
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "HTTP listen address")
	flag.Parse()
	server := &http.Server{Addr: *addr, Handler: handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("qingfan-server listening on %s", *addr)
	log.Fatal(server.ListenAndServe())
}
