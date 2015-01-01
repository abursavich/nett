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
	"strings"
	"time"
)

// Octet types from RFC 2616.
var octetTypes [256]octetType

type octetType byte

const (
	octetToken octetType = 1 << iota
	octetSpace
)

func isToken(b byte) bool {
	return octetTypes[b]&octetToken != 0
}

func isSpace(b byte) bool {
	return octetTypes[b]&octetSpace != 0
}

func init() {
	// OCTET      = <any 8-bit sequence of data>
	// CHAR       = <any US-ASCII character (octets 0 - 127)>
	// CTL        = <any US-ASCII control character (octets 0 - 31) and DEL (127)>
	// CR         = <US-ASCII CR, carriage return (13)>
	// LF         = <US-ASCII LF, linefeed (10)>
	// SP         = <US-ASCII SP, space (32)>
	// HT         = <US-ASCII HT, horizontal-tab (9)>
	// <">        = <US-ASCII double-quote mark (34)>
	// CRLF       = CR LF
	// LWS        = [CRLF] 1*( SP | HT )
	// TEXT       = <any OCTET except CTLs, but including LWS>
	// separators = "(" | ")" | "<" | ">" | "@" | "," | ";" | ":" | "\" | <">
	//              | "/" | "[" | "]" | "?" | "=" | "{" | "}" | SP | HT
	// token      = 1*<any CHAR except CTLs or separators>
	// qdtext     = <any TEXT except <">>

	for c := 0; c < 256; c++ {
		var t octetType
		isCtl := c <= 31 || c == 127
		isChar := 0 <= c && c <= 127
		isSeparator := strings.IndexRune(" \t\"(),/:;<=>?@[]\\{}", rune(c)) >= 0
		if strings.IndexRune(" \t\r\n", rune(c)) >= 0 {
			t |= octetSpace
		}
		if isChar && !isCtl && !isSeparator {
			t |= octetToken
		}
		octetTypes[c] = t
	}
}

// Copy returns a shallow copy of the header.
func Copy(h http.Header) http.Header {
	c := make(http.Header)
	for k, vs := range h {
		c[k] = vs
	}
	return c
}

var timeLayouts = []string{"Mon, 02 Jan 2006 15:04:05 GMT", time.RFC850, time.ANSIC}

// ParseTime parses the header as time. The zero value is returned if the
// header is not present or there is an error parsing the
// header.
func ParseTime(h http.Header, key string) time.Time {
	if s := h.Get(key); s != "" {
		for _, layout := range timeLayouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t.UTC()
			}
		}
	}
	return time.Time{}
}

// ParseList parses a comma separated list of values. Commas are ignored in
// quoted strings. Quoted values are not unescaped or unquoted. Whitespace is
// trimmed.
func ParseList(h http.Header, key string) []string {
	var result []string
	for _, s := range h[http.CanonicalHeaderKey(key)] {
		begin := 0
		end := 0
		escape := false
		quote := false
		for i := 0; i < len(s); i++ {
			b := s[i]
			switch {
			case escape:
				escape = false
				end = i + 1
			case quote:
				switch b {
				case '\\':
					escape = true
				case '"':
					quote = false
				}
				end = i + 1
			case b == '"':
				quote = true
				end = i + 1
			case isSpace(b):
				if begin == end {
					begin = i + 1
					end = begin
				}
			case b == ',':
				if begin < end {
					result = append(result, s[begin:end])
				}
				begin = i + 1
				end = begin
			default:
				end = i + 1
			}
		}
		if begin < end {
			result = append(result, s[begin:end])
		}
	}
	return result
}

// ParseValueAndParams parses a comma separated list of values with optional
// semicolon separated name-value pairs. Content-Type and Content-Disposition
// headers are in this format.
func ParseValueAndParams(h http.Header, key string) (value string, params map[string]string) {
	params = make(map[string]string)
	s := h.Get(key)
	value, s = expectTokenSlash(s)
	if value == "" {
		return
	}
	value = strings.ToLower(value)
	s = skipSpace(s)
	for strings.HasPrefix(s, ";") {
		var pkey string
		pkey, s = expectToken(skipSpace(s[1:]))
		if pkey == "" {
			return
		}
		if !strings.HasPrefix(s, "=") {
			return
		}
		var pvalue string
		pvalue, s = expectTokenOrQuoted(s[1:], true)
		if pvalue == "" {
			return
		}
		pkey = strings.ToLower(pkey)
		params[pkey] = pvalue
		s = skipSpace(s)
	}
	return
}

type AcceptSpec struct {
	Value string
	Q     float64
}

// ParseAccept parses Accept* headers.
func ParseAccept(h http.Header, key string) (specs []AcceptSpec) {
loop:
	for _, s := range h[key] {
		for {
			var spec AcceptSpec
			spec.Value, s = expectTokenSlash(s)
			if spec.Value == "" {
				continue loop
			}
			spec.Q = 1.0
			s = skipSpace(s)
			if strings.HasPrefix(s, ";") {
				s = skipSpace(s[1:])
				if !strings.HasPrefix(s, "q=") {
					continue loop
				}
				spec.Q, s = expectQuality(s[2:])
				if spec.Q < 0.0 {
					continue loop
				}
			}
			specs = append(specs, spec)
			s = skipSpace(s)
			if !strings.HasPrefix(s, ",") {
				continue loop
			}
			s = skipSpace(s[1:])
		}
	}
	return
}

func skipSpace(s string) (rest string) {
	for i := 0; i < len(s); i++ {
		if !isSpace(s[i]) {
			return s[i:]
		}
	}
	return ""
}

func expectToken(s string) (token, rest string) {
	for i := 0; i < len(s); i++ {
		if !isToken(s[i]) {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func expectTokenSlash(s string) (token, rest string) {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !isToken(b) && b != '/' {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func expectQuality(s string) (q float64, rest string) {
	switch {
	case len(s) == 0:
		return -1, ""
	case s[0] == '0':
		q = 0
	case s[0] == '1':
		q = 1
	default:
		return -1, ""
	}
	s = s[1:]
	if !strings.HasPrefix(s, ".") {
		return q, s
	}
	s = s[1:]
	i := 0
	n := 0
	d := 1
	for ; i < len(s); i++ {
		b := s[i]
		if b < '0' || b > '9' {
			break
		}
		n = n*10 + int(b) - '0'
		d *= 10
	}
	return q + float64(n)/float64(d), s[i:]
}

func expectTokenOrQuoted(s string, unquote bool) (value string, rest string) {
	if strings.HasPrefix(s, "\"") {
		return expectQuoted(s, unquote)
	}
	return expectToken(s)
}

func expectQuoted(s string, unquote bool) (value string, rest string) {
	if !strings.HasPrefix(s, "\"") {
		return "", s
	}
	if unquote {
		orig := s
		s = s[1:]
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '"':
				return s[:i], s[i+1:]
			case '\\':
				p := make([]byte, len(s)-1)
				j := copy(p, s[:i])
				escape := true
				for i = i + 1; i < len(s); i++ {
					b := s[i]
					switch {
					case escape:
						escape = false
						p[j] = b
						j += 1
					case b == '\\':
						escape = true
					case b == '"':
						return string(p[:j]), s[i+1:]
					default:
						p[j] = b
						j += 1
					}
				}
				return "", orig
			}
		}
		return "", orig
	}
	escape := false
	for i := 1; i < len(s); i++ {
		b := s[i]
		switch {
		case escape:
			escape = false
		case b == '\\':
			escape = true
		case b == '"':
			return s[:i+1], s[i+1:]
		}
	}
	return "", s
}
