package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"labkit.local/apps/api/internal/config"
)

// SchoolDevcenterProvider implements the school_devcenter OAuth flow.
type SchoolDevcenterProvider struct {
	cfg        config.SchoolDevcenterConfig
	httpClient *http.Client
}

// NewSchoolDevcenterProvider constructs a provider for the school_devcenter flow.
func NewSchoolDevcenterProvider(cfg config.SchoolDevcenterConfig) Provider {
	return &SchoolDevcenterProvider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *SchoolDevcenterProvider) Name() string {
	return "school_devcenter"
}

func (p *SchoolDevcenterProvider) BuildAuthorizeURL(state string) (string, error) {
	authorizeURL := strings.TrimSpace(p.cfg.AuthorizeURL)
	if authorizeURL == "" {
		return "", fmt.Errorf("oauth authorize url is required")
	}
	parsed, err := url.Parse(authorizeURL)
	if err != nil {
		return "", err
	}
	params := parsed.Query()
	params.Set("response_type", "code")
	if clientID := strings.TrimSpace(p.cfg.ClientID); clientID != "" {
		params.Set("client_id", clientID)
	}
	if redirectURL := strings.TrimSpace(p.cfg.RedirectURL); redirectURL != "" {
		params.Set("redirect_uri", redirectURL)
	}
	if scope := strings.TrimSpace(p.cfg.Scope); scope != "" {
		params.Set("scope", scope)
	}
	params.Set("state", state)
	parsed.RawQuery = params.Encode()
	return parsed.String(), nil
}

