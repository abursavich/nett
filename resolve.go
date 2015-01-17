// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	ErrMissingAddress    = errors.New("missing address")
	ErrNoSuitableAddress = errors.New("no suitable address found")

	lookupIPs = net.LookupIP // used by tests
	timeNow   = time.Now     // used by tests
)

// Resolver is an interface representing the ability to lookup the
// IP addresses of a host. It may return results containing networks
// not supported by the platform.
//
// A Resolver must be safe for concurrent use by multiple goroutines.
type Resolver interface {
	// Resolve looks up the given host and returns its IP addresses.
	Resolve(host string) ([]net.IP, error)
}

// DefaultResolver is the default Resolver.
var DefaultResolver Resolver = defaultResolver{}

// defaultResolver uses the local resolver.
type defaultResolver struct{}

// Resolve looks up the given host using the local resolver.
// It returns an array of that host's IPv4 and IPv6 addresses.
func (defaultResolver) Resolve(host string) ([]net.IP, error) {
	return lookupIPs(host)
}

// CacheResolver looks up the IP addresses of a host
// and caches successful results.
type CacheResolver struct {
	// Resolver resolves hosts that are not cached.
	// If Resolver is nil, DefaultResolver will be used.
	Resolver Resolver
	// TTL is the time to live for resolved hosts.
	// If TTL is zero, cached hosts do not expire.
	TTL time.Duration

	mu    sync.RWMutex
	cache map[string]*cacheItem
}

type cacheItem struct {
	ips []net.IP
	ttl time.Time
}

// Resolve returns a host's IP addresses.
func (r *CacheResolver) Resolve(host string) ([]net.IP, error) {
	r.mu.RLock()
	if item, ok := r.cache[host]; ok {
		if item.ttl.IsZero() || timeNow().Before(item.ttl) {
			r.mu.RUnlock()
			ips := make([]net.IP, len(item.ips))
			copy(ips, item.ips)
			return ips, nil
		}
	}
	r.mu.RUnlock()

	resolver := r.Resolver
	if resolver == nil {
		resolver = DefaultResolver
	}
	ips, err := resolver.Resolve(host)
	if err != nil {
		return nil, err
	}

	var ttl time.Time
	if r.TTL > 0 {
		ttl = timeNow().Add(r.TTL)
	}
	item := &cacheItem{ips, ttl}
	r.mu.Lock()
	if r.cache == nil {
		r.cache = make(map[string]*cacheItem)
	}
	r.cache[host] = item
	r.mu.Unlock()

	ips = make([]net.IP, len(item.ips))
	copy(ips, item.ips)
	return ips, err
}

// ipFilter selects IP addresses from ips.
type ipFilter func(ips []net.IP) []net.IP

func resolveAddrList(resolver Resolver, filter ipFilter, network, address string) (addrList, error) {
	nett, err := parseNetwork(network)
	if err != nil {
		return nil, err
	}
	if address == "" {
		return nil, ErrMissingAddress
	}
	switch nett {
	case "unix", "unixgram", "unixpacket":
		return unixList{&net.UnixAddr{Name: address, Net: nett}}, nil
	}
	return resolveInternetAddrList(resolver, filter, nett, address)
}

