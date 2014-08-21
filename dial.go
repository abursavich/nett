// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"math/rand"
	"net"
	"time"
)

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

	// AddrFilter is a method for selecting addresses to dial
	// from those available after resolving a host name.
	//
	// When dialing a TCP connection if this method returns multiple
	// addresses, then a connection will attempt to be established
	// with each address and the first that succeeds will be returned.
	// With any other type of connection, only the first address
	// returned will be dialed.
	//
	// If nil, DefaultAddrFilter will be used.
	AddrFilter AddrFilter

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
// See func Dial for a description of the network and address
// parameters.
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	deadline := d.deadline()
	addrs, err := resolveAddrs(network, address, deadline)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Addr: nil, Err: err}
	}
	filter := d.AddrFilter
	if d.AddrFilter == nil {
		filter = DefaultAddrFilter
	}
	addrs = filter(addrs)
	if addrs.Len() == 0 {
		return nil, errNoSuitableAddress
	}
	dialer := net.Dialer{
		Deadline:  deadline,
		LocalAddr: d.LocalAddr,
		KeepAlive: d.KeepAlive,
	}
	if addrs.Len() == 1 || len(network) < 3 || network[:3] != "tcp" {
		return dialer.Dial(network, addrs.Addr(0))
	}
	return dialMulti(dialer, network, addrs)
}

// dialMulti attempts to establish connections to each destination of
// the list of addresses. It will return the first established
// connection and close the other connections. Otherwise it returns
// error on the last attempt.
func dialMulti(dialer net.Dialer, network string, addrs AddrList) (net.Conn, error) {
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
func Dial(network, address string) (net.Conn, error) {
	var d Dialer
	return d.Dial(network, address)
}

// DialTimeout acts like Dial but takes a timeout.
// The timeout includes name resolution, if required.
func DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	d := Dialer{Timeout: timeout}
	return d.Dial(network, address)
}

// AddrFilter selects addresses from an AddrList.
type AddrFilter func(list AddrList) AddrList

// DefaultAddrFilter selects the first IPv4 address
// and the first IPv6 address in list.
func DefaultAddrFilter(list AddrList) AddrList {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	a := list.New()
	var ipv4, ipv6 bool
	for i := 0; i < listLen; i++ {
		if ipLen := len(list.IP(i)); !ipv4 && ipLen == net.IPv4len {
			a.Append(list, i)
			ipv4 = true
		} else if !ipv6 && ipLen == net.IPv6len {
			a.Append(list, i)
			ipv6 = true
		}
		if ipv4 && ipv6 {
			break
		}
	}
	return a
}

// SingleAddrFilter selects the first address in list
// preferring IPv4 over IPv6.
func SingleAddrFilter(list AddrList) AddrList {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	ipv6 := -1
	for i := 0; i < listLen; i++ {
		if ipLen := len(list.IP(i)); ipLen == net.IPv4len {
			return list.Slice(i, i+1)
		} else if ipv6 < 0 {
			ipv6 = i
		}
	}
	return list.Slice(ipv6, ipv6+1)
}

// IPv4AddrFilter selects all IPv4 addresses in list.
func IPv4AddrFilter(list AddrList) AddrList {
	a := list.New()
	listLen := list.Len()
	for i := 0; i < listLen; i++ {
		if len(list.IP(i)) == net.IPv4len {
			a.Append(list, i)
		}
	}
	return a
}

// IPv6AddrFilter selects all IPv6 addresses in list.
func IPv6AddrFilter(list AddrList) AddrList {
	a := list.New()
	listLen := list.Len()
	for i := 0; i < listLen; i++ {
		if len(list.IP(i)) == net.IPv6len {
			a.Append(list, i)
		}
	}
	return a
}

// MaxAddrFilter returns an AddrFilter that selects up to max
// addresses. It will split the results evenly between availabe
// IPv4 and IPv6 addresses. If one type of address doesn't exist
// in sufficient quantity to consume its share, the other type
// will be allowed to fill any extra space in the result.
// Addresses toward the front of the list are preferred.
func MaxAddrFilter(max int) AddrFilter {
	return func(list AddrList) AddrList {
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
				a.Append(list, i)
				ipv4--
			} else if ipv6 > 0 && ipLen == net.IPv6len {
				a.Append(list, i)
				ipv6--
			}
		}
		return a
	}
}

// ReverseAddrFilter selects all addresses in list
// in reverse order.
func ReverseAddrFilter(list AddrList) AddrList {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	a := list.New()
	for i := listLen - 1; i >= 0; i-- {
		a.Append(list, i)
	}
	return a
}

