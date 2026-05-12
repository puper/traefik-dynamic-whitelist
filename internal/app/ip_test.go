package app

import (
	"net/http"
	"net/netip"
	"testing"
)

func TestClientIPUsesRemoteAddrWhenProxyIsNotTrusted(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	got, err := ClientIP(req, []string{"X-Forwarded-For"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPUsesConfiguredHeaderForTrustedProxy(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")

	got, err := ClientIP(req, []string{"X-Forwarded-For"}, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	if got != "203.0.113.10" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPsCollectsIPv4AndIPv6FromTrustedHeaders(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	req.Header.Set("X-Forwarded-For", "2001:db8::10, 203.0.113.10")

	got, err := ClientIPs(req, []string{"CF-Connecting-IP", "X-Forwarded-For"}, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"203.0.113.10", "2001:db8::10"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestClientIPsKeepsOnlyFirstIPPerFamily(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	req.Header.Set("X-Forwarded-For", "198.51.100.20, 2001:db8::10, 2001:db8::20")

	got, err := ClientIPs(req, []string{"CF-Connecting-IP", "X-Forwarded-For"}, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"203.0.113.10", "2001:db8::10"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}
