package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTErrorEnvelopes(t *testing.T) {
	oldSecret, oldValidator := jwtSecret, tokenValidator
	jwtSecret, tokenValidator = []byte("test-response-secret"), nil
	t.Cleanup(func() { jwtSecret, tokenValidator = oldSecret, oldValidator })
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}})
	token, err := expired.SignedString(jwtSecret)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		header string
		code   int
		reason string
	}{
		{"", 401, "MISSING_AUTH_HEADER"},
		{"Basic invalid", 401, "INVALID_AUTH_HEADER"},
		{"Bearer malformed", 401, "TOKEN_MALFORMED"},
		{"Bearer " + token, 40101, "TOKEN_EXPIRED"},
	} {
		router := gin.New()
		reached := false
		router.GET("/private", JWTAuthMiddleware(), func(c *gin.Context) { reached = true })
		req := httptest.NewRequest("GET", "/private", nil)
		req.Header.Set("Authorization", test.header)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusUnauthorized || reached || body["code"] != float64(test.code) || body["reason"] != test.reason || body["data"] != nil || body["msg"] == "" || len(body) != 4 {
			t.Fatalf("invalid auth response: %d %s", w.Code, w.Body)
		}
	}
}

func TestAPIRecoveryUsesErrorEnvelope(t *testing.T) {
	router := gin.New()
	router.Use(APIRecovery())
	router.GET("/api/v1/panic", func(c *gin.Context) { panic("private internal detail") })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/panic", nil))
	if w.Code != http.StatusInternalServerError || w.Body.String() != `{"code":500,"data":null,"msg":"Internal server error"}` {
		t.Fatalf("unexpected recovery response: %d %s", w.Code, w.Body)
	}
}
