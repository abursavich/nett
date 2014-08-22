// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"math/rand"
	"net"
)

// AddrsFilter selects addresses from an Addrs.
type AddrsFilter func(list Addrs) Addrs

// Addrs provides a way to interact with and construct
// an enumerated collection of addresses.
type Addrs interface {
	// Network is the network of the list.
	Network() string
	// Addr is the string form of the address at index i.
	Addr(i int) string
	// IP is the IP of the address at index i.
	IP(i int) net.IP
	// Len is the number of addresses in the list.
	Len() int

	// New creates a new empty list with the same type.
	New() Addrs
	// Slice slices a new list from the list starting
	// at start and ending at end.
	Slice(start int, end int) Addrs
	// Append appends the address at index i in src
	// to the list.
	Append(src Addrs, i int) Addrs
	// Swap swaps the addresses with indexes i and j.
	Swap(i, j int)
}

type tcpAddrs []*net.TCPAddr
type udpAddrs []*net.UDPAddr
type ipAddrs []*net.IPAddr
type unixAddrs []*net.UnixAddr

func (tcpAddrs) New() Addrs                      { return make(tcpAddrs, 0) }
func (tcpAddrs) Network() string                 { return "tcp" }
func (a tcpAddrs) Addr(i int) string             { return a[i].String() }
func (a tcpAddrs) IP(i int) net.IP               { return a[i].IP }
func (a tcpAddrs) Len() int                      { return len(a) }
func (a tcpAddrs) Append(src Addrs, i int) Addrs { return append(a, src.(tcpAddrs)[i]) }
func (a tcpAddrs) Slice(start, end int) Addrs    { return a[start:end] }
func (a tcpAddrs) Swap(i, j int)                 { a[i], a[j] = a[j], a[i] }

func (udpAddrs) New() Addrs                      { return make(udpAddrs, 0) }
func (udpAddrs) Network() string                 { return "udp" }
func (a udpAddrs) Addr(i int) string             { return a[i].String() }
func (a udpAddrs) IP(i int) net.IP               { return a[i].IP }
func (a udpAddrs) Len() int                      { return len(a) }
func (a udpAddrs) Append(src Addrs, i int) Addrs { return append(a, src.(udpAddrs)[i]) }
func (a udpAddrs) Slice(start, end int) Addrs    { return a[start:end] }
func (a udpAddrs) Swap(i, j int)                 { a[i], a[j] = a[j], a[i] }

func (ipAddrs) New() Addrs                      { return make(ipAddrs, 0) }
func (ipAddrs) Network() string                 { return "ip" }
func (a ipAddrs) Addr(i int) string             { return a[i].String() }
func (a ipAddrs) IP(i int) net.IP               { return a[i].IP }
func (a ipAddrs) Len() int                      { return len(a) }
func (a ipAddrs) Append(src Addrs, i int) Addrs { return append(a, src.(ipAddrs)[i]) }
func (a ipAddrs) Slice(start, end int) Addrs    { return a[start:end] }
func (a ipAddrs) Swap(i, j int)                 { a[i], a[j] = a[j], a[i] }

func (unixAddrs) New() Addrs                      { return make(unixAddrs, 0) }
func (unixAddrs) Network() string                 { return "unix" }
func (a unixAddrs) Addr(i int) string             { return a[i].String() }
func (a unixAddrs) IP(i int) net.IP               { return nil }
func (a unixAddrs) Len() int                      { return len(a) }
func (a unixAddrs) Append(src Addrs, i int) Addrs { return append(a, src.(unixAddrs)[i]) }
func (a unixAddrs) Slice(start, end int) Addrs    { return a[start:end] }
func (a unixAddrs) Swap(i, j int)                 { a[i], a[j] = a[j], a[i] }

// DefaultAddrsFilter selects the first address IPv4 address
// in list. If only IPv6 addresses exist in list, then it
// selects the first IPv6 address.
func DefaultAddrsFilter(list Addrs) Addrs {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	ipv6 := -1
	for i := 0; i < listLen; i++ {
		if ipLen := len(list.IP(i)); ipLen == net.IPv4len {
			return list.Slice(i, i+1)
		} else if ipv6 < 0 && ipLen == net.IPv6len {
			ipv6 = i
		}
	}
	return list.Slice(ipv6, ipv6+1)
}

