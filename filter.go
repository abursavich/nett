// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"math/rand"
	"net"
)

// Filter selects addresses from addrs.
type Filter func(addrs Addrs) Addrs

// Addrs provides a way to interact with an enumerated
// collection of addresses.
type Addrs interface {
	// Len is the number of addresses in the collection.
	Len() int
	// Addr is the string form of the address at index i.
	Addr(i int) string
	// IP is the IP of the address at index i.
	IP(i int) net.IP
	// Append appends the address at index i to addrs,
	// which must be of the same type or nil.
	Append(addrs Addrs, i int) Addrs
}

type tcpAddrs []*net.TCPAddr
type udpAddrs []*net.UDPAddr
type ipAddrs []*net.IPAddr
type unixAddrs []*net.UnixAddr

func (a tcpAddrs) Len() int          { return len(a) }
func (a tcpAddrs) Addr(i int) string { return a[i].String() }
func (a tcpAddrs) IP(i int) net.IP   { return a[i].IP }
func (a tcpAddrs) Append(addrs Addrs, i int) Addrs {
	t, _ := addrs.(tcpAddrs)
	return append(t, a[i])
}

func (a udpAddrs) Len() int          { return len(a) }
func (a udpAddrs) Addr(i int) string { return a[i].String() }
func (a udpAddrs) IP(i int) net.IP   { return a[i].IP }
func (a udpAddrs) Append(addrs Addrs, i int) Addrs {
	t, _ := addrs.(udpAddrs)
	return append(t, a[i])
}

func (a ipAddrs) Len() int          { return len(a) }
func (a ipAddrs) Addr(i int) string { return a[i].String() }
func (a ipAddrs) IP(i int) net.IP   { return a[i].IP }
func (a ipAddrs) Append(addrs Addrs, i int) Addrs {
	t, _ := addrs.(ipAddrs)
	return append(t, a[i])
}

func (a unixAddrs) Len() int          { return len(a) }
func (a unixAddrs) Addr(i int) string { return a[i].String() }
func (a unixAddrs) IP(i int) net.IP   { return nil }
func (a unixAddrs) Append(addrs Addrs, i int) Addrs {
	t, _ := addrs.(unixAddrs)
	return append(t, a[i])
}

// DefaultFilter selects the first address IPv4 address
// in addrs. If only IPv6 addresses exist in addrs, then it
// selects the first IPv6 address.
func DefaultFilter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	addrsLen := addrs.Len()
	if addrsLen <= 1 {
		return addrs
	}
	ipv6 := -1
	for i := 0; i < addrsLen; i++ {
		if ipLen := len(addrs.IP(i)); ipLen == net.IPv4len {
			return addrs.Append(nil, i)
		} else if ipv6 < 0 && ipLen == net.IPv6len {
			ipv6 = i
		}
	}
	if ipv6 == -1 {
		return nil // shouldn't ever happen
	}
	return addrs.Append(nil, ipv6)
}

// NoFilter selects all addresses in addrs.
func NoFilter(addrs Addrs) Addrs {
	return addrs
}

// FirstFilter selects the first address in addrs.
func FirstFilter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	addrsLen := addrs.Len()
	if addrsLen <= 1 {
		return addrs
	}
	return addrs.Append(nil, 0)
}

// FirstEachFilter selects the first IPv4 address
// and IPv6 address in addrs.
func FirstEachFilter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	addrsLen := addrs.Len()
	if addrsLen <= 1 {
		return addrs
	}
	var (
		ipv4, ipv6 bool
		a          Addrs
	)
	for i := 0; i < addrsLen; i++ {
		if ipLen := len(addrs.IP(i)); !ipv4 && ipLen == net.IPv4len {
			a = addrs.Append(a, i)
			ipv4 = true
		} else if !ipv6 && ipLen == net.IPv6len {
			a = addrs.Append(a, i)
			ipv6 = true
		}
		if ipv4 && ipv6 {
			break
		}
	}
	return a
}

// FirstIPv4Filter selects the first IPv4 address in addrs.
func FirstIPv4Filter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	addrsLen := addrs.Len()
	for i := 0; i < addrsLen; i++ {
		if len(addrs.IP(i)) == net.IPv4len {
			return addrs.Append(nil, i)
		}
	}
	return nil
}

// FirstIPv6Filter selects the first IPv6 address in addrs.
func FirstIPv6Filter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	addrsLen := addrs.Len()
	for i := 0; i < addrsLen; i++ {
		if len(addrs.IP(i)) == net.IPv6len {
			return addrs.Append(nil, i)
		}
	}
	return nil
}

// IPv4Filter selects all IPv4 addresses in addrs.
func IPv4Filter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	var a Addrs
	addrsLen := addrs.Len()
	for i := 0; i < addrsLen; i++ {
		if len(addrs.IP(i)) == net.IPv4len {
			a = addrs.Append(a, i)
		}
	}
	return a
}

// IPv6Filter selects all IPv6 addresses in addrs.
func IPv6Filter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	var a Addrs
	addrsLen := addrs.Len()
	for i := 0; i < addrsLen; i++ {
		if len(addrs.IP(i)) == net.IPv6len {
			a = addrs.Append(a, i)
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
	return func(addrs Addrs) Addrs {
		if addrs == nil {
			return nil
		}
		addrsLen := addrs.Len()
		if addrsLen <= max {
			return addrs
		}
		var ipv4, ipv6 int
		for i := 0; i < addrsLen; i++ {
			if ipLen := len(addrs.IP(i)); ipLen == net.IPv4len {
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
		var a Addrs
		for i := 0; i < addrsLen; i++ {
			if ipLen := len(addrs.IP(i)); ipv4 > 0 && ipLen == net.IPv4len {
				a = addrs.Append(a, i)
				ipv4--
			} else if ipv6 > 0 && ipLen == net.IPv6len {
				a = addrs.Append(a, i)
				ipv6--
			}
		}
		return a
	}
}

// ReverseFilter selects all addresses in addrs
// in reverse order.
func ReverseFilter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	addrsLen := addrs.Len()
	if addrsLen <= 1 {
		return addrs
	}
	var a Addrs
	for i := addrsLen - 1; i >= 0; i-- {
		a = addrs.Append(a, i)
	}
	return a
}

// ShuffleFilter selects all addresses in addrs
// in random order.
func ShuffleFilter(addrs Addrs) Addrs {
	if addrs == nil {
		return nil
	}
	addrsLen := addrs.Len()
	if addrsLen <= 1 {
		return addrs
	}
	var a Addrs
	for _, i := range rand.Perm(addrsLen) {
		a = addrs.Append(a, i)
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
	return func(addrs Addrs) Addrs {
		for _, filter := range filters {
			addrs = filter(addrs)
		}
		return addrs
	}
}
