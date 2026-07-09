// Command server runs the Minecraft 26.2 server.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zakla/mc-server/pkg/config"
	"github.com/zakla/mc-server/pkg/server"
)

func main() {
	cfgPath := flag.String("config", "config/config.toml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Printf("could not load %s (%v); using defaults", *cfgPath, err)
		cfg = config.Default()
	}

	srv := server.New(cfg)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("shutdown signal received")
		_ = srv.Stop()
	}()

	if err := srv.Start(); err != nil {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}
