// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Internet protocol family sockets for Plan 9

package nett

func probe(filename, query string) bool {
	var file *file
	var err error
	if file, err = open(filename); err != nil {
		return false
	}

	r := false
	for line, ok := file.readLine(); ok && !r; line, ok = file.readLine() {
		f := getFields(line)
		if len(f) < 3 {
			continue
		}
		for i := 0; i < len(f); i++ {
			if query == f[i] {
				r = true
				break
			}
		}
	}
	file.close()
	return r
}

func probeIPv4Stack() bool {
	return probe("/net/iproute", "4i")
}

// probeIPv6Stack returns two boolean values.  If the first boolean
// value is true, kernel supports basic IPv6 functionality.  If the
// second boolean value is true, kernel supports IPv6 IPv4-mapping.
func probeIPv6Stack() (supportsIPv6, supportsIPv4map bool) {
	// Plan 9 uses IPv6 natively, see ip(3).
	r := probe("/net/iproute", "6i")
	v := false
	if r {
		v = probe("/net/iproute", "4i")
	}
	return r, v
}
