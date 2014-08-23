// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"net"
	"time"
)

type NotifyDialer interface {
	NotifyDial(network, address string, duration time.Duration, err error)
}

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
	// Zero means no deadline, or dependent on the operating system
	// as with the Timeout option.
	Deadline time.Time

	// LocalAddr is the local address to use when dialing an
	// address. The address must be of a compatible type for the
	// network being dialed.
	//
	// If nil, a local address is automatically chosen.
	LocalAddr net.Addr

	// Resolver is used to resolve addresses.
	//
	// When dialing a TCP connection if multiple addresses are
	// returned, then a connection will attempt to be established
	// with each address and the first that succeeds will be returned.
	// With any other type of connection, only the first address
	// returned will be dialed.
	//
	// If the Resolver also satifies NotifyDialer, it will be
	// notified of the results of all dials.
	//
	// If nil, DefaultResolver will be used.
	Resolver Resolver

	// KeepAlive specifies the keep-alive period for an active
	// network connection.
	//
	// If zero, keep-alives are not enabled. Network protocols
	// that do not support keep-alives ignore this field.
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
	resolver := d.Resolver
	if resolver == nil {
		resolver = defaultResolver
	}
	addrs, err := resolveAddrsDeadline(resolver, network, address, deadline)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Addr: nil, Err: err}
	}
	if addrs.Len() == 0 {
		return nil, errNoSuitableAddress
	}
	dialer := net.Dialer{
		Deadline:  deadline,
		LocalAddr: d.LocalAddr,
		KeepAlive: d.KeepAlive,
	}
	if addrs.Len() == 1 || len(network) < 3 || network[:3] != "tcp" {
		return d.dialSingle(dialer, network, addrs.Addr(0))
	}
	return d.dialMulti(dialer, network, addrs)
}

func resolveAddrsDeadline(resolver Resolver, network, address string, deadline time.Time) (AddrList, error) {
	if deadline.IsZero() {
		return resolver.ResolveAddrs(network, address)
	}

	timeout := deadline.Sub(time.Now())
	if timeout <= 0 {
		return nil, errTimeout
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	type res struct {
		AddrList
		error
	}
	resc := make(chan res, 1)
	go func() {
		addrs, err := resolver.ResolveAddrs(network, address)
		resc <- res{addrs, err}
	}()
	select {
	case <-t.C:
		return nil, errTimeout
	case r := <-resc:
		return r.AddrList, r.error
	}
}

func (d *Dialer) dialSingle(dialer net.Dialer, network, address string) (net.Conn, error) {
	s := time.Now()
	c, err := dialer.Dial(network, address)
	if nd, ok := d.Resolver.(NotifyDialer); ok {
		nd.NotifyDial(network, address, time.Since(s), err)
	}
	return c, err
}

// dialMulti attempts to establish connections to each destination of
// the list of addresses. It will return the first established
// connection and close the other connections. Otherwise it returns
// error on the last attempt.
func (d *Dialer) dialMulti(dialer net.Dialer, network string, addrs AddrList) (net.Conn, error) {
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
			c, err := d.dialSingle(dialer, network, addrs.Addr(i))
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
