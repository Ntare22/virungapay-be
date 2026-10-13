package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"virungapay-be/internal/config"
	"virungapay-be/internal/database"
	"virungapay-be/internal/mail"
)

func TestTouristAndOperatorSignupFlow(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("virungapay"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, container.Terminate(context.Background()))
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dsn := fmt.Sprintf("postgres://user:password@%s:%s/virungapay?sslmode=disable", host, port.Port())

	require.NoError(t, database.MigrateUp(dsn, filepath.Join(moduleRoot(t), "migrations")))

	db, err := database.New(config.DatabaseConfig{URL: dsn, MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: time.Minute})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	box := &mail.Recorder{}
	cfg := config.Config{
		Port:        8080,
		AppEnv:      "test",
		JWTSecret:   "test-secret",
		CORSOrigins: []string{"http://localhost:5173"},
		Mail:        config.MailConfig{PublicURL: "http://app.test"},
	}
	handler, err := newHandler(cfg, db, box)
	require.NoError(t, err)

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusOK, ready.Code)
	require.JSONEq(t, `{"status":"ready"}`, ready.Body.String())

	tourist := postJSON(t, handler, "/v1/auth/tourists", map[string]string{
		"first_name": "Amelie", "last_name": "Laurent", "email": "amelie@example.com", "password": "password1",
	}, "")
	require.Equal(t, http.StatusCreated, tourist.Code)
	require.NotContains(t, tourist.Body.String(), "token")
	verify := postJSON(t, handler, "/v1/auth/verify-email", map[string]string{"token": box.TokenFor("amelie@example.com")}, "")
	require.Equal(t, http.StatusOK, verify.Code)
	login := postJSON(t, handler, "/v1/auth/login", map[string]string{"email": "amelie@example.com", "password": "password1"}, "")
	require.Equal(t, http.StatusOK, login.Code)
	touristToken := jsonField(t, login.Body.Bytes(), "token")

	forgot := postJSON(t, handler, "/v1/auth/forgot-password", map[string]string{"email": "amelie@example.com"}, "")
	require.Equal(t, http.StatusOK, forgot.Code)
	require.NotContains(t, forgot.Body.String(), "token=")
	reset := postJSON(t, handler, "/v1/auth/reset-password", map[string]string{
		"token": box.TokenFor("amelie@example.com"), "password": "password2",
	}, "")
	require.Equal(t, http.StatusOK, reset.Code)
	relogin := postJSON(t, handler, "/v1/auth/login", map[string]string{"email": "amelie@example.com", "password": "password2"}, "")
	require.Equal(t, http.StatusOK, relogin.Code)

	operator := postJSON(t, handler, "/v1/operators", map[string]string{
		"first_name": "Grace", "last_name": "Namuli", "email": "grace@example.com", "password": "password1",
	}, "")
	require.Equal(t, http.StatusCreated, operator.Code)
	require.NotContains(t, operator.Body.String(), "token")
	verified := postJSON(t, handler, "/v1/auth/verify-email", map[string]string{"token": box.TokenFor("grace@example.com")}, "")
	require.Equal(t, http.StatusOK, verified.Code)
	ownerLogin := postJSON(t, handler, "/v1/auth/login", map[string]string{"email": "grace@example.com", "password": "password1"}, "")
	require.Equal(t, http.StatusOK, ownerLogin.Code)
	ownerToken := jsonField(t, ownerLogin.Body.Bytes(), "token")

	denied := postJSON(t, handler, "/v1/companies", map[string]string{"name": "No Company", "country": "UG"}, touristToken)
	require.Equal(t, http.StatusForbidden, denied.Code)

	created := postJSON(t, handler, "/v1/companies", map[string]string{"name": "Kigezi Highland Safaris", "country": "ug", "association": "AUTO"}, ownerToken)
	require.Equal(t, http.StatusCreated, created.Code)
	again := postJSON(t, handler, "/v1/companies", map[string]string{"name": "Second", "country": "UG"}, ownerToken)
	require.Equal(t, http.StatusConflict, again.Code)

	mine := getAuth(t, handler, "/v1/companies/mine", ownerToken)
	require.Equal(t, http.StatusOK, mine.Code)
	require.Contains(t, mine.Body.String(), "Kigezi Highland Safaris")

	invite := postJSON(t, handler, "/v1/companies/invites", map[string]string{
		"name": "Peter Mugisha", "email": "peter@example.com", "role": "driver",
	}, ownerToken)
	require.Equal(t, http.StatusCreated, invite.Code)
	require.NotContains(t, invite.Body.String(), box.TokenFor("peter@example.com"))
	accepted := postJSON(t, handler, "/v1/auth/invites/accept", map[string]string{
		"token": box.TokenFor("peter@example.com"), "password": "password1",
	}, "")
	require.Equal(t, http.StatusOK, accepted.Code)
	driverLogin := postJSON(t, handler, "/v1/auth/login", map[string]string{"email": "peter@example.com", "password": "password1"}, "")
	require.Equal(t, http.StatusOK, driverLogin.Code)
	driverMe := getAuth(t, handler, "/v1/me", jsonField(t, driverLogin.Body.Bytes(), "token"))
	require.Equal(t, http.StatusOK, driverMe.Code)
	require.Contains(t, driverMe.Body.String(), "driver")
	require.Contains(t, driverMe.Body.String(), jsonField(t, created.Body.Bytes(), "id"))
}

func postJSON(t *testing.T, handler http.Handler, path string, body map[string]string, token string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func getAuth(t *testing.T, handler http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func jsonField(t *testing.T, body []byte, field string) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	value, _ := payload[field].(string)
	require.NotEmpty(t, value)
	return value
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir)
		dir = parent
	}
}
