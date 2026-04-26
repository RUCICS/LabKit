package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"labkit.local/apps/cli/internal/buildinfo"
)

type Client struct {
	baseURL *url.URL
	http    *http.Client
	headers http.Header
}

func New(baseURL string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base URL must include scheme and host")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	h := make(http.Header)
	h.Set("User-Agent", buildinfo.UserAgent("labkit"))
	if v := strings.TrimSpace(buildinfo.NormalizedVersion()); v != "" {
		h.Set("X-LabKit-Client-Version", v)
	}
	if code := buildinfo.VersionCode(); code > 0 {
		h.Set("X-LabKit-Client-Version-Code", strconv.Itoa(code))
	}
	if commit := strings.TrimSpace(buildinfo.Commit); commit != "" && commit != "unknown" {
		h.Set("X-LabKit-Client-Commit", commit)
	}
	return &Client{baseURL: parsed, http: httpClient, headers: h}, nil
}

func (c *Client) NewRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	target := c.baseURL.ResolveReference(ref)
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range c.headers {
		for _, v := range vs {
			if req.Header.Get(k) == "" {
				req.Header.Add(k, v)
			}
		}
	}
	return req, nil
}

func (c *Client) NewRequestWithBytes(ctx context.Context, method, path string, body []byte, contentType string) (*http.Request, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	target := c.baseURL.ResolveReference(ref)
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, vs := range c.headers {
		for _, v := range vs {
			if req.Header.Get(k) == "" {
				req.Header.Add(k, v)
			}
		}
	}
	return req, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.http.Do(req)
}
