package main

import (
	"log"

	"classroom-control/client/internal/agent"
	"classroom-control/client/internal/service"
)

func main() {
	cfg, err := agent.LoadConfig(agent.DefaultConfigPath())
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	if err := service.Run(cfg); err != nil {
		log.Fatalf("service run failed: %v", err)
	}
}
