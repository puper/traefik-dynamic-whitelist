package app

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr       string
	AdminToken       string
	StatePath        string
	TraefikPath      string
	TempDuration     time.Duration
	ClientIPHeaders  []string
	TrustedProxyCIDR []netip.Prefix
	TraefikIPDepth   int
}

func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:      envOrDefault("LISTEN_ADDR", ":8080"),
		AdminToken:      os.Getenv("ADMIN_TOKEN"),
		StatePath:       envOrDefault("STATE_PATH", "data/state.json"),
		TraefikPath:     envOrDefault("TRAEFIK_DYNAMIC_PATH", "data/whitelist.yml"),
		ClientIPHeaders: splitCSV(envOrDefault("CLIENT_IP_HEADERS", "X-Forwarded-For,X-Real-IP")),
	}

	if cfg.AdminToken == "" {
		return Config{}, errors.New("ADMIN_TOKEN is required")
	}

	hours, err := strconv.Atoi(envOrDefault("TEMP_HOURS", "24"))
	if err != nil || hours <= 0 {
		return Config{}, fmt.Errorf("TEMP_HOURS must be a positive integer")
	}
	cfg.TempDuration = time.Duration(hours) * time.Hour

	depth, err := strconv.Atoi(envOrDefault("TRAEFIK_IP_STRATEGY_DEPTH", "0"))
	if err != nil || depth < 0 {
		return Config{}, fmt.Errorf("TRAEFIK_IP_STRATEGY_DEPTH must be a non-negative integer")
	}
	cfg.TraefikIPDepth = depth

	for _, raw := range splitCSV(os.Getenv("TRUSTED_PROXIES")) {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			addr, addrErr := netip.ParseAddr(raw)
			if addrErr != nil {
				return Config{}, fmt.Errorf("parse TRUSTED_PROXIES %q: %w", raw, err)
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		cfg.TrustedProxyCIDR = append(cfg.TrustedProxyCIDR, prefix)
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
