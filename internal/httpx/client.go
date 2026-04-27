package httpx

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

type ClientOptions struct {
	Timeout            time.Duration
	InsecureSkipVerify bool
}

func NewClient(opts ClientOptions) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxConnsPerHost:       0,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.InsecureSkipVerify,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}