func resolveInternetAddrList(resolver Resolver, filter ipFilter, network, address string) (addrList, error) {
	host, port, err := parseHostPort(network, address)
	if err != nil {
		return nil, err
	}
	var zone string
	ctor := func(ips ...net.IP) addrList {
		switch network {
		case "tcp", "tcp4", "tcp6":
			addrs := make(tcpList, len(ips))
			for i, ip := range ips {
				addrs[i] = &net.TCPAddr{IP: ip, Port: port, Zone: zone}
			}
			return addrs
		case "udp", "udp4", "udp6":
			addrs := make(udpList, len(ips))
			for i, ip := range ips {
				addrs[i] = &net.UDPAddr{IP: ip, Port: port, Zone: zone}
			}
			return addrs
		case "ip", "ip4", "ip6":
			addrs := make(ipList, len(ips))
			for i, ip := range ips {
				addrs[i] = &net.IPAddr{IP: ip, Zone: zone}
			}
			return addrs
		default:
			panic("unexpected network: " + network)
		}
	}
	if host == "" {
		return ctor(nil), nil
	}
	var ips []net.IP
	// Try as a literal IP address.
	if ip := parseIPv4(host); ip != nil {
		ips = []net.IP{ip}
	} else if ip, zone = parseIPv6(host, true); ip != nil {
		ips = []net.IP{ip}
	} else {
		// Try as a DNS name.
		host, zone = splitHostZone(host)
		if !isDomainName(host) {
			return nil, &net.DNSError{Err: "invalid domain name", Name: host}
		}
		if resolver == nil {
			resolver = DefaultResolver
		}
		ips, err = resolver.Resolve(host)
		if err != nil {
			return nil, err
		}
	}
	supported := supportedIP
	if network[len(network)-1] == '4' {
		supported = ipv4only
	} else if network[len(network)-1] == '6' || zone != "" {
		supported = ipv6only
	}
	ips = filterIPs(supported, ips)
	if filter != nil {
		ips = filter(ips)
	}
	if len(ips) == 0 {
		return nil, ErrNoSuitableAddress
	}
	return ctor(ips...), nil
}

func parseNetwork(network string) (string, error) {
	i := last(network, ':')
	if i < 0 { // no colon
		switch network {
		case "tcp", "tcp4", "tcp6":
		case "udp", "udp4", "udp6":
		case "ip", "ip4", "ip6":
		case "unix", "unixgram", "unixpacket":
		default:
			return "", net.UnknownNetworkError(network)
		}
		return network, nil
	}
	nett := network[:i]
	switch nett {
	case "ip", "ip4", "ip6":
		// Don't bother validating the protocol. The consequence of this is that
		// an invalid protocol will fail at dial-time instead of resolve-time.
		// A case might be made for doing it here like the net package does,
		// but the cost of duplicating that logic from the standard library
		// doesn't currently seem justified.
		return nett, nil
	}
	return "", net.UnknownNetworkError(network)
}

func parseHostPort(network, address string) (host string, port int, err error) {
	if address == "" {
		err = ErrMissingAddress
		return
	}
	switch network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		var portstr string
		if host, portstr, err = net.SplitHostPort(address); err != nil {
			return
		}
		port, err = parsePort(network, portstr)
	case "ip", "ip4", "ip6":
		host = address
	default:
		err = net.UnknownNetworkError(network)
	}
	return
}

// parsePort parses port as a network service port number for both
// TCP and UDP.
func parsePort(network, port string) (int, error) {
	p, i, ok := dtoi(port, 0)
	if !ok || i != len(port) {
		var err error
		p, err = net.LookupPort(network, port)
		if err != nil {
			return 0, err
		}
	}
	if p < 0 || p > 0xFFFF {
		return 0, &net.AddrError{"invalid port", port}
	}
	return p, nil
}

// parseIPv4 parses s as an IPv4 address.
func parseIPv4(s string) net.IP {
	var p [net.IPv4len]byte
	i := 0
	for j := 0; j < net.IPv4len; j++ {
		if i >= len(s) {
			// Missing octets.
			return nil
		}
		if j > 0 {
			if s[i] != '.' {
				return nil
			}
			i++
		}
		var (
			n  int
			ok bool
		)
		n, i, ok = dtoi(s, i)
		if !ok || n > 0xFF {
			return nil
		}
		p[j] = byte(n)
	}
	if i != len(s) {
		return nil
	}
	return net.IPv4(p[0], p[1], p[2], p[3])
}

