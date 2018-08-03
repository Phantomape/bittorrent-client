package network

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/perf"
	"golang.org/x/net/proxy"
)

// Maybe use net.Dialer instead?

type firewallCallback func(net.Addr) bool

type networkAndHost struct {
	Network string
	Host    string
}

type dialer interface {
	dial(_ context.Context, addr string) (net.Conn, error)
}

// Socket : socket
type Socket interface {
	net.Listener
	dialer
}

type tcpSocket struct {
	net.Listener
	d func(ctx context.Context, addr string) (net.Conn, error)
}

func (me tcpSocket) dial(ctx context.Context, addr string) (net.Conn, error) {
	return me.d(ctx, addr)
}

// ListenAll : listen all networks
func ListenAll(networks []string, getHost func(string) string, port int, proxyURL string, f firewallCallback) ([]Socket, error) {
	if len(networks) == 0 {
		return nil, nil
	}

	var nahs []networkAndHost
	for _, n := range networks {
		nahs = append(nahs, networkAndHost{n, getHost(n)})
	}

	for {
		ss, retry, err := listenAllRetry(nahs, port, proxyURL, f)
		if !retry {
			return ss, err
		}
	}
}

func listenAllRetry(nahs []networkAndHost, port int, proxyURL string, f firewallCallback) (ss []Socket, retry bool, err error) {
	ss = make([]Socket, 1, len(nahs))
	portStr := strconv.FormatInt(int64(port), 10)
	ss[0], err = listen(nahs[0].Network, net.JoinHostPort(nahs[0].Host, portStr), proxyURL, f)
	if err != nil {
		return nil, false, fmt.Errorf("first listen: %s", err)
	}

	defer func() {
		if err != nil || retry {
			for _, s := range ss {
				s.Close()
			}
			ss = nil
		}
	}()

	// What is the difference btw the first and the rest?
	portStr = strconv.FormatInt(int64(missinggo.AddrPort(ss[0].Addr())), 10)
	for _, nah := range nahs[1:] {
		s, err := listen(nah.Network, net.JoinHostPort(nah.Host, portStr), proxyURL, f)
		if err != nil {
			return ss, missinggo.IsAddrInUse(err) && port == 0, fmt.Errorf("subsequent listen: %s", err)
		}
		ss = append(ss, s)
	}
	return
}

func listen(network, addr, proxyURL string, f firewallCallback) (Socket, error) {
	if isTCPNetwork(network) {
		return listenTCP(network, addr, proxyURL)
	}
	panic(fmt.Sprintf("unknown network %q", network))
}

func isTCPNetwork(s string) bool {
	return strings.Contains(s, "tcp")
}

func listenTCP(network, address, proxyURL string) (s Socket, err error) {
	// The listen function creates servers
	l, err := net.Listen(network, address)
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			l.Close()
		}
	}()

	// If we don't need the proxy - then we should return default net.Dialer,
	// otherwise, let's try to parse the proxyURL and return proxy.Dialer
	if len(proxyURL) != 0 {
		// TODO: The error should be propagated, as proxy may be in use for
		// security or privacy reasons. Also just pass proxy.Dialer in from
		// the Config.
		if dialer, err := getProxyDialer(proxyURL); err == nil {
			return tcpSocket{l, func(ctx context.Context, addr string) (conn net.Conn, err error) {
				defer perf.ScopeTimerErr(&err)()
				return dialer.Dial(network, addr)
			}}, nil
		}
	}

	dialer := net.Dialer{}
	return tcpSocket{l, func(ctx context.Context, addr string) (conn net.Conn, err error) {
		defer perf.ScopeTimerErr(&err)() // what is this
		return dialer.DialContext(ctx, network, addr)
	}}, nil
}

func getProxyDialer(proxyURL string) (proxy.Dialer, error) {
	fixedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	return proxy.FromURL(fixedURL, proxy.Direct)
}