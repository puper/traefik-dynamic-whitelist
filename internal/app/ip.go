package app

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func ClientIP(r *http.Request, headers []string, trusted []netip.Prefix) (string, error) {
	ips, err := ClientIPs(r, headers, trusted)
	if err != nil {
		return "", err
	}
	return ips[0], nil
}

func ClientIPs(r *http.Request, headers []string, trusted []netip.Prefix) ([]string, error) {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	remote, err := netip.ParseAddr(remoteHost)
	if err != nil {
		return nil, err
	}
	remote = normalizeAddr(remote)

	if isTrusted(remote, trusted) {
		ips := make([]string, 0, len(headers))
		hasIPv4 := false
		hasIPv6 := false
		for _, header := range headers {
			for _, candidate := range strings.Split(r.Header.Get(header), ",") {
				addr, err := netip.ParseAddr(strings.TrimSpace(candidate))
				if err == nil {
					normalized := normalizeAddr(addr)
					if normalized.Is4() {
						if hasIPv4 {
							continue
						}
						hasIPv4 = true
					} else {
						if hasIPv6 {
							continue
						}
						hasIPv6 = true
					}
					ips = append(ips, normalized.String())
				}
			}
		}
		if len(ips) > 0 {
			return ips, nil
		}
	}

	return []string{remote.String()}, nil
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