func (p *SchoolDevcenterProvider) ExchangeCode(ctx context.Context, code string) (TokenSet, error) {
	tokenURL := strings.TrimSpace(p.cfg.TokenURL)
	if tokenURL == "" {
		return TokenSet{}, fmt.Errorf("oauth token url is required")
	}
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", strings.TrimSpace(code))
	if redirectURL := strings.TrimSpace(p.cfg.RedirectURL); redirectURL != "" {
		values.Set("redirect_uri", redirectURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return TokenSet{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basic := schoolDevcenterBasicAuth(strings.TrimSpace(p.cfg.ClientID), strings.TrimSpace(p.cfg.ClientSecret)); basic != "" {
		req.Header.Set("Authorization", basic)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return TokenSet{}, fmt.Errorf("oauth token exchange failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenSet{}, err
	}
	return parseSchoolDevcenterTokenSet(body)
}

func (p *SchoolDevcenterProvider) FetchIdentity(ctx context.Context, token TokenSet) (ExternalIdentity, error) {
	if strings.TrimSpace(token.AccessToken) == "" {
		return ExternalIdentity{}, fmt.Errorf("oauth access token is required")
	}

	profileURL := strings.TrimSpace(p.cfg.ProfileURL)
	if profileURL == "" {
		return ExternalIdentity{}, fmt.Errorf("oauth profile url is required")
	}

	profileReq, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return ExternalIdentity{}, err
	}
	setBearerAuth(profileReq, token)
	profileReq.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(profileReq)
	if err != nil {
		return ExternalIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return ExternalIdentity{}, fmt.Errorf("oauth profile fetch failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var profile schoolDevcenterProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return ExternalIdentity{}, err
	}

	studentID := profile.StudentID()
	if studentID == "" {
		return ExternalIdentity{}, fmt.Errorf("oauth profile missing studentID")
	}

	subject := profile.Subject()
	name := profile.DisplayName()
	email := profile.EmailAddress()

	userURL := strings.TrimSpace(p.cfg.UserURL)
	if userURL != "" && (subject == "" || name == "") {
		user, err := p.fetchUserIdentity(ctx, userURL, token)
		if err != nil {
			return ExternalIdentity{}, err
		}
		if subject == "" {
			subject = user.Subject()
		}
		if name == "" {
			name = user.DisplayName()
		}
	}
	if subject == "" {
		subject = studentID
	}

	return ExternalIdentity{
		Provider:  p.Name(),
		Subject:   subject,
		StudentID: studentID,
		Name:      name,
		Email:     email,
	}, nil
}

type schoolDevcenterProfileResponse struct {
	UID            string                   `json:"uid"`
	StudentIDValue string                   `json:"student_id"`
	StudentIDAlt   string                   `json:"studentId"`
	StudentNo      string                   `json:"student_no"`
	Name           string                   `json:"name"`
	Email          string                   `json:"email"`
	Profiles       []schoolDevcenterProfile `json:"profiles"`
}

type schoolDevcenterProfile struct {
	StudentIDValue string `json:"student_id"`
	StudentIDAlt   string `json:"studentId"`
	StudentNo      string `json:"student_no"`
	STNo           string `json:"stno"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	IsPrimary      bool   `json:"isprimary"`
}

type schoolDevcenterUserResponse struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

func (p schoolDevcenterProfileResponse) StudentID() string {
	if studentID := firstNonEmpty(p.StudentIDValue, p.StudentIDAlt, p.StudentNo); studentID != "" {
		return studentID
	}
	for _, profile := range p.Profiles {
		if profile.IsPrimary {
			if studentID := profile.StudentID(); studentID != "" {
				return studentID
			}
		}
	}
	for _, profile := range p.Profiles {
		if studentID := profile.StudentID(); studentID != "" {
			return studentID
		}
	}
	return ""
}

func (p schoolDevcenterProfileResponse) Subject() string {
	return firstNonEmpty(p.UID)
}

func (p schoolDevcenterProfileResponse) DisplayName() string {
	if name := strings.TrimSpace(p.Name); name != "" {
		return name
	}
	for _, profile := range p.Profiles {
		if name := strings.TrimSpace(profile.Name); name != "" {
			return name
		}
	}
	return ""
}

func (p schoolDevcenterProfileResponse) EmailAddress() string {
	if email := strings.TrimSpace(p.Email); email != "" {
		return email
	}
	for _, profile := range p.Profiles {
		if email := strings.TrimSpace(profile.Email); email != "" {
			return email
		}
	}
	return ""
}

func (p schoolDevcenterProfile) StudentID() string {
	return firstNonEmpty(p.STNo, p.StudentIDValue, p.StudentIDAlt, p.StudentNo)
}

func (u schoolDevcenterUserResponse) Subject() string {
	return firstNonEmpty(u.Username)
}

func (u schoolDevcenterUserResponse) DisplayName() string {
	return firstNonEmpty(u.Name)
}

func (p *SchoolDevcenterProvider) fetchUserIdentity(ctx context.Context, userURL string, token TokenSet) (schoolDevcenterUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return schoolDevcenterUserResponse{}, err
	}
	setBearerAuth(req, token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return schoolDevcenterUserResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return schoolDevcenterUserResponse{}, fmt.Errorf("oauth user fetch failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var user schoolDevcenterUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return schoolDevcenterUserResponse{}, err
	}
	return user, nil
}

func setBearerAuth(req *http.Request, token TokenSet) {
	tokenType := strings.TrimSpace(token.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	req.Header.Set("Authorization", tokenType+" "+strings.TrimSpace(token.AccessToken))
}

func schoolDevcenterBasicAuth(clientID, clientSecret string) string {
	if clientID == "" && clientSecret == "" {
		return ""
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret))
}

func parseSchoolDevcenterTokenSet(body []byte) (TokenSet, error) {
	var payload struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		Scope            string `json:"scope"`
		ExpiresIn        int64  `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TokenSet{}, err
	}
	if strings.TrimSpace(payload.Error) != "" || strings.TrimSpace(payload.ErrorDescription) != "" {
		return TokenSet{}, fmt.Errorf("oauth token response error: %s %s", strings.TrimSpace(payload.Error), strings.TrimSpace(payload.ErrorDescription))
	}
	accessToken := strings.TrimSpace(payload.AccessToken)
	if accessToken == "" {
		return TokenSet{}, fmt.Errorf("oauth token response missing access token")
	}
	tokenSet := TokenSet{
		AccessToken: accessToken,
		TokenType:   strings.TrimSpace(payload.TokenType),
		Scope:       strings.TrimSpace(payload.Scope),
	}
	if payload.ExpiresIn > 0 {
		tokenSet.Expiry = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return tokenSet, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
