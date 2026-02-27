package proofcheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type NetHTTPFetcher struct {
	client *http.Client
}

func NewNetHTTPFetcher() *NetHTTPFetcher {
	return &NetHTTPFetcher{
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (f *NetHTTPFetcher) Get(ctx context.Context, rawURL string) (status int, body []byte, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, nil, fmt.Errorf("parse url: %w", err)
	}
	if parsed.Scheme != "https" {
		return 0, nil, fmt.Errorf("only https is allowed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "audicatalog-worker/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	reader := io.LimitReader(resp.Body, 32*1024)
	data, err := io.ReadAll(reader)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read body: %w", err)
	}

	return resp.StatusCode, data, nil
}
