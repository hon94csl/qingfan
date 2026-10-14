package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/shopsync"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

func handler() http.Handler {
	return routes(nil, nil, "", nil)
}

func withGateway(g *ai.Gateway, bootstrap string, logger *log.Logger) http.Handler {
	return routes(g, nil, bootstrap, logger)
}

func routes(g *ai.Gateway, accounts *auth.Store, bootstrap string, logger *log.Logger) http.Handler {
	return serve(g, accounts, bootstrap, logger, nil)
}

func serve(g *ai.Gateway, accounts *auth.Store, bootstrap string, logger *log.Logger, shops *shopsync.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"status":"ok","service":"qingfan-server"}`))
	})
	origins := auth.DefaultOrigins()
	if accounts != nil {
		auth.Mount(mux, accounts, origins)
	}
	if shops != nil {
		shopsync.Mount(mux, shops)
	}
	if g != nil {
		mux.Handle("/v1/", ai.Routes(g, bootstrap, logger))
	}
	return auth.Protect(mux, origins)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "HTTP listen address")
	usage := flag.String("usage", "data/usage.sqlite", "usage sqlite path")
	accountsPath := flag.String("auth", "data/auth.sqlite", "account sqlite path")
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
	accounts, err := auth.Open(*accountsPath)
	if err != nil {
		log.Fatal(err)
	}
	defer accounts.Close()
	data := os.Getenv("QINGFAN_DATA")
	if data == "" {
		data = "data"
	}
	business, err := store.Open(filepath.Join(data, "qingfan.sqlite"), filepath.Join(data, "cache"))
	if err != nil {
		log.Fatal(err)
	}
	defer business.Close()
	shops := &shopsync.Service{
		DB:       business,
		Client:   tiktok.ClientFromEnv(),
		Callback: callbackURL(*addr, *cert != ""),
	}
	server := &http.Server{
		Addr:              *addr,
		Handler:           serve(gateway, accounts, os.Getenv("QINGFAN_BOOTSTRAP"), log.Default(), shops),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("qingfan-server listening on %s", *addr)
	if *cert != "" {
		log.Fatal(server.ListenAndServeTLS(*cert, *key))
	}
	log.Fatal(server.ListenAndServe())
}

func callbackURL(addr string, tls bool) string {
	if value := os.Getenv("QINGFAN_TIKTOK_CALLBACK"); value != "" {
		return value
	}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	return scheme + "://" + addr + "/v1/tiktok/callback"
}
