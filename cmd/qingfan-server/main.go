package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hon94csl/qingfan/internal/ai"
)

func handler() http.Handler {
	return routes(nil, "", nil)
}

func withGateway(g *ai.Gateway, bootstrap string, logger *log.Logger) http.Handler {
	return routes(g, bootstrap, logger)
}

func routes(g *ai.Gateway, bootstrap string, logger *log.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"status":"ok","service":"qingfan-server"}`))
	})
	if g != nil {
		mux.Handle("/v1/", ai.Routes(g, bootstrap, logger))
	}
	return mux
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "HTTP listen address")
	usage := flag.String("usage", "data/usage.sqlite", "usage sqlite path")
	cert := flag.String("tls-cert", "", "TLS certificate")
	key := flag.String("tls-key", "", "TLS private key")
	public := flag.Bool("public", false, "listen for clients outside this machine")
	flag.Parse()
	if err := ai.RequirePublicTLS(*public, *cert, *key); err != nil {
		log.Fatal(err)
	}
	provider, err := ai.ProviderFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	gateway, err := ai.Open(*usage, provider, 2)
	if err != nil {
		log.Fatal(err)
	}
	defer gateway.Close()
	server := &http.Server{
		Addr:              *addr,
		Handler:           withGateway(gateway, os.Getenv("QINGFAN_BOOTSTRAP"), log.Default()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("qingfan-server listening on %s", *addr)
	if *cert != "" {
		log.Fatal(server.ListenAndServeTLS(*cert, *key))
	}
	log.Fatal(server.ListenAndServe())
}
