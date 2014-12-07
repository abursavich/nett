# nett [![GoDoc](https://godoc.org/github.com/abursavich/nett?status.svg)](https://godoc.org/github.com/abursavich/nett) [![Build Status](https://travis-ci.org/abursavich/nett.svg?branch=master)](https://travis-ci.org/abursavich/nett)
    import "github.com/abursavich/nett"

Package nett steals from the standard library's net package
and attempts to provide additional useful features. The primary
motivation was to provide a dialer with a pluggable host resolver.

### EXPERIMENTAL
There are no plans to break the API, but it should be considered unstable
for the time being.

Example:

``` go
dialer := &nett.Dialer{
    // Cache successful DNS lookups for five minutes
    // using nett.DefaultResolver to fill the cache.
    Resolver: nett.NewCacheResolver(nil, 5*time.Minute),
    // If host resolves to multiple IP addresses,
    // dial two concurrently splitting between
    // IPv4 and IPv6 addresses and return the
    // connection that is established first.
    IPFilter: nett.MaxIPFilter(2),
    // Give up after 5 seconds including DNS resolution.
    Timeout: 5 * time.Second,
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



## func DefaultIPFilter
``` go
func DefaultIPFilter(ips []net.IP) []net.IP
```
DefaultIPFilter selects the first IPv4 address in ips.
If only IPv6 addresses exist in ips, then it selects
the first IPv6 address.


## func DualStackIPFilter
``` go
func DualStackIPFilter(ips []net.IP) []net.IP
```
DualStackIPFilter selects the first IPv4 address
and IPv6 address in ips.


## func NoIPFilter
``` go
func NoIPFilter(ips []net.IP) []net.IP
```
NoIPFilter selects all IP addresses.


## func ResolveIPAddrs
``` go
func ResolveIPAddrs(resolver Resolver, filter IPFilter, network, address string) ([]*net.IPAddr, error)
```
ResolveIPAddrs parses address of the form "host" or "ipv6-host%zone" and
resolves a list of IP addresses on the network, which must be "ip", "ip4"
or "ip6".

If host is a domain name, resolver is used to resolve a list of platform
supported IP addresses. If resolver is nil, DefaultResolver is used.

If filter is non-nil, resolved IP addresses are selected by applying it.


## func ResolveTCPAddrs
``` go
func ResolveTCPAddrs(resolver Resolver, filter IPFilter, network, address string) ([]*net.TCPAddr, error)
```
ResolveTCPAddrs parses address of the form "host:port" or
"[ipv6-host%zone]:port" and resolves a list of TCP addresses on the
network, which must be "tcp", "tcp4" or "tcp6". A literal address or
host name for IPv6 must be enclosed in square brackets, as in "[::1]:80",
"[ipv6-host]:http" or "[ipv6-host%zone]:80".

If host is a domain name, resolver is used to resolve a list of platform
supported IP addresses. If resolver is nil, DefaultResolver is used.

If filter is non-nil, resolved IP addresses are selected by applying it.


## func ResolveUDPAddrs
``` go
func ResolveUDPAddrs(resolver Resolver, filter IPFilter, network, address string) ([]*net.UDPAddr, error)
```
ResolveUDPAddrs parses address of the form "host:port" or
"[ipv6-host%zone]:port" and resolves a list of UDP addresses on the
network, which must be "udp", "udp4" or "udp6". A literal address or
host name for IPv6 must be enclosed in square brackets, as in "[::1]:80",
"[ipv6-host]:http" or "[ipv6-host%zone]:80".

If host is a domain name, resolver is used to resolve a list of platform
supported IP addresses. If resolver is nil, DefaultResolver is used.

If filter is non-nil, resolved IP addresses are selected by applying it.


## func SupportedIP
``` go
func SupportedIP(ip net.IP) net.IP
```
SupportedIP returns a version of the IP that the platform
supports. If it is not supported it returns nil.



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
CacheResolver looks up the IP addresses of a host and caches results.









### func NewCacheResolver
``` go
func NewCacheResolver(resolver Resolver, ttl time.Duration) *CacheResolver
```
NewCacheResolver returns a new CacheResolver with the given
resolver and ttl.




### func (\*CacheResolver) Resolve
``` go
func (r *CacheResolver) Resolve(host string) ([]net.IP, error)
```
Resolve looks up the IP addresses of a host in its cache.
If not found in its cache, the CacheResolver's Resolver is
used to look up the hosts IPs. Successful results are added
to the cache.







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
    // If nil, DefaultIPFilter is used.
    IPFilter IPFilter

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



## type IPFilter
``` go
type IPFilter func(ips []net.IP) []net.IP
```
IPFilter selects IP addresses from ips.









### func MaxIPFilter
``` go
func MaxIPFilter(max int) IPFilter
```
MaxIPFilter returns a IPFilter that selects up to max addresses.
It will split the results evenly between availabe IPv4 and
IPv6 addresses. If one type of address doesn't exist in
sufficient quantity to consume its share, the other type
will be allowed to fill any extra space in the result.
Addresses toward the front of the collection are preferred.




## type Resolver
``` go
type Resolver interface {
    // Resolve returns a slice of host's IPv4 and IPv6 addresses.
    Resolve(domain string) ([]net.IP, error)
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
