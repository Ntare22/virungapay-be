package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"virungapay-be/docs"
	"virungapay-be/internal/auth"
	"virungapay-be/internal/company"
	"virungapay-be/internal/config"
	"virungapay-be/internal/database"
	"virungapay-be/internal/itinerary"
	"virungapay-be/internal/mail"
	"virungapay-be/internal/middleware"
	"virungapay-be/internal/waitlist"
)

// statusResponse is the body of the liveness and readiness probes.
type statusResponse struct {
	Status string `json:"status" example:"ok"`
}

func newServer(cfg config.Config, db database.Service, sender mail.Sender) (*http.Server, error) {
	handler, err := newHandler(cfg, db, sender)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}, nil
}

func newHandler(cfg config.Config, db database.Service, sender mail.Sender) (http.Handler, error) {
	if !cfg.ExposeDocs() {
		gin.SetMode(gin.ReleaseMode)
	}
	users := auth.NewRepository(db.DB())
	companies := company.NewRepository(db.DB())
	authSvc := auth.NewService(users, companies, companies, companies, sender, cfg.JWTSecret, cfg.Mail.PublicURL)
	if err := authSvc.EnsureStaff(context.Background(), cfg.Staff.Name, cfg.Staff.Email, cfg.Staff.Password); err != nil {
		return nil, err
	}
	companySvc := company.NewService(companies, authSvc, sender, db.WithTx, cfg.Mail.PublicURL)
	itineraries := itinerary.NewService(itinerary.NewRepository(db.DB()), companySvc, sender, db.WithTx, cfg.Mail.PublicURL, cfg.Mail.PWAURL)
	authCtrl := auth.NewController(authSvc)
	companyCtrl := company.NewController(companySvc)
	itineraryCtrl := itinerary.NewController(itineraries)

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.AccessLog())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(cfg.CORSOrigins))

	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz(db))
	if cfg.ExposeDocs() {
		mountDocs(r)
	}
	authCtrl.Mount(r)
	waitlist.NewController(waitlist.NewService(waitlist.NewRepository(db.DB()))).Mount(r)
	companyCtrl.Mount(r, middleware.RequireAuth(authCtrl))
	companyCtrl.MountAdmin(r, middleware.RequireAuth(authCtrl))
	itineraryCtrl.Mount(r, middleware.RequireAuth(authCtrl))
	return r, nil
}

func mountDocs(r *gin.Engine) {
	docs := ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/doc.json"), ginSwagger.PersistAuthorization(true))
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/docs/index.html")
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/docs/index.html")
	})
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/docs/index.html")
	})
	r.GET("/swagger/*any", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/docs/index.html")
	})
	r.GET("/docs/*any", func(c *gin.Context) {
		switch c.Param("any") {
		case "/", "":
			c.Redirect(http.StatusFound, "/docs/index.html")
		case "/doc.json":
			swaggerSpec(c)
		default:
			docs(c)
		}
	})
}

// swaggerSpec serves the OpenAPI document with the host that received the request.
func swaggerSpec(c *gin.Context) {
	spec := *docs.SwaggerInfo
	spec.Host, spec.Schemes = requestOrigin(c.Request)
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(spec.ReadDoc()))
}

func requestOrigin(r *http.Request) (string, []string) {
	host := r.Host
	if forwarded := headerFirst(r, "X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := headerFirst(r, "X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host = strings.TrimSpace(host)
	if scheme == "https" {
		host = strings.TrimSuffix(host, ":443")
	}
	if scheme == "http" {
		host = strings.TrimSuffix(host, ":80")
	}
	return host, []string{scheme}
}

func headerFirst(r *http.Request, name string) string {
	value := strings.TrimSpace(r.Header.Get(name))
	if value == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(value, ",")[0])
}

// healthz godoc
//
//	@Summary		Liveness
//	@Description	Reports that the process is running.
//	@Tags			probes
//	@Produce		json
//	@Success		200	{object}	statusResponse
//	@Router			/healthz [get]
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, statusResponse{Status: "ok"})
}

// readyz godoc
//
//	@Summary		Readiness
//	@Description	Pings Postgres.
//	@Tags			probes
//	@Produce		json
//	@Success		200	{object}	statusResponse
//	@Failure		503	{object}	middleware.ErrorResponse
//	@Router			/readyz [get]
func readyz(db database.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		sqlDB, err := db.DB().DB()
		if err == nil {
			err = sqlDB.PingContext(ctx)
		}
		if err != nil {
			zerolog.Ctx(c.Request.Context()).Error().Err(err).Msg("readyz")
			middleware.WriteError(c, middleware.Unavailable("not_ready", "database is not ready"))
			return
		}
		c.JSON(http.StatusOK, statusResponse{Status: "ready"})
	}
}