// parseIPv6 parses s as a literal IPv6 address described in RFC 4291
// and RFC 5952.  It can also parse a literal scoped IPv6 address with
// zone identifier which is described in RFC 4007 when zoneAllowed is
// true.
func parseIPv6(s string, zoneAllowed bool) (ip net.IP, zone string) {
	ip = make(net.IP, net.IPv6len)
	ellipsis := -1 // position of ellipsis in p
	i := 0         // index in string s

	if zoneAllowed {
		s, zone = splitHostZone(s)
	}

	// Might have leading ellipsis
	if len(s) >= 2 && s[0] == ':' && s[1] == ':' {
		ellipsis = 0
		i = 2
		// Might be only ellipsis
		if i == len(s) {
			return ip, zone
		}
	}

	// Loop, parsing hex numbers followed by colon.
	j := 0
	for j < net.IPv6len {
		// Hex number.
		n, i1, ok := xtoi(s, i)
		if !ok || n > 0xFFFF {
			return nil, zone
		}

		// If followed by dot, might be in trailing IPv4.
		if i1 < len(s) && s[i1] == '.' {
			if ellipsis < 0 && j != net.IPv6len-net.IPv4len {
				// Not the right place.
				return nil, zone
			}
			if j+net.IPv4len > net.IPv6len {
				// Not enough room.
				return nil, zone
			}
			ip4 := parseIPv4(s[i:])
			if ip4 == nil {
				return nil, zone
			}
			ip[j] = ip4[12]
			ip[j+1] = ip4[13]
			ip[j+2] = ip4[14]
			ip[j+3] = ip4[15]
			i = len(s)
			j += net.IPv4len
			break
		}

		// Save this 16-bit chunk.
		ip[j] = byte(n >> 8)
		ip[j+1] = byte(n)
		j += 2

		// Stop at end of string.
		i = i1
		if i == len(s) {
			break
		}

		// Otherwise must be followed by colon and more.
		if s[i] != ':' || i+1 == len(s) {
			return nil, zone
		}
		i++

		// Look for ellipsis.
		if s[i] == ':' {
			if ellipsis >= 0 { // already have one
				return nil, zone
			}
			ellipsis = j
			if i++; i == len(s) { // can be at end
				break
			}
		}
	}

	// Must have used entire string.
	if i != len(s) {
		return nil, zone
	}

	// If didn't parse enough, expand ellipsis.
	if j < net.IPv6len {
		if ellipsis < 0 {
			return nil, zone
		}
		n := net.IPv6len - j
		for k := j - 1; k >= ellipsis; k-- {
			ip[k+n] = ip[k]
		}
		for k := ellipsis + n - 1; k >= ellipsis; k-- {
			ip[k] = 0
		}
	} else if ellipsis >= 0 {
		// Ellipsis must represent at least one 0 group.
		return nil, zone
	}
	return ip, zone
}

func splitHostZone(s string) (host, zone string) {
	// The IPv6 scoped addressing zone identifier starts after the
	// last percent sign.
	if i := last(s, '%'); i > 0 {
		host, zone = s[:i], s[i+1:]
	} else {
		host = s
	}
	return
}

// filterIPs returns the non-nil results of filter applied to ips.
// It is processed in-place: the contents of ips is not preserved
// and the result is sliced from its backing array.
func filterIPs(filter func(net.IP) net.IP, ips []net.IP) []net.IP {
	n := 0
	for i := range ips {
		if ip := filter(ips[i]); ip != nil {
			ips[n] = ip
			n++
		}
	}
	return ips[:n]
}

func isDomainName(s string) bool {
	// See RFC 1035, RFC 3696.
	if len(s) == 0 {
		return false
	}
	if len(s) > 255 {
		return false
	}

	last := byte('.')
	ok := false // Ok once we've seen a letter.
	partlen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_':
			ok = true
			partlen++
		case '0' <= c && c <= '9':
			// fine
			partlen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = c
	}
	if last == '-' || partlen > 63 {
		return false
	}

	return ok
}

// ipv4only returns IPv4 addresses that we can use with the kernel's
// IPv4 addressing modes. If ip is an IPv4 address, ipv4only returns ip.
// Otherwise it returns nil.
func ipv4only(ip net.IP) net.IP {
	if supportsIPv4 {
		return ip.To4()
	}
	return nil
}

// ipv6only returns IPv6 addresses that we can use with the kernel's
// IPv6 addressing modes.  It returns IPv4-mapped IPv6 addresses as
// nils and returns other IPv6 address types as IPv6 addresses.
func ipv6only(ip net.IP) net.IP {
	if supportsIPv6 && len(ip) == net.IPv6len && ip.To4() == nil {
		return ip
	}
	return nil
}
