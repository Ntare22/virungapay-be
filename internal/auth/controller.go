package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"virungapay-be/internal/middleware"
)

// Controller is the HTTP edge for tourist signup, sessions, and account recovery.
type Controller struct {
	svc *Service
}

// NewController mounts nothing. The server attaches these handlers.
func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

// RegisterTourist godoc
//
//	@Summary		Register a tourist
//	@Description	Creates a tourist with no password. Sign-in on the phone uses a code emailed to this address. The response has no session token.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		TouristSignupRequest	true	"Tourist"
//	@Success		201		{object}	RegisterResponse
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		409		{object}	middleware.ErrorResponse
//	@Router			/v1/auth/tourists [post]
func (h *Controller) RegisterTourist(c *gin.Context) {
	var req TouristSignupRequest
	if !bind(c, &req) {
		return
	}
	user, err := h.svc.RegisterTourist(c.Request.Context(), Signup{
		FirstName: req.FirstName, LastName: req.LastName, Email: req.Email,
	})
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, RegisterResponse{User: user.public("")})
}

// VerifyEmail godoc
//
//	@Summary		Verify an email
//	@Description	Consumes the token from the verification email.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		TokenRequest	true	"Verification token"
//	@Success		200		{object}	MessageResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/auth/verify-email [post]
func (h *Controller) VerifyEmail(c *gin.Context) {
	var req TokenRequest
	if !bind(c, &req) {
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, MessageResponse{Message: "email verified"})
}

// Login godoc
//
//	@Summary		Sign in on the PWA
//	@Description	Signs in a verified active tourist, operator owner, guide, or driver.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginRequest	true	"Credentials"
//	@Success		200		{object}	Session
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Router			/v1/auth/login [post]
func (h *Controller) Login(c *gin.Context) {
	h.login(c, AudiencePWA)
}

// AdminLogin godoc
//
//	@Summary		Admin login
//	@Description	Signs in a staff role on /admin/v1. Tourists, operators, guides, and drivers are rejected.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginRequest	true	"Credentials"
//	@Success		200		{object}	Session
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/auth/login [post]
func (h *Controller) AdminLogin(c *gin.Context) {
	h.login(c, AudienceAdmin)
}

func (h *Controller) login(c *gin.Context, audience string) {
	var req LoginRequest
	if !bind(c, &req) {
		return
	}
	session, err := h.svc.Login(c.Request.Context(), Login{Email: req.Email, Password: req.Password}, audience)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

// RequestLoginCode godoc
//
//	@Summary		Email a phone sign-in code
//	@Description	Sends a code when the email can sign in on the phone. The response is the same when the email is unknown, and it never contains the code.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginCodeRequest	true	"Email"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Router			/v1/auth/login-code [post]
func (h *Controller) RequestLoginCode(c *gin.Context) {
	var req LoginCodeRequest
	if !bind(c, &req) {
		return
	}
	if err := h.svc.RequestLoginCode(c.Request.Context(), req.Email); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, MessageResponse{Message: "If this email can sign in on the phone, we sent a code."})
}

// ConfirmLoginCode godoc
//
//	@Summary		Sign in with a phone code
//	@Description	Consumes the emailed code and returns a session for the phone app.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ConfirmLoginCodeRequest	true	"Email and code"
//	@Success		200		{object}	Session
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Router			/v1/auth/login-code/verify [post]
func (h *Controller) ConfirmLoginCode(c *gin.Context) {
	var req ConfirmLoginCodeRequest
	if !bind(c, &req) {
		return
	}
	session, err := h.svc.ConfirmLoginCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

// ForgotPassword godoc
//
//	@Summary		Request a password reset
//	@Description	Emails a reset link when the account can sign in on this API. The response is the same when the email is unknown, and it never contains the token.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ForgotRequest	true	"Email"
//	@Success		200		{object}	MessageResponse
//	@Router			/v1/auth/forgot-password [post]
func (h *Controller) ForgotPassword(c *gin.Context) {
	h.forgot(c, AudiencePWA)
}

// AdminForgotPassword godoc
//
//	@Summary		Admin forgot password
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ForgotRequest	true	"Email"
//	@Success		200		{object}	MessageResponse
//	@Router			/admin/v1/auth/forgot-password [post]
func (h *Controller) AdminForgotPassword(c *gin.Context) {
	h.forgot(c, AudienceAdmin)
}

func (h *Controller) forgot(c *gin.Context, audience string) {
	var req ForgotRequest
	if !bind(c, &req) {
		return
	}
	if err := h.svc.ForgotPassword(c.Request.Context(), req.Email, audience); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, MessageResponse{Message: "if an account exists, a reset email has been sent"})
}

// ResetPassword godoc
//
//	@Summary		Set a new password
//	@Description	Consumes a reset token. The response does not contain a token.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ResetRequest	true	"Reset"
//	@Success		200		{object}	MessageResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/auth/reset-password [post]
func (h *Controller) ResetPassword(c *gin.Context) {
	h.reset(c, AudiencePWA)
}

// AdminResetPassword godoc
//
//	@Summary		Admin reset password
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ResetRequest	true	"Reset"
//	@Success		200		{object}	MessageResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/auth/reset-password [post]
func (h *Controller) AdminResetPassword(c *gin.Context) {
	h.reset(c, AudienceAdmin)
}

func (h *Controller) reset(c *gin.Context, audience string) {
	var req ResetRequest
	if !bind(c, &req) {
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), ResetPassword{Token: req.Token, Password: req.Password}, audience); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, MessageResponse{Message: "password updated"})
}

// AcceptInvite godoc
//
//	@Summary		Accept a guide or driver invite
//	@Description	Sets a password, verifies the email, and activates the invited user.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		AcceptRequest	true	"Invite"
//	@Success		200		{object}	Session
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/auth/invites/accept [post]
func (h *Controller) AcceptInvite(c *gin.Context) {
	var req AcceptRequest
	if !bind(c, &req) {
		return
	}
	session, err := h.svc.AcceptInvite(c.Request.Context(), AcceptInvite{Token: req.Token, Password: req.Password})
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

// Me godoc
//
//	@Summary		Current PWA user
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	PublicUser
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Router			/v1/me [get]
func (h *Controller) Me(c *gin.Context) {
	h.me(c)
}

// AdminMe godoc
//
//	@Summary		Admin current user
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	PublicUser
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/me [get]
func (h *Controller) AdminMe(c *gin.Context) {
	h.me(c)
}

// UpdateProfile godoc
//
//	@Summary		Add a phone number
//	@Description	A signed-in tourist saves a phone number on their profile.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		ProfileRequest	true	"Phone"
//	@Success		200		{object}	PublicUser
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		409		{object}	middleware.ErrorResponse
//	@Router			/v1/me [patch]
func (h *Controller) UpdateProfile(c *gin.Context) {
	var req ProfileRequest
	if !bind(c, &req) {
		return
	}
	user, err := h.svc.UpdatePhone(c.Request.Context(), middleware.UserID(c), req.Phone)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Controller) me(c *gin.Context) {
	user, err := h.svc.Me(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return false
	}
	return true
}
