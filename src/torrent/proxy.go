package torrent

import (
	"net"
	"net/http"

	"golang.org/x/net/proxy"
)

func proxyNetDial(dialer proxy.Dialer, network, address string) (net.Conn, error) {
	if dialer != nil {
		return dialer.Dial(network, address)
	}
	return net.Dial(network, address)
}

func proxyHTTPGet(dialer proxy.Dialer, url string) (response *http.Response, e error) {
	return proxyHTTPClient(dialer).Get(url)
}

func proxyHTTPClient(dialer proxy.Dialer) (client *http.Client) {
	if dialer == nil {
		dialer = proxy.Direct
	}
	tr := &http.Transport{Dial: dialer.Dial}
	client = &http.Client{Transport: tr}
	return
}
