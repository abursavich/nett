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
	net, addr string
}

var googleTCPAddrs, googleUDPAddrs []testAddr

func init() {
	if _, err := net.ResolveTCPAddr("tcp", "google.com:80"); err != nil {
		return
	}
	googleTCPAddrs = []testAddr{
		{"tcp", "google.com:http"},
		{"tcp", "google.com:80"},
	}
	if supportsIPv4 {
		googleTCPAddrs = append(googleTCPAddrs, testAddr{"tcp4", "google.com:80"})
	}
	if supportsIPv6 {
		if _, err := net.ResolveTCPAddr("tcp6", "google.com:80"); err == nil {
			googleTCPAddrs = append(googleTCPAddrs, testAddr{"tcp6", "google.com:80"})
		}
	}
	googleUDPAddrs = make([]testAddr, len(googleTCPAddrs))
	for i, ta := range googleTCPAddrs {
		ta.net = strings.Replace(ta.net, "tcp", "udp", -1)
		googleUDPAddrs[i] = ta
	}
}

func TestResolveGoogleTCP(t *testing.T) {
	if len(googleTCPAddrs) == 0 {
		t.Skipf("google.com not found")
	}
	for _, ta := range googleTCPAddrs {
		addrs, err := ResolveTCPAddrs(ta.net, ta.addr)
		if err != nil {
			t.Errorf("net: %s; addr: %s\nerror: %v\n", ta.net, ta.addr, err)
		} else if len(addrs) == 0 {
			t.Errorf("net: %s; addr: %s\nno addresses\n", ta.net, ta.addr)
		}
	}
}

func TestResolveGoogleUDP(t *testing.T) {
	if len(googleUDPAddrs) == 0 {
		t.Skipf("google.com not found")
	}
	for _, ta := range googleUDPAddrs {
		addrs, err := ResolveUDPAddrs(ta.net, ta.addr)
		if err != nil {
			t.Errorf("net: %s; addr: %s\nerror: %v\n", ta.net, ta.addr, err)
		} else if len(addrs) == 0 {
			t.Errorf("net: %s; addr: %s\nno addresses\n", ta.net, ta.addr)
		}
	}
}
