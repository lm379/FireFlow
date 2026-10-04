package middleware

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type firstLoginStub struct {
	first bool
	err   error
}

func (s firstLoginStub) IsFirstLogin(uint) (bool, error) { return s.first, s.err }

func TestFirstLoginPermissions(t *testing.T) {
	for _, state := range []firstLoginStub{{first: true}, {}, {err: errors.New("database unavailable")}} {
		for _, route := range []struct {
			method, path, url string
			allowed           bool
		}{
			{"POST", "/api/v1/auth/change-password", "/api/v1/auth/change-password", true},
			{"POST", "/api/v1/auth/logout", "/api/v1/auth/logout", true},
			{"GET", "/api/v1/auth/me", "/api/v1/auth/me", true},
			{"GET", "/api/v1/auth/first-login", "/api/v1/auth/first-login", true},
			{"POST", "/api/v1/auth/refresh", "/api/v1/auth/refresh", false},
			{"GET", "/api/v1/rules/", "/api/v1/rules/", false},
			{"POST", "/api/v1/cloud-configs/", "/api/v1/cloud-configs/", false},
			{"POST", "/api/v1/system/ip/sync", "/api/v1/system/ip/sync", false},
			{"DELETE", "/api/v1/rules/:id", "/api/v1/rules/1", false},
		} {
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() }, RequirePasswordChangeCompleted(state))
			reached := false
			router.Handle(route.method, route.path, func(c *gin.Context) { reached = true; c.Status(204) })
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(route.method, route.url, nil))
			want := 204
			if state.err != nil {
				want = 500
			} else if state.first && !route.allowed {
				want = 403
			}
			if w.Code != want || reached != (want == 204) {
				t.Fatalf("%s %s first=%v: %d %s", route.method, route.url, state.first, w.Code, w.Body)
			}
			if want == 403 && !strings.Contains(w.Body.String(), `"reason":"PASSWORD_CHANGE_REQUIRED"`) {
				t.Fatal("missing reason")
			}
		}
	}
}
