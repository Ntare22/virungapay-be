package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	WriteError(c, Invalid("bad_request", "nope"))

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":{"code":"bad_request","message":"nope","details":null}}`, w.Body.String())
}