// ShuffleAddrFilter selects all addresses in list
// in random order.
func ShuffleAddrFilter(list AddrList) AddrList {
	listLen := list.Len()
	if listLen <= 1 {
		return list
	}
	a := list.New()
	for _, i := range rand.Perm(listLen) {
		a.Append(list, i)
	}
	return a
}

// ComposeAddrFilters returns an AddrFilter that applies
// filters in sequence.
//
// Example:
// 	ComposeAddrFilters(ShuffleAddrFilter, DefaultAddrFilter) // selects one random IPv4 and IPv6 address
func ComposeAddrFilters(filters ...AddrFilter) AddrFilter {
	return func(list AddrList) AddrList {
		for _, filter := range filters {
			list = filter(list)
		}
		return list
	}
}

// AddrList provides a way to interact with and construct
// an enumerated collection of addresses.
type AddrList interface {
	// Network is the network of the list.
	Network() string
	// Addr is the string form of the address at index i.
	Addr(i int) string
	// IP is the IP of the address at index i.
	IP(i int) net.IP
	// Len is the number of addresses in the list.
	Len() int

	// New creates a new empty list with the same type.
	New() AddrList
	// Slice slices a new list from the list starting
	// at start and ending at end.
	Slice(start int, end int) AddrList

	// Append appends the address at index i in src
	// to the list.
	Append(src AddrList, i int)
	// Swap swaps the addresses with indexes i and j.
	Swap(i, j int)
}

type tcpList []*net.TCPAddr
type udpList []*net.UDPAddr
type ipList []*net.IPAddr
type unixList []*net.UnixAddr

func (list *tcpList) Network() string   { return "tcp" }
func (list *tcpList) Addr(i int) string { return (*list)[i].String() }
func (list *tcpList) IP(i int) net.IP   { return (*list)[i].IP }
func (list *tcpList) Len() int          { return len(*list) }
func (*tcpList) New() AddrList {
	list := make(tcpList, 0)
	return &list
}
func (list *tcpList) Append(src AddrList, i int) {
	if srcList, ok := src.(*tcpList); ok {
		*list = append(*list, (*srcList)[i])
	}
}
func (list *tcpList) Slice(start, end int) AddrList {
	a := (*list)[start:end]
	return &a
}
func (list *tcpList) Swap(i, j int) {
	a := *list
	a[i], a[j] = a[j], a[i]
}

func (list *udpList) Network() string   { return "udp" }
func (list *udpList) Addr(i int) string { return (*list)[i].String() }
func (list *udpList) IP(i int) net.IP   { return (*list)[i].IP }
func (list *udpList) Len() int          { return len(*list) }
func (*udpList) New() AddrList {
	list := make(udpList, 0)
	return &list
}
func (list *udpList) Append(src AddrList, i int) {
	if srcList, ok := src.(*udpList); ok {
		*list = append(*list, (*srcList)[i])
	}
}
func (list *udpList) Slice(start, end int) AddrList {
	a := (*list)[start:end]
	return &a
}
func (list *udpList) Swap(i, j int) {
	a := *list
	a[i], a[j] = a[j], a[i]
}

func (list *ipList) Network() string   { return "ip" }
func (list *ipList) Addr(i int) string { return (*list)[i].String() }
func (list *ipList) IP(i int) net.IP   { return (*list)[i].IP }
func (list *ipList) Len() int          { return len(*list) }
func (*ipList) New() AddrList {
	list := make(ipList, 0)
	return &list
}
func (list *ipList) Append(src AddrList, i int) {
	if srcList, ok := src.(*ipList); ok {
		*list = append(*list, (*srcList)[i])
	}
}
func (list *ipList) Slice(start, end int) AddrList {
	a := (*list)[start:end]
	return &a
}
func (list *ipList) Swap(i, j int) {
	a := *list
	a[i], a[j] = a[j], a[i]
}

func (list *unixList) Network() string   { return "unix" }
func (list *unixList) Addr(i int) string { return (*list)[i].String() }
func (list *unixList) IP(i int) net.IP   { return nil }
func (list *unixList) Len() int          { return len(*list) }
func (*unixList) New() AddrList {
	list := make(unixList, 0)
	return &list
}
func (list *unixList) Append(src AddrList, i int) {
	if srcList, ok := src.(*unixList); ok {
		*list = append(*list, (*srcList)[i])
	}
}
func (list *unixList) Slice(start, end int) AddrList {
	a := (*list)[start:end]
	return &a
}
func (list *unixList) Swap(i, j int) {
	a := *list
	a[i], a[j] = a[j], a[i]
}
