package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPhoneBodiesBind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		path string
		body string
		bind func(*gin.Context) error
	}{
		{"/tourists", `{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"}`, func(c *gin.Context) error {
			var req TouristSignupRequest
			return c.ShouldBindJSON(&req)
		}},
		{"/login-code", `{"email":"jim.ntare@gmail.com"}`, func(c *gin.Context) error {
			var req LoginCodeRequest
			return c.ShouldBindJSON(&req)
		}},
		{"/verify", `{"email":"jim.ntare@gmail.com","code":"012345"}`, func(c *gin.Context) error {
			var req ConfirmLoginCodeRequest
			return c.ShouldBindJSON(&req)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			require.NoError(t, tc.bind(c))
		})
	}
}
