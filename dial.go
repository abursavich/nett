// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"net"
	"time"
)

var errTimeout = error(&timeoutError{})

// Filter selcts IP addresses from ips.
type Filter func(ips []net.IP) []net.IP

// Return either now+Timeout or Deadline, whichever comes first.
// Or zero, if neither is set.
func (d *Dialer) deadline() time.Time {
	if d.Timeout == 0 {
		return d.Deadline
	}
	timeout := time.Now().Add(d.Timeout)
	if d.Deadline.IsZero() || timeout.Before(d.Deadline) {
		return timeout
	}
	return d.Deadline
}

// Dial connects to the address on the named network.
//
// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only),
// "udp", "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4"
// (IPv4-only), "ip6" (IPv6-only), "unix", "unixgram" and
// "unixpacket".
//
// For TCP and UDP networks, addresses have the form host:port.
// If host is a literal IPv6 address it must be enclosed
// in square brackets as in "[::1]:80" or "[ipv6-host%zone]:80".
// The functions JoinHostPort and SplitHostPort manipulate addresses
// in this form.
//
// Examples:
//	Dial("tcp", "12.34.56.78:80")
//	Dial("tcp", "google.com:http")
//	Dial("tcp", "[2001:db8::1]:http")
//	Dial("tcp", "[fe80::1%lo0]:80")
//
// For IP networks, the network must be "ip", "ip4" or "ip6" followed
// by a colon and a protocol number or name and the addr must be a
// literal IP address.
//
// Examples:
//	Dial("ip4:1", "127.0.0.1")
//	Dial("ip6:ospf", "::1")
//
// For Unix networks, the address must be a file system path.
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	deadline := d.deadline()
	resolver := d.Resolver
	filter := d.Filter
	if filter == nil {
		filter = DefaultFilter
	}
	addrs, err := resolveAddrsDeadline(resolver, filter, network, address, deadline)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Addr: nil, Err: err}
	}
	dialer := d.netDialer(deadline)
	if addrs.Len() == 1 || len(network) < 3 || network[:3] != "tcp" {
		return dialer.Dial(network, addrs.Addr(0))
	}
	return dialMulti(dialer, network, addrs)
}

func resolveAddrsDeadline(resolver Resolver, filter Filter, network, address string, deadline time.Time) (addrList, error) {
	if deadline.IsZero() {
		return resolveAddrList(resolver, filter, network, address)
	}

	timeout := deadline.Sub(time.Now())
	if timeout <= 0 {
		return nil, errTimeout
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	type res struct {
		addrList
		error
	}
	resc := make(chan res, 1)
	go func() {
		addrs, err := resolveAddrList(resolver, filter, network, address)
		resc <- res{addrs, err}
	}()
	select {
	case <-t.C:
		return nil, errTimeout
	case r := <-resc:
		return r.addrList, r.error
	}
}

// dialMulti attempts to establish connections to each destination of
// the list of addresses. It will return the first established
// connection and close the other connections. Otherwise it returns
// error on the last attempt.
func dialMulti(dialer net.Dialer, network string, addrs addrList) (net.Conn, error) {
	type racer struct {
		net.Conn
		error
	}
	addrsLen := addrs.Len()
	// Sig controls the flow of dial results on lane. It passes a
	// token to the next racer and also indicates the end of flow
	// by using closed channel.
	sig := make(chan bool, 1)
	lane := make(chan racer, 1)
	for i := 0; i < addrsLen; i++ {
		go func(i int) {
			c, err := dialer.Dial(network, addrs.Addr(i))
			if _, ok := <-sig; ok {
				lane <- racer{c, err}
			} else if err == nil {
				// We have to return the resources
				// that belong to the other
				// connections here for avoiding
				// unnecessary resource starvation.
				c.Close()
			}
		}(i)
	}
	defer close(sig)
	lastErr := errTimeout
	for i := 0; i < addrsLen; i++ {
		sig <- true
		racer := <-lane
		if racer.error == nil {
			return racer.Conn, nil
		}
		lastErr = racer.error
	}
	return nil, lastErr
}

// DefaultFilter selects the first IPv4 address in ips.
// If only IPv6 addresses exist in ips, then it selects
// the first IPv6 address.
func DefaultFilter(ips []net.IP) []net.IP {
	if len(ips) <= 1 {
		return ips
	}
	var ipv6 net.IP
	for _, ip := range ips {
		if ipLen := len(ip); ipLen == net.IPv4len {
			return []net.IP{ip}
		} else if ipv6 == nil && ipLen == net.IPv6len {
			ipv6 = ip
		}
	}
	if ipv6 == nil {
		return nil // shouldn't ever happen
	}
	return []net.IP{ipv6}
}

// DualStackFilter selects the first IPv4 address
// and IPv6 address in ips.
func DualStackFilter(ips []net.IP) []net.IP {
	k := len(ips)
	if k <= 1 {
		return ips
	}
	var (
		ipv4, ipv6 bool
		a          []net.IP
	)
	for _, ip := range ips {
		if ipLen := len(ip); !ipv4 && ipLen == net.IPv4len {
			a = append(a, ip)
			ipv4 = true
		} else if !ipv6 && ipLen == net.IPv6len {
			a = append(a, ip)
			ipv6 = true
		}
		if ipv4 && ipv6 {
			break
		}
	}
	return a
}

// NoFilter selects all IP addresses.
func NoFilter(ips []net.IP) []net.IP {
	return ips
}

// MaxFilter returns a Filter that selects up to max addresses.
// It will split the results evenly between availabe IPv4 and
// IPv6 addresses. If one type of address doesn't exist in
// sufficient quantity to consume its share, the other type
// will be allowed to fill any extra space in the result.
// Addresses toward the front of the collection are preferred.
func MaxFilter(max int) Filter {
	return func(ips []net.IP) []net.IP {
		if len(ips) <= max {
			return ips
		}
		var ipv4, ipv6 int
		for _, ip := range ips {
			if ipLen := len(ip); ipLen == net.IPv4len {
				ipv4++
			} else if ipLen == net.IPv6len {
				ipv6++
			}
		}
		if halfLen := max / 2; ipv6 <= halfLen {
			ipv4 = max - ipv6
		} else if ipv4 <= halfLen {
			ipv6 = max - ipv4
		} else {
			ipv4 = max - halfLen // give rounding benefit to ipv4
			ipv6 = halfLen
		}
		var a []net.IP
		for _, ip := range ips {
			if ipLen := len(ip); ipv4 > 0 && ipLen == net.IPv4len {
				a = append(a, ip)
				ipv4--
			} else if ipv6 > 0 && ipLen == net.IPv6len {
				a = append(a, ip)
				ipv6--
			}
			if ipv4 == 0 && ipv6 == 0 {
				break
			}
		}
		return a
	}
}

type addrList interface {
	Len() int
	Addr(i int) string
}

type tcpList []*net.TCPAddr
type udpList []*net.UDPAddr
type ipList []*net.IPAddr
type unixList []*net.UnixAddr

func (list tcpList) Len() int          { return len(list) }
func (list tcpList) Addr(i int) string { return list[i].String() }

func (list udpList) Len() int          { return len(list) }
func (list udpList) Addr(i int) string { return list[i].String() }

func (list ipList) Len() int          { return len(list) }
func (list ipList) Addr(i int) string { return list[i].String() }

func (list unixList) Len() int          { return len(list) }
func (list unixList) Addr(i int) string { return list[i].String() }

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