// NoAddrsFilter selects all addresses in the list.
func NoAddrsFilter(list Addrs) Addrs {
	return list
}

// FirstAddrsFilter selects the first address in list.
func FirstAddrsFilter(list Addrs) Addrs {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	return list.Slice(0, 1)
}

// FirstEachAddrsFilter selects the first IPv4 address
// and IPv6 address in list.
func FirstEachAddrsFilter(list Addrs) Addrs {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	a := list.New()
	var ipv4, ipv6 bool
	for i := 0; i < listLen; i++ {
		if ipLen := len(list.IP(i)); !ipv4 && ipLen == net.IPv4len {
			a = a.Append(list, i)
			ipv4 = true
		} else if !ipv6 && ipLen == net.IPv6len {
			a = a.Append(list, i)
			ipv6 = true
		}
		if ipv4 && ipv6 {
			break
		}
	}
	return a
}

// FirstIPv4AddrsFilter selects the first IPv4 address in list.
func FirstIPv4AddrsFilter(list Addrs) Addrs {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	for i := 0; i < listLen; i++ {
		if len(list.IP(i)) == net.IPv4len {
			return list.New().Append(list, i)
		}
	}
	return list.New()
}

// FirstIPv6AddrsFilter selects the first IPv6 address in list.
func FirstIPv6AddrsFilter(list Addrs) Addrs {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	a := list.New()
	for i := 0; i < listLen; i++ {
		if len(list.IP(i)) == net.IPv6len {
			return a.Append(list, i)
		}
	}
	return a
}

// IPv4AddrsFilter selects all IPv4 addresses in list.
func IPv4AddrsFilter(list Addrs) Addrs {
	a := list.New()
	listLen := list.Len()
	for i := 0; i < listLen; i++ {
		if len(list.IP(i)) == net.IPv4len {
			a = a.Append(list, i)
		}
	}
	return a
}

// IPv6AddrsFilter selects all IPv6 addresses in list.
func IPv6AddrsFilter(list Addrs) Addrs {
	a := list.New()
	listLen := list.Len()
	for i := 0; i < listLen; i++ {
		if len(list.IP(i)) == net.IPv6len {
			a = a.Append(list, i)
		}
	}
	return a
}

// MaxAddrsFilter returns an AddrsFilter that selects up to max
// addresses. It will split the results evenly between availabe
// IPv4 and IPv6 addresses. If one type of address doesn't exist
// in sufficient quantity to consume its share, the other type
// will be allowed to fill any extra space in the result.
// Addresses toward the front of the list are preferred.
func MaxAddrsFilter(max int) AddrsFilter {
	return func(list Addrs) Addrs {
		listLen := list.Len()
		if listLen <= max {
			return list
		}
		var ipv4, ipv6 int
		for i := 0; i < listLen; i++ {
			if ipLen := len(list.IP(i)); ipLen == net.IPv4len {
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
		a := list.New()
		for i := 0; i < listLen; i++ {
			if ipLen := len(list.IP(i)); ipv4 > 0 && ipLen == net.IPv4len {
				a = a.Append(list, i)
				ipv4--
			} else if ipv6 > 0 && ipLen == net.IPv6len {
				a = a.Append(list, i)
				ipv6--
			}
		}
		return a
	}
}

// ReverseAddrsFilter selects all addresses in list
// in reverse order.
func ReverseAddrsFilter(list Addrs) Addrs {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	a := list.New()
	for i := listLen - 1; i >= 0; i-- {
		a = a.Append(list, i)
	}
	return a
}

// ShuffleAddrsFilter selects all addresses in list
// in random order.
func ShuffleAddrsFilter(list Addrs) Addrs {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	a := list.New()
	for _, i := range rand.Perm(listLen) {
		a = a.Append(list, i)
	}
	return a
}

// ComposeAddrsFilters returns an AddrsFilter that applies
// filters in sequence.
//
// Example:
//	// selects one random IPv4 and IPv6 address
//	ComposeAddrsFilters(ShuffleAddrsFilter, FirstEachAddrsFilter)
//	// equivalent to FirstIPv4AddrsFilter
//	ComposeAddrsFilters(IPv4AddrsFilter, FirstAddrsFilter)
func ComposeAddrsFilters(filters ...AddrsFilter) AddrsFilter {
	return func(list Addrs) Addrs {
		for _, filter := range filters {
			list = filter(list)
		}
		return list
	}
}
