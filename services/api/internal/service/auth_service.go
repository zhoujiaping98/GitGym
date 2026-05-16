package service

import "context"

type GitHubProfile struct {
	ID        uint64
	Login     string
	Name      string
	AvatarURL string
	Email     string
}

type UserStore interface {
	UpsertGitHubUser(ctx context.Context, profile GitHubProfile) (uint64, error)
	CreateBrowserSession(ctx context.Context, userID uint64, tokenHash string, userAgent string, ip string) error
}
