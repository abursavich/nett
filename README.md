# nett [![GoDoc](https://godoc.org/github.com/abursavich/nett?status.svg)](https://godoc.org/github.com/abursavich/nett) [![Build Status](https://travis-ci.org/abursavich/nett.svg?branch=master)](https://travis-ci.org/abursavich/nett)
    import "github.com/abursavich/nett"

Package nett steals from the standard library's net package
and provides a dialer with a pluggable host resolver.


### Example:

``` go
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



## func DualStack
``` go
func DualStack(ips []net.IP) []net.IP
```
DualStack selects the first IPv4 address
and IPv6 address in ips.



## type CacheResolver
``` go
type CacheResolver struct {
    // Resolver resolves hosts that are not cached.
    // If Resolver is nil, DefaultResolver will be used.
    Resolver Resolver
    // TTL is the time to live for resolved hosts.
    // If TTL is zero, cached hosts do not expire.
    TTL time.Duration
    // contains filtered or unexported fields
}
```
CacheResolver looks up the IP addresses of a host
and caches successful results.











### func (\*CacheResolver) Resolve
``` go
func (r *CacheResolver) Resolve(host string) ([]net.IP, error)
```
Resolve returns a host's IP addresses.



## type Dialer
``` go
type Dialer struct {
    // Timeout is the maximum amount of time a dial will wait for
    // a connect to complete. If Deadline is also set, it may fail
    // earlier.
    //
    // The default is no timeout.
    //
    // With or without a timeout, the operating system may impose
    // its own earlier timeout. For instance, TCP timeouts are
    // often around 3 minutes.
    Timeout time.Duration

    // Deadline is the absolute point in time after which dials
    // will fail. If Timeout is set, it may fail earlier.
    //
    // Zero means no deadline, or dependent on the operating system
    // as with the Timeout option.
    Deadline time.Time

    // LocalAddr is the local address to use when dialing an
    // address. The address must be of a compatible type for the
    // network being dialed.
    //
    // If nil, a local address is automatically chosen.
    LocalAddr net.Addr

    // Resolver is used to resolve IP addresses from domain names.
    //
    // If nil, DefaultResolver will be used.
    Resolver Resolver

    // IPFilter selects addresses from those available after
    // resolving a host to a set of supported IPs.
    //
    // When dialing a TCP connection if multiple addresses are
    // returned, then a connection will attempt to be established
    // with each address and the first that succeeds will be returned.
    // With any other type of connection, only the first address
    // returned will be dialed.
    //
    // If nil, a single address is selected.
    IPFilter func(ips []net.IP) []net.IP

    // KeepAlive specifies the keep-alive period for an active
    // network connection.
    //
    // If zero, keep-alives are not enabled. Network protocols
    // that do not support keep-alives ignore this field.
    //
    // Only functional for go1.3+.
    KeepAlive time.Duration
}
```
A Dialer contains options for connecting to an address.











### func (\*Dialer) Dial
``` go
func (d *Dialer) Dial(network, address string) (net.Conn, error)
```
Dial connects to the address on the named network.

Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only),
"udp", "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4"
(IPv4-only), "ip6" (IPv6-only), "unix", "unixgram" and
"unixpacket".

For TCP and UDP networks, addresses have the form host:port.
If host is a literal IPv6 address it must be enclosed
in square brackets as in "[::1]:80" or "[ipv6-host%zone]:80".
The functions JoinHostPort and SplitHostPort manipulate addresses
in this form.

Examples:


    Dial("tcp", "12.34.56.78:80")
    Dial("tcp", "google.com:http")
    Dial("tcp", "[2001:db8::1]:http")
    Dial("tcp", "[fe80::1%lo0]:80")

For IP networks, the network must be "ip", "ip4" or "ip6" followed
by a colon and a protocol number or name and the addr must be a
literal IP address.

Examples:


    Dial("ip4:1", "127.0.0.1")
    Dial("ip6:ospf", "::1")

For Unix networks, the address must be a file system path.











## type Resolver
``` go
type Resolver interface {
    // Resolve looks up the given host and returns its IP addresses.
    Resolve(host string) ([]net.IP, error)
}
```
Resolver is an interface representing the ability to lookup the
IP addresses of a host. It may return results containing networks
not supported by the platform.

A Resolver must be safe for concurrent use by multiple goroutines.





``` go
var DefaultResolver Resolver = defaultResolver{}
```
DefaultResolver is the default Resolver.













- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
