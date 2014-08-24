// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
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
