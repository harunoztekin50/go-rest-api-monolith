package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	stderr "errors"
	"fmt"
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
	loginWithEmail(ctx context.Context, username, password string) (entity.AuthTokens, error)
	loginWithAnonymus(ctx context.Context, deviceKey string) (entity.AuthTokens, error)
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
func (s service) loginWithEmail(ctx context.Context, username, password string) (entity.AuthTokens, error) {

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	logger := s.logger.With(ctx, "user", username)

	// TODO: the following authentication logic is only for demo purpose
	var user entity.User
	if username == "demo" && password == "pass" {
		logger.Infof("authentication successful")
		user = entity.User{ID: "100", Name: "demo"}
	}

	accessToken, err := s.generateJWT(user)

	if err != nil {
		return entity.AuthTokens{}, nil
	}

	return entity.AuthTokens{
		RefreshToken: "",
		AccessToken:  accessToken,
	}, nil

}

func (s service) loginWithAnonymus(ctx context.Context, deviceKey string) (entity.AuthTokens, error) {
	// sistemde kulanıcı var mı
	// JWt token hazrılanır kulanıcı için
	// JWT return edilir

	var authToken entity.AuthTokens

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	user, err := s.repository.GetUserByDeviceKey(ctx, deviceKey)

	if err != nil && stderr.Is(err, sql.ErrNoRows) {
		user, err = s.repository.CreateAnnonymusUser(ctx, deviceKey)
		if err != nil {
			s.logger.With(ctx).Errorf("user oluşturulamadı, device key: %s, err: %v", deviceKey, err)
			return authToken, errors.InternalServerError("")
		}
	} else if err != nil {
		s.logger.With(ctx).Errorf("DB hatası, deviceKey: %s, err: %v", deviceKey, err)
		return authToken, errors.InternalServerError("")
	}

	accessToken, err := s.generateJWT(user)
	if err != nil {
		s.logger.With(ctx).Errorf("JWT üretilemedi, userID: %s, err: %v", user.ID, err)
		return authToken, errors.InternalServerError("")
	}

	refreshToken := entity.GenerateID()
	hash := sha256.Sum256([]byte(refreshToken))
	refreshTokenHashed := fmt.Sprintf("%x", hash)

	err = s.repository.CreateNewRefreshToken(ctx, deviceKey, user.ID, refreshTokenHashed)

	if err != nil {
		return authToken, err
	}

	authToken.AccessToken = accessToken
	authToken.RefreshToken = refreshToken

	return authToken, nil
}

// generateJWT generates a JWT that encodes an identity.
func (s service) generateJWT(identity Identity) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   identity.GetID(),
		"name": identity.GetName(),
		"exp":  time.Now().Add(time.Duration(s.tokenExpiration) * time.Minute).Unix(),
	}).SignedString([]byte(s.signingKey))
}
