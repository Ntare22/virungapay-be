package auth

import (
	"github.com/gin-gonic/gin"

	"virungapay-be/internal/middleware"
)

// Mount registers tourist, session, and staff routes.
func (h *Controller) Mount(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.POST("/auth/tourists", h.RegisterTourist)
	v1.POST("/auth/verify-email", h.VerifyEmail)
	v1.POST("/auth/login", h.Login)
	v1.POST("/auth/login-code", h.RequestLoginCode)
	v1.POST("/auth/login-code/verify", h.ConfirmLoginCode)
	v1.POST("/auth/forgot-password", h.ForgotPassword)
	v1.POST("/auth/reset-password", h.ResetPassword)
	v1.POST("/auth/invites/accept", h.AcceptInvite)

	authed := v1.Group("")
	authed.Use(middleware.RequireAuth(h))
	authed.GET("/me", h.Me)
	authed.PATCH("/me", h.UpdateProfile)

	admin := r.Group("/admin/v1")
	admin.POST("/auth/login", h.AdminLogin)
	admin.POST("/auth/forgot-password", h.AdminForgotPassword)
	admin.POST("/auth/reset-password", h.AdminResetPassword)

	staff := admin.Group("")
	staff.Use(middleware.RequireAuth(h))
	staff.Use(middleware.RequireRoles(
		RoleVAOps, RoleVAFinance, RoleVASuperadmin, RoleAutoAdmin, RoleUHOAAdmin,
	))
	staff.GET("/me", h.AdminMe)
}

// ParseToken adapts a session token for the auth middleware.
func (h *Controller) ParseToken(raw string) (middleware.TokenClaims, error) {
	claims, err := h.svc.ParseToken(raw)
	if err != nil {
		return middleware.TokenClaims{}, err
	}
	return middleware.TokenClaims{Subject: claims.Subject, Role: claims.Role}, nil
}
