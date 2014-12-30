// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testAddr struct {
	net, addr  string
	ips        []net.IP
	ipv4, ipv6 bool
	err        error
}

var testTCPAddrs, testUDPAddrs, testIPAddrs []testAddr

func init() {
	ipv4 := net.IP{127, 0, 0, 1}
	ipv6 := net.IPv6loopback
	ips := []net.IP{ipv4, ipv6}
	testTCPAddrs = []testAddr{
		{"tcp", "foo.com:http", ips, true, true, nil},
		{"tcp", "foo.com:80", ips, true, true, nil},
		{"tcp4", "foo.com:80", ips, true, false, nil},
		{"tcp4", "foo.com:80", ips, false, true, ErrNoSuitableAddress},
		{"tcp6", "foo.com:80", ips, false, true, nil},
		{"tcp6", "foo.com:80", ips, true, false, ErrNoSuitableAddress},
		{"tcp", "127.0.0.1:80", nil, true, true, nil},
		{"tcp6", "127.0.0.1:80", nil, false, true, ErrNoSuitableAddress},
		{"tcp", "[::1%911]:80", nil, false, true, nil},
		{"tcp", "[::1%911]:80", nil, true, false, ErrNoSuitableAddress},
	}
	testUDPAddrs = make([]testAddr, len(testTCPAddrs))
	testIPAddrs = make([]testAddr, len(testTCPAddrs))
	for i, ta := range testTCPAddrs {
		ta.net = strings.Replace(ta.net, "tcp", "udp", -1)
		testUDPAddrs[i] = ta
		ta.net = strings.Replace(ta.net, "udp", "ip", -1)
		ta.addr, _, _ = net.SplitHostPort(ta.addr) // TODO: shitty, but ok now
		testIPAddrs[i] = ta
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
		addrs, err := resolveAddrList(nil, nil, ta.net, ta.addr)
		if err != ta.err {
			t.Errorf("test %d: expecting error: %v\ngot: error: %v\n", i, ta.err, err)
		} else if err == nil && addrs.Len() == 0 {
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
		addrs, err := resolveAddrList(nil, nil, ta.net, ta.addr)
		if err != ta.err {
			t.Errorf("test: %#v\nexpecting error: %v\ngot error: %v\n", ta, ta.err, err)
		} else if err == nil && addrs.Len() == 0 {
			t.Errorf("test: %#v\nnet: %s; addr: %s\nno addresses\n", ta, ta.net, ta.addr)
		}
	}
}

func TestResolveIP(t *testing.T) {
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
	for _, ta := range testIPAddrs {
		ips = ta.ips
		supportsIPv4 = ta.ipv4
		supportsIPv6 = ta.ipv6
		addrs, err := resolveAddrList(nil, nil, ta.net, ta.addr)
		if err != ta.err {
			t.Errorf("test: %#v\nexpecting error: %v\ngot error: %v\n", ta, ta.err, err)
		} else if err == nil && addrs.Len() == 0 {
			t.Errorf("test: %#v\nnet: %s; addr: %s\nno addresses\n", ta, ta.net, ta.addr)
		}
	}
}

func TestCacheResolver(t *testing.T) {
	defer func(lookupFn func(string) ([]net.IP, error), timeFn func() time.Time) {
		lookupIPs = lookupFn
		timeNow = timeFn
	}(lookupIPs, timeNow)
	lookups := 0
	ips := []net.IP{net.IPv6loopback}
	lookupIPs = func(string) ([]net.IP, error) {
		lookups++
		return ips, nil
	}
	start := time.Now()
	now := start
	ttl := time.Second
	timeNow = func() time.Time { return now }
	resolver := &CacheResolver{TTL: ttl}
	validate := func(host string, expLookups int) {
		ips0, err := resolver.Resolve(host)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lookups != expLookups {
			t.Fatalf("lookups: expected %d; got %d", expLookups, lookups)
		}
		if !reflect.DeepEqual(ips, ips0) {
			t.Fatalf("ips: expected %v; got %v", ips, ips0)
		}
		ips0[0] = nil
		if reflect.DeepEqual(ips, ips0) {
			t.Fatal("ips: expected copy; got same")
		}
	}
	validate("foo.com", 1)       // lookup foo.com
	now = start.Add(ttl / 2)     //
	validate("bar.net", 2)       // lookup bar.net
	validate("foo.com", 2)       // cached
	now = start.Add(ttl)         // expire foo.com
	validate("foo.com", 3)       // lookup foo.com
	validate("bar.net", 3)       // cached
	now = start.Add(ttl + ttl/2) // expire bar.net
	validate("foo.com", 3)       // cached
	validate("bar.net", 4)       // lookup bar.net
}
