package healthcheck

import (
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/test"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
	"net/http"
	"testing"
)

func TestAPI(t *testing.T) {
	logger, _ := log.NewForTest()
	router := test.MockRouter(logger)
	RegisterHandlers(router, "0.9.0")
	test.Endpoint(t, router, test.APITestCase{
		"ok", "GET", "/healthcheck", "", nil, http.StatusOK, `"OK 0.9.0"`,
	})
}
