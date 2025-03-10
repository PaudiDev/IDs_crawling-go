package httpx

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"crawler/app/pkg/assert"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

func setHeaders(req *http.Request, headers map[string]string) {
	assert.NotNil(req, "nil pointer to request must never happen")
	assert.Assert(len(headers) > 0, "empty headers map")

	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func BuildRequest(
	ctx context.Context,
	method string,
	url string,
	body io.Reader,
	headers map[string]string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	setHeaders(req, headers)

	return req, nil
}

// TODO: add docs (specify that this only works with HTTP2 if HTTPS)
func MakeRequestWithProxyAndFingerprint(
	req *http.Request,
	cookieJar http.CookieJar,
	proxyUrl *url.URL,
	utlsProfile *utls.ClientHelloID,
	timeout int,
) (*http.Response, error) {
	var transport http.RoundTripper

	if req.URL.Scheme == "https" {
		transport = &http2.Transport{
			DialTLSContext: func(ctx context.Context, network string, addr string, cfg *tls.Config) (net.Conn, error) {
				proxyAuth := ""
				if proxyUrl.User != nil {
					password, _ := proxyUrl.User.Password()
					proxyAuth = fmt.Sprintf("%s:%s", proxyUrl.User.Username(), password)
				}

				return dialWithUTLS(addr, proxyUrl.Host, proxyAuth, utlsProfile)
			},
		}
	} else {
		transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
	}

	client := &http.Client{
		Transport: transport,
		Jar:       cookieJar,
		Timeout:   (time.Duration)(timeout) * time.Second,
	}

	return client.Do(req)
}

func dialWithUTLS(targetAddr string, proxyHost string, proxyAuth string, utlsProfile *utls.ClientHelloID) (*utls.UConn, error) {
	// Open a low level raw TCP connection to the proxy server
	// All data sent through this connection will be routed through the proxy
	conn, err := net.DialTimeout("tcp", proxyHost, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to open TCP connection to proxy: %v", err)
	}

	// fmt.Fprintf writes to the network buffer, which will be flushed to the proxy server
	// (usually almost instantly)
	//
	// Use the CONNECT method to instruct the proxy to open a direct raw TCP connection to the target
	// Without this step, the proxy would see all data encrypted and would not be able to route it
	// to the target server
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr)
	if proxyAuth != "" {
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(proxyAuth))
		fmt.Fprintf(conn, "Proxy-Authorization: Basic %s\r\n", encodedAuth)
	}
	fmt.Fprint(conn, "\r\n") // CRLF

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil || resp.StatusCode != 200 {
		conn.Close()
		return nil, fmt.Errorf("failed to open TCP connection between proxy and target: %v", err)
	}

	// Setup the connection to use the uTLS profile fingerprint
	//
	// Remove the port from targetAddr to use it as ServerName
	tlsConfig := &utls.Config{ServerName: strings.Split(targetAddr, ":")[0]}
	utlsConn := utls.UClient(conn, tlsConfig, *utlsProfile)

	// Perform the TLS handshake
	if err := utlsConn.Handshake(); err != nil {
		utlsConn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %v", err)
	}

	return utlsConn, nil
}
