// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import "testing"

func TestDialGoogleTCP(t *testing.T) {
	if len(googleTCPAddrs) == 0 {
		t.Skipf("google.com not found")
	}
	var d Dialer
	for _, ta := range googleTCPAddrs {
		c, err := d.Dial(ta.net, ta.addr)
		if err != nil {
			t.Errorf("net: %s; addr: %s\nerror: %v\n", ta.net, ta.addr, err)
			continue
		}
		c.Close()
	}
}
