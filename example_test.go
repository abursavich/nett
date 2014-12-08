// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/abursavich/nett"
)

func Example() {
	dialer := &nett.Dialer{
		// Cache successful DNS lookups for five minutes
		// using DefaultResolver to fill the cache.
		Resolver: &nett.CacheResolver{TTL: 5 * time.Minute},
		// Concurrently dial an IPv4 and an IPv6 address and
		// return the connection that is established first.
		IPFilter: nett.DualStack,
		// Give up after ten seconds including DNS resolution.
		Timeout: 10 * time.Second,
	}
	client := &http.Client{
		Transport: &http.Transport{
			// Use the Dialer.
			Dial: dialer.Dial,
		},
	}
	urls := []string{
		"https://www.google.com/search?q=golang",      // lookup google.com
		"https://www.google.com/search?q=godoc",       // cached google.com
		"https://www.google.com/search?q=golang-nuts", // cached google.com
	}
	for _, url := range urls {
		resp, err := client.Get(url)
		if err != nil {
			panic(err)
		}
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}
