// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Internet protocol family sockets

package nett

import "net"

var (
	// supportsIPv4 reports whether the platform supports IPv4
	// networking functionality.
	supportsIPv4 bool

	// supportsIPv6 reports whether the platform supports IPv6
	// networking functionality.
	supportsIPv6 bool

	// supportsIPv4map reports whether the platform supports
	// mapping an IPv4 address inside an IPv6 address at transport
	// layer protocols.  See RFC 4291, RFC 4038 and RFC 3493.
	supportsIPv4map bool
)

func init() {
	supportsIPv4 = probeIPv4Stack()
	supportsIPv6, supportsIPv4map = probeIPv6Stack()
}

// supportedIP returns a version of the IP that the platform
// supports. If it is not supported it returns nil.
func supportedIP(ip net.IP) net.IP {
	if supportsIPv4 {
		if v4 := ip.To4(); v4 != nil {
			return v4
		}
	}
	if supportsIPv6 && len(ip) == net.IPv6len {
		return ip
	}
	return nil
}
