package entity

type AuthMetod string

const (
	AuthMethodAnonymous AuthMetod = "anonymous"
	AuthMethodGoogle    AuthMetod = "google"
	AuthMethodApple     AuthMetod = "apple"
)
