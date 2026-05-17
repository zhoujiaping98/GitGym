package service

import (
	"context"
	"errors"

	"gitgym/services/api/internal/domain"
)

var ErrBrowserSessionNotFound = errors.New("browser session not found")

type GitHubProfile struct {
	ID        uint64
	Login     string
	Name      string
	AvatarURL string
	Email     string
}

type BrowserSessionRecord struct {
	UserID    uint64
	TokenHash string
	UserAgent string
	IPAddress string
	ExpiresAt string
}

type UserStore interface {
	UpsertGitHubUser(ctx context.Context, profile GitHubProfile) (uint64, error)
	GetUserByGitHubID(ctx context.Context, githubUserID uint64) (domain.CurrentUser, error)
	GetUserByID(ctx context.Context, userID uint64) (domain.CurrentUser, error)
	CreateBrowserSession(ctx context.Context, userID uint64, tokenHash string, userAgent string, ip string) error
	GetBrowserSessionByTokenHash(ctx context.Context, tokenHash string) (domain.BrowserSession, error)
	RevokeBrowserSession(ctx context.Context, tokenHash string) error
}
