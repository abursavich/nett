// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"errors"
	"net"
	"time"
)

var noDeadline = time.Time{}

var (
	errTimeout           error = &timeoutError{}
	errMissingAddress          = errors.New("missing addr")
	errNoSuitableAddress       = errors.New("no suitable addr found")
)

// ResolveTCPAddrs parses addr as a TCP address of the form "host:port"
// or "[ipv6-host%zone]:port" and resolves a pair of domain name and port
// name on the network nett, which must be "tcp", "tcp4" or "tcp6".
// A literal address or host name for IPv6 must be enclosed in square
// brackets, as in "[::1]:80", "[ipv6-host]:http" or "[ipv6-host%zone]:80".
func ResolveTCPAddrs(nett, addr string) ([]*net.TCPAddr, error) {
	switch nett {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, net.UnknownNetworkError(nett)
	}
	iaddrs, err := resolveAddrs(nett, addr, noDeadline)
	if err != nil {
		return nil, err
	}
	return iaddrs.([]*net.TCPAddr), nil
}

// ResolveUDPAddrs parses addr as a UDP address of the form "host:port"
// or "[ipv6-host%zone]:port" and resolves a pair of domain name and port
// name on the network nett, which must be "upd", "upd4" or "udp6".
// A literal address or host name for IPv6 must be enclosed in square
// brackets, as in "[::1]:80", "[ipv6-host]:http" or "[ipv6-host%zone]:80".
func ResolveUDPAddrs(nett, addr string) ([]*net.UDPAddr, error) {
	switch nett {
	case "udp", "udp4", "udp6":
	default:
		return nil, net.UnknownNetworkError(nett)
	}
	iaddrs, err := resolveAddrs(nett, addr, noDeadline)
	if err != nil {
		return nil, err
	}
	return iaddrs.([]*net.UDPAddr), nil
}

// ResolveIPAddrs parses addr as an IP address of the form "host" or
// "ipv6-host%zone" and resolves the domain name on the network nett,
// which must be "ip", "ip4" or "ip6".
func ResolveIPAddrs(nett, addr string) ([]*net.IPAddr, error) {
	nettt, err := parseNetwork(nett)
	if err != nil {
		return nil, err
	}
	switch nettt {
	case "ip", "ip4", "ip6":
	default:
		return nil, net.UnknownNetworkError(nett)
	}
	iaddrs, err := resolveAddrs(nettt, addr, noDeadline)
	if err != nil {
		return nil, err
	}
	return iaddrs.([]*net.IPAddr), nil
}

// ResolveUnixAddrs parses addr as a Unix domain socket address.
// The string nett gives the network name "unix", "unixgram" or
// "unixpacket".
func ResolveUnixAddrs(nett, addr string) ([]*net.UnixAddr, error) {
	// this function is really stupid but included for completeness/symmetry
	switch nett {
	case "unix", "unixgram", "unixpacket":
		return []*net.UnixAddr{&net.UnixAddr{Name: addr, Net: nett}}, nil
	default:
		return nil, net.UnknownNetworkError(nett)
	}
}

func resolveAddrs(nett, addr string, deadline time.Time) (interface{}, error) {
	nettt, err := parseNetwork(nett)
	if err != nil {
		return nil, err
	}
	if addr == "" {
		return nil, errMissingAddress
	}
	switch nettt {
	case "unix", "unixgram", "unixpacket":
		return []*net.UnixAddr{&net.UnixAddr{Name: addr, Net: nett}}, nil
	}
	return resolveInternetAddrs(nettt, addr, deadline)
}

