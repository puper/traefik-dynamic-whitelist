package app

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func ClientIP(r *http.Request, headers []string, trusted []netip.Prefix) (string, error) {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	remote, err := netip.ParseAddr(remoteHost)
	if err != nil {
		return "", err
	}
	remote = normalizeAddr(remote)

	if isTrusted(remote, trusted) {
		for _, header := range headers {
			for _, candidate := range strings.Split(r.Header.Get(header), ",") {
				addr, err := netip.ParseAddr(strings.TrimSpace(candidate))
				if err == nil {
					return normalizeAddr(addr).String(), nil
				}
			}
		}
	}

	return remote.String(), nil
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	if len(trusted) == 0 {
		return false
	}
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func normalizeAddr(addr netip.Addr) netip.Addr {
	if addr.Is4In6() {
		return addr.Unmap()
	}
	return addr
}
