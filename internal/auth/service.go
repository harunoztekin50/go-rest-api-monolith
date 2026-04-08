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
	RefreshToken(ctx context.Context, deviceKey, refreshToken string) (entity.AuthTokens, error)
	GetUser(ctx context.Context, userID string) (entity.User, error)
	LogOutWithDevice(ctx context.Context, deviceKey, userID string) error
	LogOutWithUserID(ctx context.Context, userID string) error
}

// Identity represents an authenticated user identity.

type service struct {
	signingKey      string
	tokenExpiration int
	logger          log.Logger
	repository      AuthRepository
}

// GetUserByDeviceKey implements [Service].

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
		return entity.AuthTokens{}, errors.InternalServerError("TOKEN_GENERATION_FAILED")
	}
	return entity.AuthTokens{
		RefreshToken: "",
		AccessToken:  accessToken,
	}, err

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
			s.logger.With(ctx).Errorf("kullanıcı oluşturulamadı, deviceKey: %s, err: %v", deviceKey, err)
			return authToken, errors.InternalServerError("USER_CREATE_FAILED")
		}
	} else if err != nil {
		return authToken, errors.InternalServerError("DB_ERROR")
	}

	return s.createAuthToken(ctx, user)
}

func (s service) CreateHashToken(refreshToken string) string {
	hash := sha256.Sum256([]byte(refreshToken))
	refreshTokenHashed := fmt.Sprintf("%x", hash)
	return refreshTokenHashed
}

func (s service) RefreshToken(ctx context.Context, deviceKey, refreshToken string) (entity.AuthTokens, error) {

	var authToken entity.AuthTokens

	hashedRefeshToken := s.CreateHashToken(refreshToken)

	userID, err := s.repository.ValidateRefreshToken(ctx, deviceKey, hashedRefeshToken)

	if stderr.Is(err, sql.ErrNoRows) {
		return authToken, errors.Unauthorized("")
	} else if err != nil {
		return authToken, errors.InternalServerError("")
	}

	user, err := s.repository.GetUserByUserID(ctx, userID)

	if err != nil {
		s.logger.With(ctx).Errorf("kullanıcı bulunamadı, userID: %s, err: %v", userID, err)
		return authToken, errors.InternalServerError("")
	}

	return s.createAuthToken(ctx, user)

}

func (s service) createAuthToken(ctx context.Context, user *entity.User) (entity.AuthTokens, error) {

	var authToken entity.AuthTokens

	accessToken, err := s.generateJWT(user)
	if err != nil {
		s.logger.With(ctx).Errorf("JWT üretilemedi, userID: %s, err: %v", user.ID, err)
		return authToken, errors.InternalServerError("TOKEN_GENERATION_FAILED")
	}

	refreshTokenCreate := entity.GenerateID()
	refreshTokenHashed := s.CreateHashToken(refreshTokenCreate)

	err = s.repository.CreateNewRefreshToken(ctx, user.AuthID, user.ID, refreshTokenHashed)

	if err != nil {
		s.logger.With(ctx).Errorf("refresh token oluşturulamadı, userID: %s, err: %v", user.ID, err)
		return authToken, errors.InternalServerError("")
	}
	authToken.AccessToken = accessToken
	authToken.RefreshToken = refreshTokenCreate

	return authToken, err

}

func (s service) GetUser(ctx context.Context, userID string) (entity.User, error) {
	user, err := s.repository.GetUserByUserID(ctx, userID)
	if err != nil {
		if stderr.Is(err, sql.ErrNoRows) {
			return entity.User{}, errors.NotFound("user not found")
		}
		return entity.User{}, errors.InternalServerError("")
	}
	return *user, err
}
func (s service) LogOutWithDevice(ctx context.Context, deviceKey, userID string) error {
	err := s.repository.LogOutWithDevice(ctx, deviceKey, userID)
	if err != nil {
		switch {
		case stderr.Is(err, context.DeadlineExceeded):
			s.logger.With(ctx).Errorf("logout timeout: userID=%s deviceKey=%s err=%v", userID, deviceKey, err)
			return errors.InternalServerError("")
		case stderr.Is(err, context.Canceled):
			s.logger.With(ctx).Errorf("logout canceled: userID=%s deviceKey=%s", userID, deviceKey)
			return errors.InternalServerError("")
		default:
			s.logger.With(ctx).Errorf("logout failed: userID=%s deviceKey=%s err=%v", userID, deviceKey, err)
			return errors.InternalServerError("")
		}
	}

	s.logger.With(ctx).Infof("logout success: userID=%s deviceKey=%s", userID, deviceKey)
	return err
}

func (s service) LogOutWithUserID(ctx context.Context, userID string) error {
	err := s.repository.LogOutAll(ctx, userID)
	if err != nil {
		switch {
		case stderr.Is(err, context.DeadlineExceeded):
			s.logger.With(ctx).Errorf("logout-all timeout: userID=%s err=%v", userID, err)
			return errors.InternalServerError("")
		case stderr.Is(err, context.Canceled):
			s.logger.With(ctx).Errorf("logout-all canceled: userID=%s", userID)
			return errors.InternalServerError("")
		default:
			s.logger.With(ctx).Errorf("logout-all failed: userID=%s err=%v", userID, err)
			return errors.InternalServerError("")
		}
	}

	s.logger.With(ctx).Infof("logout-all success: userID=%s", userID)
	return err
}

// generateJWT generates a JWT that encodes an identity.
func (s service) generateJWT(identity Identity) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   identity.GetID(),
		"name": identity.GetName(),
		"exp":  time.Now().Add(time.Duration(s.tokenExpiration) * time.Minute).Unix(),
	}).SignedString([]byte(s.signingKey))
}
