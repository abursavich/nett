// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"net"
	"testing"
)

func TestResolveGoogleTCP(t *testing.T) {
	if _, err := net.ResolveTCPAddr("tcp", "google.com:80"); err != nil {
		t.Skipf("google.com not found: %v", err)
	}
	type test struct{ net, addr string }
	tests := []test{
		{"tcp", "google.com:http"},
		{"tcp", "google.com:80"},
	}
	if supportsIPv4 {
		tests = append(tests, test{"tcp4", "google.com:80"})
	}
	if supportsIPv6 {
		tests = append(tests, test{"tcp6", "google.com:80"})
	}
	for _, tt := range tests {
		addrs, err := ResolveTCPAddrs(tt.net, tt.addr)
		if err != nil {
			t.Errorf("net: %s; addr: %s\nerror: %v\n", tt.net, tt.addr, err)
		} else if len(addrs) == 0 {
			t.Errorf("net: %s; addr: %s\nno addresses\n", tt.net, tt.addr)
		}
	}
}

func TestResolveGoogleUDP(t *testing.T) {
	if _, err := net.ResolveTCPAddr("udp", "google.com:80"); err != nil {
		t.Skipf("google.com not found: %v", err)
	}
	type test struct{ net, addr string }
	tests := []test{
		{"udp", "google.com:http"},
		{"udp", "google.com:80"},
	}
	if supportsIPv4 {
		tests = append(tests, test{"udp4", "google.com:80"})
	}
	if supportsIPv6 {
		tests = append(tests, test{"udp6", "google.com:80"})
	}
	for _, tt := range tests {
		addrs, err := ResolveUDPAddrs(tt.net, tt.addr)
		if err != nil {
			t.Errorf("net: %s; addr: %s\nerror: %v\n", tt.net, tt.addr, err)
		} else if len(addrs) == 0 {
			t.Errorf("net: %s; addr: %s\nno addresses\n", tt.net, tt.addr)
		}
	}
}
