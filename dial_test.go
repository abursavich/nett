// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDialHTTP(t *testing.T) {
	b := []byte{'O', 'K'}
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

func Example() {
	dialer := Dialer{
		// Cache successful DNS lookups for five minutes
		// using DefaultResolver to fill the cache.
		Resolver: NewCacheResolver(nil, 5*time.Minute),
		// If host resolves to multiple IP addresses,
		// dial two concurrently splitting between
		// IPv4 and IPv6 addresses and return the
		// connection that is established first.
		Filter: MaxFilter(2),
		// Give up on dial after 5 seconds including
		// DNS resolution.
		Timeout: 5 * time.Second,
	}
	client := http.Client{
		Transport: &http.Transport{
			// Use the Dialer.
			Dial: dialer.Dial,
		},
	}
	urls := []string{
		"https://www.google.com/search?q=golang",
		"https://www.google.com/search?q=godoc",
		"https://www.google.com/search?q=golang-nuts",
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
