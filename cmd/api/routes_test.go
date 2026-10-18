package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSwaggerUsesTheRequestHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	mountDocs(r)

	req := httptest.NewRequest(http.MethodGet, "/docs/doc.json", nil)
	req.Host = "internal.railway"
	req.Header.Set("X-Forwarded-Host", "dev.api.virungapay.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"host": "dev.api.virungapay.com"`)
	require.Contains(t, rr.Body.String(), `"schemes": ["https"]`)
	require.NotContains(t, rr.Body.String(), "localhost")
}

func TestHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/healthz", healthz)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"status":"ok"}`, rr.Body.String())
}
