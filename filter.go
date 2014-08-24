// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"math/rand"
	"net"
)

// Filter selcts IP addresses from ips.
type Filter func(ips []net.IP) []net.IP

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

// FirstFilter selects the first address in ips.
func FirstFilter(ips []net.IP) []net.IP {
	if len(ips) <= 1 {
		return ips
	}
	return []net.IP{ips[0]}
}

// FirstEachFilter selects the first IPv4 address
// and IPv6 address in ips.
func FirstEachFilter(ips []net.IP) []net.IP {
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

// FirstIPv4Filter selects the first IPv4 address in ips.
func FirstIPv4Filter(ips []net.IP) []net.IP {
	for _, ip := range ips {
		if len(ip) == net.IPv4len {
			return []net.IP{ip}
		}
	}
	return nil
}

// FirstIPv6Filter selects the first IPv6 address in ips.
func FirstIPv6Filter(ips []net.IP) []net.IP {
	for _, ip := range ips {
		if len(ip) == net.IPv6len {
			return []net.IP{ip}
		}
	}
	return nil
}

// IPv4Filter selects all IPv4 addresses in ips.
func IPv4Filter(ips []net.IP) []net.IP {
	var a []net.IP
	for _, ip := range ips {
		if len(ip) == net.IPv4len {
			a = append(a, ip)
		}
	}
	return a
}

// IPv6Filter selects all IPv6 addresses in ips.
func IPv6Filter(ips []net.IP) []net.IP {
	var a []net.IP
	for _, ip := range ips {
		if len(ip) == net.IPv6len {
			a = append(a, ip)
		}
	}
	return a
}

// MaxFilter returns an Filter that selects up to max
// addresses. It will split the results evenly between availabe
// IPv4 and IPv6 addresses. If one type of address doesn't exist
// in sufficient quantity to consume its share, the other type
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

// ShuffleFilter selects all addresses in ips
// in random order.
func ShuffleFilter(ips []net.IP) []net.IP {
	k := len(ips)
	if k <= 1 {
		return ips
	}
	a := make([]net.IP, k)
	n := 0
	for _, i := range rand.Perm(k) {
		a[n] = ips[i]
		n++
	}
	return a
}

// ComposeFilters returns an Filter that applies
// filters in sequence.
//
// Example:
//	// selects one random IPv4 and IPv6 address
//	ComposeFilters(ShuffleFilter, FirstEachFilter)
//	// equivalent to FirstIPv4Filter
//	ComposeFilters(IPv4Filter, FirstFilter)
func ComposeFilters(filters ...Filter) Filter {
	return func(ips []net.IP) []net.IP {
		for _, filter := range filters {
			ips = filter(ips)
		}
		return ips
	}
}
