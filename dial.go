// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"net"
	"time"
)

var errTimeout = error(&timeoutError{})

// A Dialer contains options for connecting to an address.
type Dialer struct {
	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete. If Deadline is also set, it may fail
	// earlier.
	//
	// The default is no timeout.
	//
	// With or without a timeout, the operating system may impose
	// its own earlier timeout. For instance, TCP timeouts are
	// often around 3 minutes.
	Timeout time.Duration

	// Deadline is the absolute point in time after which dials
	// will fail. If Timeout is set, it may fail earlier.
	//
	// Zero means no deadline, or dependent on the operating system
	// as with the Timeout option.
	Deadline time.Time

	// LocalAddr is the local address to use when dialing an
	// address. The address must be of a compatible type for the
	// network being dialed.
	//
	// If nil, a local address is automatically chosen.
	LocalAddr net.Addr

	// Resolver is used to resolve IP addresses from domain names.
	//
	// If nil, DefaultResolver will be used.
	Resolver Resolver

	// IPFilter selects addresses from those available after
	// resolving a host to a set of supported IPs.
	//
	// When dialing a TCP connection if multiple addresses are
	// returned, then a connection will attempt to be established
	// with each address and the first that succeeds will be returned.
	// With any other type of connection, only the first address
	// returned will be dialed.
	//
	// If nil, a single address is selected.
	IPFilter func(ips []net.IP) []net.IP

	// KeepAlive specifies the keep-alive period for an active
	// network connection.
	//
	// If zero, keep-alives are not enabled. Network protocols
	// that do not support keep-alives ignore this field.
	//
	// Only functional for go1.3+.
	KeepAlive time.Duration
}

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
	filter := d.IPFilter
	if filter == nil {
		filter = defaultIP
	}
	addrs, err := resolveAddrsDeadline(d.Resolver, filter, network, address, deadline)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Addr: nil, Err: err}
	}
	dialer := d.netDialer(deadline)
	if addrs.Len() == 1 || len(network) < 3 || network[:3] != "tcp" {
		return dialer.Dial(network, addrs.Addr(0))
	}
	return dialMulti(dialer, network, addrs)
}

func resolveAddrsDeadline(resolver Resolver, filter ipFilter, network, address string, deadline time.Time) (addrList, error) {
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

// defaultIP gives priority to IPv4 addresses and selects the first address.
func defaultIP(ips []net.IP) []net.IP {
	if len(ips) <= 1 {
		return ips
	}
	v6 := -1
	for i, ip := range ips {
		if ipLen := len(ip); ipLen == net.IPv4len {
			return ips[i : i+1]
		} else if v6 == -1 && ipLen == net.IPv6len {
			v6 = i
		}
	}
	if v6 == -1 {
		return nil // shouldn't ever happen
	}
	return ips[v6 : v6+1]
}

// DualStack selects the first IPv4 address
// and IPv6 address in ips.
func DualStack(ips []net.IP) []net.IP {
	if len(ips) <= 1 {
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
