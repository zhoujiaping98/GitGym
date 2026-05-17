package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gitgym/services/api/internal/service"
	"golang.org/x/oauth2"
)

func GitHubConfig(clientID string, clientSecret string, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}
}

type GitHubOAuthClient interface {
	AuthCodeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (string, error)
	FetchProfile(ctx context.Context, accessToken string) (service.GitHubProfile, error)
}

type gitHubOAuthClient struct {
	config     *oauth2.Config
	httpClient *http.Client
}

type gitHubUserResponse struct {
	ID        uint64 `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

type gitHubEmailResponse struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func NewGitHubOAuthClient(clientID string, clientSecret string, redirectURL string, httpClient *http.Client) GitHubOAuthClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &gitHubOAuthClient{
		config:     GitHubConfig(clientID, clientSecret, redirectURL),
		httpClient: httpClient,
	}
}

func (c *gitHubOAuthClient) AuthCodeURL(state string) string {
	return c.config.AuthCodeURL(state)
}

func (c *gitHubOAuthClient) ExchangeCode(ctx context.Context, code string) (string, error) {
	token, err := c.config.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange github oauth code: %w", err)
	}
	return token.AccessToken, nil
}

func (c *gitHubOAuthClient) FetchProfile(ctx context.Context, accessToken string) (service.GitHubProfile, error) {
	var user gitHubUserResponse
	if err := c.getJSON(ctx, accessToken, "https://api.github.com/user", &user); err != nil {
		return service.GitHubProfile{}, err
	}

	email := user.Email
	if email == "" {
		var emails []gitHubEmailResponse
		if err := c.getJSON(ctx, accessToken, "https://api.github.com/user/emails", &emails); err != nil {
			return service.GitHubProfile{}, err
		}
		email = primaryVerifiedEmail(emails)
	}

	return service.GitHubProfile{
		ID:        user.ID,
		Login:     user.Login,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		Email:     email,
	}, nil
}

func (c *gitHubOAuthClient) getJSON(ctx context.Context, accessToken string, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github request %s returned %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

func primaryVerifiedEmail(emails []gitHubEmailResponse) string {
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email
		}
	}
	for _, email := range emails {
		if email.Verified {
			return email.Email
		}
	}
	return ""
}
