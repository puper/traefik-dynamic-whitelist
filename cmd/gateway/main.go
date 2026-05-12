package main

import (
	"log"
	"net/http"
	"os"

	"github.com/puper/traefik-dynamic-whitelist/internal/app"
)

func main() {
	cfg, err := app.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	server, err := app.NewServer(cfg)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	log.Printf("gateway console listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, server.Routes()); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
