package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreAddMovesIPBetweenLists(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "whitelist.yml"), 24*time.Hour)
	now := time.Date(2026, 5, 12, 6, 30, 0, 0, time.UTC)

	if err := store.Add("203.0.113.10", "temp", now); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("203.0.113.10", "perm", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	temporary, permanent, err := store.Info(now.Add(2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 {
		t.Fatalf("temporary should be empty: %#v", temporary)
	}
	if len(permanent) != 1 || permanent[0].IP != "203.0.113.10" {
		t.Fatalf("unexpected permanent list: %#v", permanent)
	}
}

func TestStoreWritesTraefikConfig(t *testing.T) {
	dir := t.TempDir()
	traefikPath := filepath.Join(dir, "whitelist.yml")
	store := NewStore(filepath.Join(dir, "state.json"), traefikPath, 24*time.Hour)
	now := time.Date(2026, 5, 12, 6, 30, 0, 0, time.UTC)

	if err := store.Add("203.0.113.10", "temp", now); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(traefikPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "dynamic-whitelist") || !strings.Contains(string(data), "203.0.113.10/32") {
		t.Fatalf("unexpected traefik config:\n%s", string(data))
	}
}

func TestStoreAddManyWritesIPv4AndIPv6(t *testing.T) {
	dir := t.TempDir()
	traefikPath := filepath.Join(dir, "whitelist.yml")
	store := NewStore(filepath.Join(dir, "state.json"), traefikPath, 24*time.Hour)
	now := time.Date(2026, 5, 12, 6, 30, 0, 0, time.UTC)

	if err := store.AddMany([]string{"203.0.113.10", "2001:db8::10"}, "perm", now); err != nil {
		t.Fatal(err)
	}

	_, permanent, err := store.Info(now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(permanent) != 2 {
		t.Fatalf("unexpected permanent list: %#v", permanent)
	}

	data, err := os.ReadFile(traefikPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "203.0.113.10/32") || !strings.Contains(content, "2001:db8::10/128") {
		t.Fatalf("unexpected traefik config:\n%s", content)
	}
}

func TestStoreWritesTraefikIPStrategyDepth(t *testing.T) {
	dir := t.TempDir()
	traefikPath := filepath.Join(dir, "whitelist.yml")
	store := NewStore(filepath.Join(dir, "state.json"), traefikPath, 24*time.Hour, TraefikConfig{IPStrategyDepth: 1})
	now := time.Date(2026, 5, 12, 6, 30, 0, 0, time.UTC)

	if err := store.Add("203.0.113.10", "temp", now); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(traefikPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "ipStrategy:") || !strings.Contains(content, "depth: 1") {
		t.Fatalf("unexpected traefik config:\n%s", content)
	}
}

func TestStoreDeleteRemovesIPAndUpdatesTraefikConfig(t *testing.T) {
	dir := t.TempDir()
	traefikPath := filepath.Join(dir, "whitelist.yml")
	store := NewStore(filepath.Join(dir, "state.json"), traefikPath, 24*time.Hour)
	now := time.Date(2026, 5, 12, 6, 30, 0, 0, time.UTC)

	if err := store.Add("203.0.113.10", "perm", now); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("203.0.113.10", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	temporary, permanent, err := store.Info(now.Add(2 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 || len(permanent) != 0 {
		t.Fatalf("unexpected state: temporary=%#v permanent=%#v", temporary, permanent)
	}

	data, err := os.ReadFile(traefikPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "203.0.113.10/32") {
		t.Fatalf("deleted ip still exists in traefik config:\n%s", string(data))
	}
}
