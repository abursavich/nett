// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !go1.3

package nett

import (
	"net"
	"time"
)

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

	// Filter selects addresses from those available after
	// resolving a host to a set of supported IPs.
	//
	// When dialing a TCP connection if multiple addresses are
	// returned, then a connection will attempt to be established
	// with each address and the first that succeeds will be returned.
	// With any other type of connection, only the first address
	// returned will be dialed.
	//
	// If nil, DefaultFilter is used.
	Filter Filter
}

func (d *Dialer) netDialer(deadline time.Time) net.Dialer {
	return net.Dialer{
		Deadline:  deadline,
		LocalAddr: d.LocalAddr,
	}
}
