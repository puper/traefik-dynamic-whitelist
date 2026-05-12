package app

import (
	"context"
	"log"
	"time"
)

func (s *Server) RunCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed, err := s.store.CleanupExpired(s.now().UTC())
			if err != nil {
				log.Printf("cleanup expired whitelist entries: %v", err)
				continue
			}
			if changed {
				log.Printf("cleanup expired whitelist entries: updated state and traefik config")
			}
		}
	}
}
