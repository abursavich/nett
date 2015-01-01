// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package header

import (
	"net/http"
	"reflect"
	"testing"
)

func TestParsePragma(t *testing.T) {
	yes := struct{}{}
	tests := []struct {
		str string
		exp Pragma
	}{
		{"", Pragma{}},
		{"no-cache", Pragma{"no-cache": yes}},
		{"no-cache, extension", Pragma{"no-cache": yes, "extension": yes}},
		{"no-cache  ,  \"quoted\"", Pragma{"no-cache": yes, "\"quoted\"": yes}},
	}
	for _, tt := range tests {
		h := http.Header{"Pragma": {tt.str}}
		if got := ParsePragma(h); !reflect.DeepEqual(tt.exp, got) {
			t.Errorf("ParsePragma for %q = %q, want %q", tt.str, got, tt.exp)
		}
	}
}

func TestParseKVList(t *testing.T) {
	tests := []struct {
		str string
		esc bool
		exp map[string]string
	}{
		{"", false, map[string]string{}},
		{"foo", false, map[string]string{"foo": ""}},
		{"foo=bar", false, map[string]string{"foo": "bar"}},
		{`foo="bar\"baz\"qux", bar=baz`, false, map[string]string{"foo": `"bar\"baz\"qux"`, "bar": "baz"}},
		{`foo="bar\"baz\"qux", bar=baz`, true, map[string]string{"foo": `bar"baz"qux`, "bar": "baz"}},
		{"foo=bar, bar=baz", false, map[string]string{"foo": "bar", "bar": "baz"}},
		{"  foo=bar  ,   bar=baz  ", false, map[string]string{"foo": "bar", "bar": "baz"}},
	}
	for _, tt := range tests {
		h := http.Header{"Foo": {tt.str}}
		if got := ParseKVList(h, "Foo", tt.esc); !reflect.DeepEqual(tt.exp, got) {
			t.Errorf("ParseKVList for %q = %q, want %q", tt.str, got, tt.exp)
		}
	}
}

func TestParseCacheControl(t *testing.T) {
	tests := []struct {
		str string
		exp CacheControl
	}{
		{"", CacheControl{}},
		{"no-cache", CacheControl{"no-cache": ""}},
		{"max-age=0, no-cache", CacheControl{"max-age": "0", "no-cache": ""}},
		{"no-cache=\"Set-Cookie\", foobar, foo=bar",
			CacheControl{"no-cache": "\"Set-Cookie\"", "foobar": "", "foo": "bar"}},
	}
	for _, tt := range tests {
		h := http.Header{"Cache-Control": {tt.str}}
		if got := ParseCacheControl(h); !reflect.DeepEqual(tt.exp, got) {
			t.Errorf("CacheControl for %q = %q, want %q", tt.str, got, tt.exp)
		}
	}
}
