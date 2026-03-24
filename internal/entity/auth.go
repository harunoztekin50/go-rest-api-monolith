package entity

type AuthMetod string

const (
	AuthMethodAnonymous AuthMetod = "anonymous"
	AuthMethodGoogle    AuthMetod = "google"
	AuthMethodApple     AuthMetod = "apple"
)

type AuthTokens struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}
