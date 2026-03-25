package auth

import (
	"net/http"

	routing "github.com/go-ozzo/ozzo-routing/v2"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/errors"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
)

// RegisterHandlers registers handlers for different HTTP requests.
func RegisterHandlers(rg *routing.RouteGroup, service Service, logger log.Logger) {
	r := &resource{
		service: service,
		logger:  logger,
	}

	rg.Post("/auth/login/email-pass", r.loginWithEmail)
	rg.Post("/auth/login/anonymus", r.loginWithAnonymus)
	rg.Post("/auth/refresh", r.refreshTokens)

}

type resource struct {
	service Service
	logger  log.Logger
}

// loginWithEmail returns a handler that handles user loginWithEmail request.
func (r *resource) loginWithEmail(c *routing.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Read(&req); err != nil {
		r.logger.With(c.Request.Context()).Errorf("invalid request: %v", err)
		return errors.BadRequest("")
	}

	authTokens, err := r.service.loginWithEmail(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		r.logger.With(c.Request.Context()).Errorf("loginWithEmail failed: %v", err)
		return err
	}
	return c.WriteWithStatus(authTokens, http.StatusOK)

}
func (r *resource) loginWithAnonymus(c *routing.Context) error {
	var req struct {
		DeviceKey string `json:"device_key"`
	}

	if err := c.Read(&req); err != nil {
		r.logger.With(c.Request.Context()).Errorf("invalid request: %v", err)
		return errors.BadRequest("")
	}

	if req.DeviceKey == "" {
		r.logger.With(c.Request.Context()).Errorf("device_key boş")
		return errors.BadRequest("device_key boş olamaz")
	}

	authToken, err := r.service.loginWithAnonymus(c.Request.Context(), req.DeviceKey)
	if err != nil {
		r.logger.With(c.Request.Context()).Errorf("login failed: %v", err)
		return err
	}
	return c.WriteWithStatus(authToken, http.StatusOK)
}

func (r *resource) refreshTokens(c *routing.Context) error {
	var req struct {
		DeviceKey    string `json:"device_key"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.Read(&req); err != nil {
		r.logger.With(c.Request.Context()).Errorf("invalid request: %v", err)
		return errors.BadRequest("")
	}

	if req.DeviceKey == "" || req.RefreshToken == "" {
		r.logger.With(c.Request.Context()).Errorf("device_key boş")
		return errors.BadRequest("device_key boş olamaz , ve refresh token boş olamaz")
	}

	authToken, err := r.service.RefreshToken(c.Request.Context(), req.DeviceKey, req.RefreshToken)
	if err != nil {
		r.logger.With(c.Request.Context()).Errorf("refreshTokens failed: %v", err)
		return err
	}
	return c.WriteWithStatus(authToken, http.StatusOK)
}
