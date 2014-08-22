// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"errors"
	"net"
)

var (
	errTimeout           = error(&timeoutError{})
	errMissingAddress    = errors.New("missing address")
	errNoSuitableAddress = errors.New("no suitable address found")

	defaultResolver = &DefaultResolver{}
)

type Resolver interface {
	ResolveAddrs(network, address string) (Addrs, error)
}

type DefaultResolver struct {
	// Filter selects addresses from those available after
	// resolving a host. It is not applied to Unix addresses.
	//
	// If nil, DefaultAddrsFilter is used.
	Filter AddrsFilter
}

func (r *DefaultResolver) ResolveAddrs(network, address string) (Addrs, error) {
	filter := r.Filter
	if filter == nil {
		filter = DefaultAddrsFilter
	}
	return resolveAddrs(network, address, filter)
}

// ResolveTCPAddrs parses address as a TCP address of the form "host:port"
// or "[ipv6-host%zone]:port" and resolves list of pairs of domain name and
// port number on the network, which must be "tcp", "tcp4" or "tcp6".
// A literal address or host name for IPv6 must be enclosed in square
// brackets, as in "[::1]:80", "[ipv6-host]:http" or "[ipv6-host%zone]:80".
func ResolveTCPAddrs(network, address string) ([]*net.TCPAddr, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	iaddrs, err := resolveInternetAddrs(network, address)
	if err != nil {
		return nil, err
	}
	return iaddrs.(tcpAddrs), nil
}

// ResolveUDPAddrs parses address as a UDP address of the form "host:port"
// or "[ipv6-host%zone]:port" and resolves a list of pairs of domain name and
// port number on the network, which must be "upd", "upd4" or "udp6".
// A literal address or host name for IPv6 must be enclosed in square
// brackets, as in "[::1]:80", "[ipv6-host]:http" or "[ipv6-host%zone]:80".
func ResolveUDPAddrs(network, address string) ([]*net.UDPAddr, error) {
	switch network {
	case "udp", "udp4", "udp6":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	iaddrs, err := resolveInternetAddrs(network, address)
	if err != nil {
		return nil, err
	}
	return iaddrs.(udpAddrs), nil
}

// ResolveIPAddrs parses address as an IP address of the form "host" or
// "ipv6-host%zone" and resolves the list of domain names on the network,
// which must be "ip", "ip4" or "ip6".
func ResolveIPAddrs(network, address string) ([]*net.IPAddr, error) {
	nett, err := parseNetwork(network)
	if err != nil {
		return nil, err
	}
	switch nett {
	case "ip", "ip4", "ip6":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	iaddrs, err := resolveInternetAddrs(nett, address)
	if err != nil {
		return nil, err
	}
	return iaddrs.(ipAddrs), nil
}

// ResolveUnixAddrs parses address as a Unix domain socket address.
// The string network gives the network name "unix", "unixgram" or
// "unixpacket".
func ResolveUnixAddrs(network, address string) ([]*net.UnixAddr, error) {
	// this function is really stupid but included for completeness/symmetry
	switch network {
	case "unix", "unixgram", "unixpacket":
		return []*net.UnixAddr{&net.UnixAddr{Name: address, Net: network}}, nil
	default:
		return nil, net.UnknownNetworkError(network)
	}
}

func resolveAddrs(network, address string, filter AddrsFilter) (Addrs, error) {
	nett, err := parseNetwork(network)
	if err != nil {
		return nil, err
	}
	if address == "" {
		return nil, errMissingAddress
	}
	switch nett {
	case "unix", "unixgram", "unixpacket":
		return unixAddrs{&net.UnixAddr{Name: address, Net: nett}}, nil
	}
	addrs, err := resolveInternetAddrs(nett, address)
	if filter != nil {
		addrs = filter(addrs)
	}
	if addrs == nil || addrs.Len() == 0 {
		return nil, errNoSuitableAddress
	}
	return addrs, nil
}

func resolveInternetAddrs(network, address string) (Addrs, error) {
	var (
		err              error
		host, port, zone string
		portnum          int
	)
	switch network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		if address != "" {
			if host, port, err = net.SplitHostPort(address); err != nil {
				return nil, err
			}
			if portnum, err = parsePort(network, port); err != nil {
				return nil, err
			}
		}
	case "ip", "ip4", "ip6":
		host = address
	default:
		return nil, net.UnknownNetworkError(network)
	}
	ctor := func(ips ...net.IP) Addrs {
		switch network {
		case "tcp", "tcp4", "tcp6":
			addrs := make(tcpAddrs, len(ips))
			for i, ip := range ips {
				addrs[i] = &net.TCPAddr{IP: ip, Port: portnum, Zone: zone}
			}
			return addrs
		case "udp", "udp4", "udp6":
			addrs := make(udpAddrs, len(ips))
			for i, ip := range ips {
				addrs[i] = &net.UDPAddr{IP: ip, Port: portnum, Zone: zone}
			}
			return addrs
		case "ip", "ip4", "ip6":
			addrs := make(ipAddrs, len(ips))
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
	// Try as a literal IP address.
	var ip net.IP
	if ip = parseIPv4(host); ip != nil {
		return ctor(ip), nil
	}
	if ip, zone = parseIPv6(host, true); ip != nil {
		return ctor(ip), nil
	}
	// Try as a DNS name.
	host, zone = splitHostZone(host)
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	filter := SupportedIP
	if network != "" && network[len(network)-1] == '4' {
		filter = ipv4only
	}
	if network != "" && network[len(network)-1] == '6' || zone != "" {
		filter = ipv6only
	}
	ips = filterIPs(filter, ips)
	if len(ips) == 0 {
		return nil, errNoSuitableAddress
	}
	return ctor(ips...), nil
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
		// don't bother validating the proto
		return nett, nil
	}
	return "", net.UnknownNetworkError(network)
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

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
