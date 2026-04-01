package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"labkit.local/apps/api/internal/config"
)

// OAuthHTTPClient is the production OAuth client used by the API service.
type OAuthHTTPClient struct {
	cfg        config.OAuthConfig
	httpClient *http.Client
}

func NewOAuthHTTPClient(cfg config.OAuthConfig) *OAuthHTTPClient {
	return &OAuthHTTPClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *OAuthHTTPClient) ExchangeCode(ctx context.Context, code string) (string, error) {
	values := url.Values{}
	values.Set("code", code)
	values.Set("client_id", c.cfg.ClientID)
	values.Set("client_secret", c.cfg.ClientSecret)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", c.cfg.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("oauth token exchange failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	token, err := parseAccessToken(body)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (c *OAuthHTTPClient) FetchProfile(ctx context.Context, accessToken string) (OAuthProfile, error) {
	endpoint, err := url.Parse(c.cfg.ProfileURL)
	if err != nil {
		return OAuthProfile{}, err
	}
	query := endpoint.Query()
	query.Set("access_token", accessToken)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return OAuthProfile{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OAuthProfile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return OAuthProfile{}, fmt.Errorf("oauth profile fetch failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		LoginName string `json:"loginName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return OAuthProfile{}, err
	}
	if strings.TrimSpace(payload.LoginName) == "" {
		return OAuthProfile{}, errors.New("oauth profile missing loginName")
	}
	return OAuthProfile{LoginName: payload.LoginName}, nil
}

func parseAccessToken(body []byte) (string, error) {
	var payload struct {
		AccessToken      string `json:"access_token"`
		Token            string `json:"token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if strings.TrimSpace(payload.Error) != "" || strings.TrimSpace(payload.ErrorDescription) != "" {
			return "", fmt.Errorf("oauth token response error: %s %s", strings.TrimSpace(payload.Error), strings.TrimSpace(payload.ErrorDescription))
		}
		switch {
		case strings.TrimSpace(payload.AccessToken) != "":
			return payload.AccessToken, nil
		case strings.TrimSpace(payload.Token) != "":
			return payload.Token, nil
		}
	}

	values, err := url.ParseQuery(strings.TrimSpace(string(body)))
	if err == nil {
		if token := strings.TrimSpace(values.Get("access_token")); token != "" {
			return token, nil
		}
		if token := strings.TrimSpace(values.Get("token")); token != "" {
			return token, nil
		}
	}

	token := strings.TrimSpace(string(body))
	if isTokenLike(token) {
		return token, nil
	}
	return "", errors.New("oauth token response missing access token")
}

func isTokenLike(token string) bool {
	if token == "" {
		return false
	}
	if strings.ContainsAny(token, " \t\r\n{}[]=:&\"") {
		return false
	}
	return true
}
