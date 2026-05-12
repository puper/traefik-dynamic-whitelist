package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Store struct {
	path       string
	traefik    string
	tempWindow time.Duration
	traefikCfg TraefikConfig
	mu         sync.Mutex
}

type TraefikConfig struct {
	IPStrategyDepth int
}

type stateFile struct {
	Temporary map[string]temporaryEntry `json:"temporary"`
	Permanent map[string]permanentEntry `json:"permanent"`
}

func NewStore(statePath, traefikPath string, tempWindow time.Duration, traefikCfg ...TraefikConfig) *Store {
	cfg := TraefikConfig{}
	if len(traefikCfg) > 0 {
		cfg = traefikCfg[0]
	}

	return &Store{
		path:       statePath,
		traefik:    traefikPath,
		tempWindow: tempWindow,
		traefikCfg: cfg,
	}
}

func (s *Store) Info(now time.Time) (temporary []temporaryEntry, permanent []permanentEntry, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, changed, err := s.load(now)
	if err != nil {
		return nil, nil, err
	}
	if changed {
		if err := s.save(state); err != nil {
			return nil, nil, err
		}
		if err := s.writeTraefik(state); err != nil {
			return nil, nil, err
		}
	}

	return sortedTemporary(state), sortedPermanent(state), nil
}

func (s *Store) Add(ip string, kind string, now time.Time) error {
	if _, err := netip.ParseAddr(ip); err != nil {
		return fmt.Errorf("invalid ip %q: %w", ip, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, _, err := s.load(now)
	if err != nil {
		return err
	}

	// 最新授权操作必须成为唯一状态，避免同一个 IP 同时存在于临时和永久白名单。
	delete(state.Temporary, ip)
	delete(state.Permanent, ip)

	switch kind {
	case "temp":
		state.Temporary[ip] = temporaryEntry{
			IP:        ip,
			AddedAt:   now.UTC(),
			ExpiresAt: now.Add(s.tempWindow).UTC(),
		}
	case "perm":
		state.Permanent[ip] = permanentEntry{
			IP:      ip,
			AddedAt: now.UTC(),
		}
	default:
		return fmt.Errorf("unsupported add type %q", kind)
	}

	if err := s.save(state); err != nil {
		return err
	}
	return s.writeTraefik(state)
}

func (s *Store) Delete(ip string, now time.Time) error {
	if _, err := netip.ParseAddr(ip); err != nil {
		return fmt.Errorf("invalid ip %q: %w", ip, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, _, err := s.load(now)
	if err != nil {
		return err
	}

	delete(state.Temporary, ip)
	delete(state.Permanent, ip)

	if err := s.save(state); err != nil {
		return err
	}
	return s.writeTraefik(state)
}

func (s *Store) load(now time.Time) (stateFile, bool, error) {
	state := stateFile{
		Temporary: map[string]temporaryEntry{},
		Permanent: map[string]permanentEntry{},
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, false, nil
		}
		return stateFile{}, false, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return stateFile{}, false, err
	}
	if state.Temporary == nil {
		state.Temporary = map[string]temporaryEntry{}
	}
	if state.Permanent == nil {
		state.Permanent = map[string]permanentEntry{}
	}

	changed := false
	for ip, entry := range state.Temporary {
		if !entry.ExpiresAt.After(now) {
			delete(state.Temporary, ip)
			changed = true
		}
	}

	return state, changed, nil
}

func (s *Store) save(state stateFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0600)
}

func (s *Store) writeTraefik(state stateFile) error {
	if err := os.MkdirAll(filepath.Dir(s.traefik), 0755); err != nil {
		return err
	}

	ips := make([]string, 0, len(state.Temporary)+len(state.Permanent))
	for ip := range state.Temporary {
		ips = append(ips, ip)
	}
	for ip := range state.Permanent {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	content := "http:\n  middlewares:\n    dynamic-whitelist:\n      ipAllowList:\n        sourceRange:\n"
	if len(ips) == 0 {
		content += "          []\n"
	} else {
		for _, ip := range ips {
			addr, err := netip.ParseAddr(ip)
			if err != nil {
				return err
			}
			bits := 128
			if addr.Is4() {
				bits = 32
			}
			content += fmt.Sprintf("          - %s/%d\n", ip, bits)
		}
	}
	if s.traefikCfg.IPStrategyDepth > 0 {
		content += fmt.Sprintf("        ipStrategy:\n          depth: %d\n", s.traefikCfg.IPStrategyDepth)
	}

	return os.WriteFile(s.traefik, []byte(content), 0644)
}

func sortedTemporary(state stateFile) []temporaryEntry {
	items := make([]temporaryEntry, 0, len(state.Temporary))
	for _, item := range state.Temporary {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ExpiresAt.Equal(items[j].ExpiresAt) {
			return items[i].IP < items[j].IP
		}
		return items[i].ExpiresAt.Before(items[j].ExpiresAt)
	})
	return items
}

func sortedPermanent(state stateFile) []permanentEntry {
	items := make([]permanentEntry, 0, len(state.Permanent))
	for _, item := range state.Permanent {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].IP < items[j].IP
	})
	return items
}
