// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDialHTTP(t *testing.T) {
	b := []byte("OK")
	h := func(w http.ResponseWriter, r *http.Request) { w.Write(b) }
	s := httptest.NewServer(http.HandlerFunc(h))
	defer s.Close()

	var d Dialer
	c := http.Client{Transport: &http.Transport{Dial: d.Dial}}
	resp, err := c.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, b) {
		t.Fatal("response doesn't match")
	}
}

func TestDialMulti(t *testing.T) {
	ips, err := lookupIPs("localhost")
	if err != nil {
		t.Fatalf("lookupIPs failed: %v", err)
	}
	if len(ips) < 2 || !supportsIPv4 || !supportsIPv6 {
		t.Skip("localhost doesn't have a pair of different address family IP addresses")
	}

	touchServer := func(dss *dualStackServer, ln net.Listener) {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}
	dss, err := newDualStackServer([]streamListener{
		{net: "tcp4", addr: "127.0.0.1"},
		{net: "tcp6", addr: "[::1]"},
	})
	if err != nil {
		t.Fatalf("newDualStackServer failed: %v", err)
	}
	defer dss.teardown()
	if err := dss.buildup(touchServer); err != nil {
		t.Fatalf("dualStackServer.buildup failed: %v", err)
	}

	d := &Dialer{IPFilter: DualStack} // dial all addresses
	for _ = range dss.lns {
		if c, err := d.Dial("tcp", "localhost:"+dss.port); err != nil {
			t.Errorf("Dial failed: %v", err)
		} else {
			if addr := c.LocalAddr().(*net.TCPAddr); addr.IP.To4() != nil {
				dss.teardownNetwork("tcp4")
			} else if addr.IP.To16() != nil && addr.IP.To4() == nil {
				dss.teardownNetwork("tcp6")
			}
			c.Close()
		}
	}
}
