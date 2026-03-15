package main

import (
	"log"
	"net/http"
	"time"

	"file-transfer/internal/fileproxy"
)

func main() {
	cfg, err := fileproxy.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	server, err := fileproxy.NewServer(cfg.RootDir)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("serving %s on http://%s", cfg.RootDir, cfg.ListenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen and serve: %v", err)
	}
}
