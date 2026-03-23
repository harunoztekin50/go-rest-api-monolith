package auth

import (
	"context"
	"database/sql"
	stderr "errors"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/entity"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/errors"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
)

// Service encapsulates the authentication logic.
type Service interface {
	// authenticate authenticates a user using username and password.
	// It returns a JWT token if authentication succeeds. Otherwise, an error is returned.
	loginWithEmail(ctx context.Context, username, password string) (string, error)
	loginWithAnonymus(ctx context.Context, deviceKey string) (string, error)
}

// Identity represents an authenticated user identity.
type Identity interface {
	// GetID returns the user ID.
	GetID() string
	// GetName returns the user name.
	GetName() string
}

type service struct {
	signingKey      string
	tokenExpiration int
	logger          log.Logger
	repository      AuthRepository
}

// NewService creates a new authentication service.
func NewService(signingKey string, tokenExpiration int, logger log.Logger, rerepository AuthRepository) Service {
	return service{signingKey, tokenExpiration, logger, rerepository}
}

// Login authenticates a user and generates a JWT token if authentication succeeds.
// Otherwise, an error is returned.
func (s service) loginWithEmail(ctx context.Context, username, password string) (string, error) {

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	logger := s.logger.With(ctx, "user", username)

	// TODO: the following authentication logic is only for demo purpose
	var user entity.User
	if username == "demo" && password == "pass" {
		logger.Infof("authentication successful")
		user = entity.User{ID: "100", Name: "demo"}
	}

	return s.generateJWT(user)
}

func (s service) loginWithAnonymus(ctx context.Context, deviceKey string) (string, error) {
	// sistemde kulanıcı var mı
	// JWt token hazrılanır kulanıcı için
	// JWT return edilir

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	user, err := s.repository.GetUserByDeviceKey(ctx, deviceKey)

	if err != nil && stderr.Is(err, sql.ErrNoRows) {
		user, err = s.repository.CreateAnnonymusUser(ctx, deviceKey)
		if err != nil {
			s.logger.With(ctx).Errorf("user oluşturulamadı, device key: %s, err: %v", deviceKey, err)
			return "", errors.InternalServerError("")
		}
	} else if err != nil {
		s.logger.With(ctx).Errorf("DB hatası, deviceKey: %s, err: %v", deviceKey, err)
		return "", errors.InternalServerError("")
	}

	token, err := s.generateJWT(user)
	if err != nil {
		s.logger.With(ctx).Errorf("JWT üretilemedi, userID: %s, err: %v", user.ID, err)
		return "", errors.InternalServerError("")
	}
	return token, nil

}

// generateJWT generates a JWT that encodes an identity.
func (s service) generateJWT(identity Identity) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   identity.GetID(),
		"name": identity.GetName(),
		"exp":  time.Now().Add(time.Duration(s.tokenExpiration) * time.Minute).Unix(),
	}).SignedString([]byte(s.signingKey))
}
