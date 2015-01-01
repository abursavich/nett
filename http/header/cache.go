// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package header

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const httpDate = time.RFC1123

// Date returns the value of the Date header.
func Date(h http.Header) time.Time {
	return ParseTime(h, "Date")
}

// SetDate sets the Date header.
func SetDate(h http.Header, t time.Time) {
	h.Set("Date", t.Format(httpDate))
}

// Expires returns the value of the Expires header.
func Expires(h http.Header) time.Time {
	return ParseTime(h, "Expires")
}

// SetExpires sets the Expires header.
func SetExpires(h http.Header, t time.Time) {
	h.Set("Expires", t.Format(httpDate))
}

type Pragma map[string]struct{}

// SetPragma sets the Pragma header.
func SetPragma(h http.Header, p Pragma) {
	if len(p) == 0 {
		h.Del("Pragma")
	} else {
		h.Set("Pragma", p.String())
	}
}

func ParsePragma(h http.Header) Pragma {
	p := make(Pragma)
	for _, v := range ParseList(h, "Pragma") {
		p[v] = struct{}{}
	}
	return p
}

func (p Pragma) NoCache() bool {
	_, ok := p["no-cache"]
	return ok
}

func (p Pragma) SetNoCache(v bool) {
	if v {
		p["no-cache"] = struct{}{}
	} else {
		delete(p, "no-cache")
	}
}

func (p Pragma) String() string {
	s := ""
	for k := range p {
		s = appendString(s, k)
	}
	return s
}

type CacheControl map[string]string

// SetCacheControl sets the Cache-Control header.
func SetCacheControl(h http.Header, c CacheControl) {
	if len(c) == 0 {
		h.Del("Cache-Control")
	} else {
		h.Set("Cache-Control", c.String())
	}
}

func ParseCacheControl(h http.Header) CacheControl {
	return CacheControl(ParseKVList(h, "Cache-Control", false))
}

func (c CacheControl) Private() ([]string, bool) {
	return c.fields("private")
}

func (c CacheControl) SetPrivate(v ...string) {
	c.setFields("private", v...)
}

func (c CacheControl) Public() bool {
	return c.bool("public")
}

func (c CacheControl) SetPublic(v bool) {
	c.setBool("public", v)
}

func (c CacheControl) MaxAge() (time.Duration, bool) {
	return c.seconds("max-age")
}

func (c CacheControl) SetMaxAge(v time.Duration) {
	c.setSeconds("max-age", v)
}

func (c CacheControl) SharedMaxAge() (time.Duration, bool) {
	return c.seconds("s-maxage")
}

func (c CacheControl) SetSharedMaxAge(v time.Duration) {
	c.setSeconds("s-maxage", v)
}

func (c CacheControl) MaxStale() (time.Duration, bool) {
	return c.seconds("max-stale")
}

func (c CacheControl) SetMaxStale(v time.Duration) {
	c.setSeconds("max-stale", v)
}

func (c CacheControl) MinFresh() (time.Duration, bool) {
	return c.seconds("min-fresh")
}

func (c CacheControl) SetMinFresh(v time.Duration) {
	c.setSeconds("min-fresh", v)
}

func (c CacheControl) MustRevalidate() bool {
	return c.bool("must-revalidate")
}

func (c CacheControl) SetMustRevalidate(v bool) {
	c.setBool("must-revalidate", v)
}

func (c CacheControl) ProxyRevalidate() bool {
	return c.bool("proxy-revalidate")
}

func (c CacheControl) SetProxyRevalidate(v bool) {
	c.setBool("proxy-revalidate", v)
}

func (c CacheControl) NoCache() ([]string, bool) {
	return c.fields("no-cache")
}

func (c CacheControl) SetNoCache(v ...string) {
	c.setFields("no-cache", v...)
}

func (c CacheControl) NoStore() bool {
	return c.bool("no-store")
}

func (c CacheControl) SetNoStore(v bool) {
	c.setBool("no-store", v)
}

func (c CacheControl) NoTransform() bool {
	return c.bool("no-transform")
}

func (c CacheControl) SetNoTransform(v bool) {
	c.setBool("no-transform", v)
}

func (c CacheControl) OnlyIfCached() bool {
	return c.bool("only-if-cached")
}

func (c CacheControl) SetOnlyIfCached(v bool) {
	c.setBool("only-if-cached", v)
}

func (c CacheControl) String() string {
	s := ""
	for k, v := range c {
		if v == "" {
			s = appendString(s, k)
		} else {
			s = appendString(s, k+"="+v)
		}
	}
	return s
}

func (c CacheControl) bool(k string) bool {
	_, ok := c[k]
	return ok
}

func (c CacheControl) setBool(k string, v bool) {
	if v {
		c[k] = ""
	} else {
		delete(c, k)
	}
}

func (c CacheControl) fields(k string) ([]string, bool) {
	s, ok := c[k]
	if s == "" {
		return nil, ok
	}
	return parseTokenList(s[1:]), true
}

func (c CacheControl) setFields(k string, v ...string) {
	if len(v) == 0 {
		delete(c, k)
	} else if len(v) == 1 && v[0] == "" {
		c[k] = ""
	} else {
		s := ""
		for _, i := range v {
			s = appendString(s, i)
		}
		c[k] = fmt.Sprintf("%q", s)
	}
}

func (c CacheControl) seconds(k string) (time.Duration, bool) {
	v, ok := c[k]
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return time.Duration(i) * time.Second, true
}

func (c CacheControl) setSeconds(k string, v time.Duration) {
	if v < 0 {
		delete(c, k)
	} else {
		c[k] = strconv.Itoa(int(v / time.Second))
	}
}

func ParseKVList(h http.Header, key string, unquote bool) map[string]string {
	result := make(map[string]string)
	for _, s := range h[http.CanonicalHeaderKey(key)] {
		for {
			var k, v string
			k, s = expectToken(skipSpace(s))
			if k == "" {
				break
			}
			if strings.HasPrefix(s, "=") {
				v, s = expectTokenOrQuoted(s[1:], unquote)
			}
			result[k] = v
			s = skipSpace(s)
			if !strings.HasPrefix(s, ",") {
				break
			}
			s = s[1:]
		}
	}
	return result
}

func parseTokenList(s string) []string {
	var result []string
	for {
		var v string
		v, s = expectToken(skipSpace(s))
		if v == "" {
			break
		}
		result = append(result, v)
		s = skipSpace(s)
		if !strings.HasPrefix(s, ",") {
			break
		}
		s = s[1:]
	}
	return result
}

func appendString(s, v string) string {
	if s == "" {
		return v
	}
	return s + ", " + v
}
