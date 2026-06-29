package auth

import "time"

type User struct {
	ID             string
	Username       string
	DisplayName    string
	IsAdmin        bool
	Status         string
	LinuxDoBinding *LinuxDoBinding
}

type Session struct {
	ID        string
	UserID    string
	CSRFToken string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type LinuxDoBinding struct {
	Bound           bool
	LinuxDoUserID   string
	LinuxDoUsername string
	TrustLevel      int
	AvatarURL       string
	BoundAt         time.Time
	LastSyncedAt    time.Time
}

type OAuthProfile struct {
	Provider         string
	Subject          string
	Username         string
	DisplayName      string
	Email            string
	AvatarURL        string
	TrustLevel       int
	GrantAdmin       bool
	LinuxDoUserID    string
	LinuxDoUsername  string
	LinuxDoAvatarURL string
}

type PasswordCredential struct {
	User      User
	Algorithm string
	Salt      string
	Hash      string
}

type OAuthUserResult struct {
	User    User
	Created bool
}

type SetPasswordInput struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

type EmailRegistrationStartInput struct {
	Email string
}

type EmailRegistrationChallenge struct {
	Email     string
	ExpiresAt time.Time
	DevCode   string
}

type EmailRegistrationConfirmInput struct {
	Email              string
	Code               string
	UsernameCandidates []string
}
