// Copyright 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.
//
// This file was originally copied from the
// github.com/golang/gddo/httputil/header package
// and some changes were made.

package header

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestGetHeaderList(t *testing.T) {
	var tests = []struct {
		s string
		l []string
	}{
		{s: `a`, l: []string{`a`}},
		{s: `a, b , c `, l: []string{`a`, `b`, `c`}},
		{s: `a,, b , , c `, l: []string{`a`, `b`, `c`}},
		{s: `a,b,c`, l: []string{`a`, `b`, `c`}},
		{s: ` a b, c d `, l: []string{`a b`, `c d`}},
		{s: `"a, b, c", d `, l: []string{`"a, b, c"`, "d"}},
		{s: `","`, l: []string{`","`}},
		{s: `"\""`, l: []string{`"\""`}},
		{s: `" "`, l: []string{`" "`}},
	}
	for _, tt := range tests {
		header := http.Header{"Foo": {tt.s}}
		if l := ParseList(header, "foo"); !reflect.DeepEqual(tt.l, l) {
			t.Errorf("ParseList for %q = %q, want %q", tt.s, l, tt.l)
		}
	}
}

func TestParseValueAndParams(t *testing.T) {
	var tests = []struct {
		s      string
		value  string
		params map[string]string
	}{
		{`text/html`, "text/html", map[string]string{}},
		{`text/html  `, "text/html", map[string]string{}},
		{`text/html ; `, "text/html", map[string]string{}},
		{`tExt/htMl`, "text/html", map[string]string{}},
		{`tExt/htMl; fOO=";"; hellO=world`, "text/html", map[string]string{
			"hello": "world",
			"foo":   `;`,
		}},
		{`text/html; foo=bar, hello=world`, "text/html", map[string]string{"foo": "bar"}},
		{`text/html ; foo=bar `, "text/html", map[string]string{"foo": "bar"}},
		{`text/html ;foo=bar `, "text/html", map[string]string{"foo": "bar"}},
		{`text/html; foo="b\ar"`, "text/html", map[string]string{"foo": "bar"}},
		{`text/html; foo="bar\"baz\"qux"`, "text/html", map[string]string{"foo": `bar"baz"qux`}},
		{`text/html; foo="b,ar"`, "text/html", map[string]string{"foo": "b,ar"}},
		{`text/html; foo="b;ar"`, "text/html", map[string]string{"foo": "b;ar"}},
		{`text/html; FOO="bar"`, "text/html", map[string]string{"foo": "bar"}},
		{`form-data; filename="file.txt"; name=file`, "form-data", map[string]string{"filename": "file.txt", "name": "file"}},
	}
	for _, tt := range tests {
		header := http.Header{"Content-Type": {tt.s}}
		value, params := ParseValueAndParams(header, "Content-Type")
		if value != tt.value {
			t.Errorf("%q, value=%q, want %q", tt.s, value, tt.value)
		}
		if !reflect.DeepEqual(params, tt.params) {
			t.Errorf("%q, param=%#v, want %#v", tt.s, params, tt.params)
		}
	}
}

func TestParseTime(t *testing.T) {
	var tests = []string{
		"Sun, 06 Nov 1994 08:49:37 GMT",
		"Sunday, 06-Nov-94 08:49:37 GMT",
		"Sun Nov  6 08:49:37 1994",
	}
	expected := time.Date(1994, 11, 6, 8, 49, 37, 0, time.UTC)
	for _, s := range tests {
		header := http.Header{"Date": {s}}
		actual := ParseTime(header, "Date")
		if actual != expected {
			t.Errorf("GetTime(%q)=%v, want %v", s, actual, expected)
		}
	}
	tests = []string{
		"junk",
	}
	for _, s := range tests {
		header := http.Header{"Date": {s}}
		actual := ParseTime(header, "Date")
		if !actual.IsZero() {
			t.Errorf("GetTime(%q) did not return zero", s)
		}
	}
}

func TestParseAccept(t *testing.T) {
	var tests = []struct {
		s        string
		expected []AcceptSpec
	}{
		{"text/html", []AcceptSpec{{"text/html", 1}}},
		{"text/html; q=0", []AcceptSpec{{"text/html", 0}}},
		{"text/html; q=0.0", []AcceptSpec{{"text/html", 0}}},
		{"text/html; q=1", []AcceptSpec{{"text/html", 1}}},
		{"text/html; q=1.0", []AcceptSpec{{"text/html", 1}}},
		{"text/html; q=0.1", []AcceptSpec{{"text/html", 0.1}}},
		{"text/html;q=0.1", []AcceptSpec{{"text/html", 0.1}}},
		{"text/html, text/plain", []AcceptSpec{{"text/html", 1}, {"text/plain", 1}}},
		{"text/html; q=0.1, text/plain", []AcceptSpec{{"text/html", 0.1}, {"text/plain", 1}}},
		{"iso-8859-5, unicode-1-1;q=0.8,iso-8859-1", []AcceptSpec{{"iso-8859-5", 1}, {"unicode-1-1", 0.8}, {"iso-8859-1", 1}}},
		{"iso-8859-1", []AcceptSpec{{"iso-8859-1", 1}}},
		{"*", []AcceptSpec{{"*", 1}}},
		{"da, en-gb;q=0.8, en;q=0.7", []AcceptSpec{{"da", 1}, {"en-gb", 0.8}, {"en", 0.7}}},
		{"da, q, en-gb;q=0.8", []AcceptSpec{{"da", 1}, {"q", 1}, {"en-gb", 0.8}}},
		{"image/png, image/*;q=0.5", []AcceptSpec{{"image/png", 1}, {"image/*", 0.5}}},

		// bad cases
		{"value1; q=0.1.2", []AcceptSpec{{"value1", 0.1}}},
		{"da, en-gb;q=foo", []AcceptSpec{{"da", 1}}},
	}
	for _, tt := range tests {
		header := http.Header{"Accept": {tt.s}}
		actual := ParseAccept(header, "Accept")
		if !reflect.DeepEqual(actual, tt.expected) {
			t.Errorf("ParseAccept(h, %q)=%v, want %v", tt.s, actual, tt.expected)
		}
	}
}
