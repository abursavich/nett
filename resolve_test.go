// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"net"
	"strings"
	"testing"
)

type testAddr struct {
	net, addr  string
	ips        []net.IP
	ipv4, ipv6 bool
	err        error
}

var testTCPAddrs, testUDPAddrs []testAddr

func init() {
	ipv4 := net.IP{127, 0, 0, 1}
	ipv6 := net.IPv6loopback
	ips := []net.IP{ipv4, ipv6}
	testTCPAddrs = []testAddr{
		{"tcp", "foo.com:http", ips, true, true, nil},
		{"tcp", "foo.com:80", ips, true, true, nil},
		{"tcp4", "foo.com:80", ips, true, false, nil},
		{"tcp4", "foo.com:80", ips, false, true, errNoSuitableAddress},
		{"tcp6", "foo.com:80", ips, false, true, nil},
		{"tcp6", "foo.com:80", ips, true, false, errNoSuitableAddress},
	}
	testUDPAddrs = make([]testAddr, len(testTCPAddrs))
	for i, ta := range testTCPAddrs {
		ta.net = strings.Replace(ta.net, "tcp", "udp", -1)
		testUDPAddrs[i] = ta
	}
}

func TestResolveTCP(t *testing.T) {
	defer func(fn func(string) ([]net.IP, error), ipv4, ipv6 bool) {
		lookupIPs = fn
		supportsIPv4 = ipv4
		supportsIPv6 = ipv6
	}(lookupIPs, supportsIPv4, supportsIPv6)
	var ips []net.IP
	lookupIPs = func(host string) ([]net.IP, error) {
		clone := make([]net.IP, len(ips))
		copy(clone, ips)
		return clone, nil
	}
	for i, ta := range testTCPAddrs {
		ips = ta.ips
		supportsIPv4 = ta.ipv4
		supportsIPv6 = ta.ipv6
		addrs, err := ResolveTCPAddrs(nil, nil, ta.net, ta.addr)
		if err != ta.err {
			t.Errorf("test %d: expecting error: %v\ngot: error: %v\n", i, ta.err, err)
		} else if err == nil && len(addrs) == 0 {
			t.Errorf("test %d: net: %s; addr: %s\nno addresses\n", i, ta.net, ta.addr)
		}
	}
}

func TestResolveUDP(t *testing.T) {
	defer func(fn func(string) ([]net.IP, error), ipv4, ipv6 bool) {
		lookupIPs = fn
		supportsIPv4 = ipv4
		supportsIPv6 = ipv6
	}(lookupIPs, supportsIPv4, supportsIPv6)
	var ips []net.IP
	lookupIPs = func(host string) ([]net.IP, error) {
		clone := make([]net.IP, len(ips))
		copy(clone, ips)
		return clone, nil
	}
	for _, ta := range testUDPAddrs {
		ips = ta.ips
		supportsIPv4 = ta.ipv4
		supportsIPv6 = ta.ipv6
		addrs, err := ResolveUDPAddrs(nil, nil, ta.net, ta.addr)
		if err != ta.err {
			t.Errorf("test: %#v\nexpecting error: %v\ngot error: %v\n", ta, ta.err, err)
		} else if err == nil && len(addrs) == 0 {
			t.Errorf("test: %#v\nnet: %s; addr: %s\nno addresses\n", ta, ta.net, ta.addr)
		}
	}
}
