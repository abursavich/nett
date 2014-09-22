nett
====

Package nett steals from the standard library's net package and attempts to provide additional useful features.

**EXPERIMENTAL**: API subject to change.

[![GoDoc](https://godoc.org/github.com/abursavich/nett?status.svg)](https://godoc.org/github.com/abursavich/nett) [![Build Status](https://travis-ci.org/abursavich/nett.svg?branch=master)](https://travis-ci.org/abursavich/nett)

```Go
dialer := nett.Dialer{
    // Cache successful DNS lookups for five minutes
    // using nett.DefaultResolver to fill the cache.
    Resolver: nett.NewCacheResolver(nil, 5*time.Minute),
    // If host resolves to multiple IP addresses,
    // dial two concurrently splitting between
    // IPv4 and IPv6 addresses and return the
    // connection that is established first.
    Filter: nett.MaxFilter(2),
    // Give up on dial after five seconds including
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
```