func resolveInternetAddrs(nett, addr string, deadline time.Time) (interface{}, error) {
	var (
		err              error
		host, port, zone string
		portnum          int
	)
	switch nett {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		if addr != "" {
			if host, port, err = net.SplitHostPort(addr); err != nil {
				return nil, err
			}
			if portnum, err = parsePort(nett, port); err != nil {
				return nil, err
			}
		}
	case "ip", "ip4", "ip6":
		host = addr
	default:
		return nil, net.UnknownNetworkError(nett)
	}
	appendAddr := func(addrs interface{}, ip net.IP) interface{} {
		switch nett {
		case "tcp", "tcp4", "tcp6":
			a := &net.TCPAddr{IP: ip, Port: portnum, Zone: zone}
			if addrs == nil {
				return []*net.TCPAddr{a}
			}
			return append(addrs.([]*net.TCPAddr), a)
		case "udp", "udp4", "udp6":
			a := &net.UDPAddr{IP: ip, Port: portnum, Zone: zone}
			if addrs == nil {
				return []*net.UDPAddr{a}
			}
			return append(addrs.([]*net.UDPAddr), a)
		case "ip", "ip4", "ip6":
			a := &net.IPAddr{IP: ip, Zone: zone}
			if addrs == nil {
				return []*net.IPAddr{a}
			}
			return append(addrs.([]*net.IPAddr), a)
		default:
			panic("unexpected network: " + nett)
		}
	}
	if host == "" {
		return appendAddr(nil, nil), nil
	}
	// Try as a literal IP address.
	var ip net.IP
	if ip = parseIPv4(host); ip != nil {
		return appendAddr(nil, ip), nil
	}
	if ip, zone = parseIPv6(host, true); ip != nil {
		return appendAddr(nil, ip), nil
	}
	// Try as a DNS name.
	host, zone = splitHostZone(host)
	ips, err := lookupIPDeadline(host, deadline)
	if err != nil {
		return nil, err
	}
	filter := SupportedIP
	if nett != "" && nett[len(nett)-1] == '4' {
		filter = ipv4only
	}
	if nett != "" && nett[len(nett)-1] == '6' || zone != "" {
		filter = ipv6only
	}
	return filterAddrs(filter, ips, appendAddr)
}

func lookupIPDeadline(host string, deadline time.Time) ([]net.IP, error) {
	if deadline.IsZero() {
		return net.LookupIP(host)
	}

	// TODO(bradfitz): consider pushing the deadline down into the
	// name resolution functions. But that involves fixing it for
	// the native Go resolver, cgo, Windows, etc.
	//
	// In the meantime, just use a goroutine. Most users affected
	// by http://golang.org/issue/2631 are due to TCP connections
	// to unresponsive hosts, not DNS.
	timeout := deadline.Sub(time.Now())
	if timeout <= 0 {
		return nil, errTimeout
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	type res struct {
		ips []net.IP
		err error
	}
	resc := make(chan res, 1)
	go func() {
		ips, err := net.LookupIP(host)
		resc <- res{ips, err}
	}()
	select {
	case <-t.C:
		return nil, errTimeout
	case r := <-resc:
		return r.ips, r.err
	}
}

func filterAddrs(filter func(net.IP) net.IP, ips []net.IP, appendAddr func(interface{}, net.IP) interface{}) (interface{}, error) {
	var addrs interface{}
	for i := range ips {
		if ip := filter(ips[i]); ip != nil {
			addrs = appendAddr(addrs, ip)
		}
	}
	if addrs == nil {
		return nil, errNoSuitableAddress
	}
	// TODO(abursavich): Sort addresses? Package net prefers
	// IPv4 to IPv6 but uses at maximum one of each kind.
	return addrs, nil
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

func parseNetwork(nett string) (string, error) {
	i := last(nett, ':')
	if i < 0 { // no colon
		switch nett {
		case "tcp", "tcp4", "tcp6":
		case "udp", "udp4", "udp6":
		case "ip", "ip4", "ip6":
		case "unix", "unixgram", "unixpacket":
		default:
			return "", net.UnknownNetworkError(nett)
		}
		return nett, nil
	}
	nettt := nett[:i]
	switch nettt {
	case "ip", "ip4", "ip6":
		// TODO(abursavich): Come back to parsing
		// the proto if/when it's time to dial IP.
		return nettt, nil
	}
	return "", net.UnknownNetworkError(nett)
}

// parsePort parses port as a network service port number for both
// TCP and UDP.
func parsePort(nett, port string) (int, error) {
	p, i, ok := dtoi(port, 0)
	if !ok || i != len(port) {
		var err error
		p, err = net.LookupPort(nett, port)
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
