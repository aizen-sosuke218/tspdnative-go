package tspdnative

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// httpRequest describes one GET request in the solve pipeline.
type httpRequest struct {
	url     string
	headers http.Header
	cookie  string
	referer string
}

// httpResponse is a fully buffered response.
type httpResponse struct {
	status     int
	body       string
	setCookies []string
	finalURL   string
}

// httpSession owns a keep-alive connection pool for one or more solves.
type httpSession struct {
	client    *http.Client
	transport *http.Transport
}

func newHTTPSession(proxy string) (*httpSession, error) {
	proxyURL, err := parseProxyURL(proxy)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   32,
		MaxConnsPerHost:       32,
		IdleConnTimeout:       300 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	if proxyURL != nil {
		// net/http understands http, https, socks5 and socks5h proxies,
		// including credentials embedded in the URL.
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &httpSession{
		client:    &http.Client{Transport: transport},
		transport: transport,
	}, nil
}

func (s *httpSession) get(ctx context.Context, request httpRequest) (*httpResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header = request.headers.Clone()
	if request.cookie != "" {
		req.Header.Set("Cookie", request.cookie)
	}
	if request.referer != "" {
		req.Header.Set("Referer", request.referer)
	}

	response, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	finalURL := request.url
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}

	return &httpResponse{
		status:     response.StatusCode,
		body:       string(body),
		setCookies: response.Header.Values("Set-Cookie"),
		finalURL:   finalURL,
	}, nil
}

func (s *httpSession) close() {
	s.transport.CloseIdleConnections()
}
