package waitlist

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"virungapay-be/internal/middleware"
)

// JoinRequest is the email for the public waitlist.
type JoinRequest struct {
	Email string `json:"email" binding:"required,email" example:"amina@example.com"`
}

// JoinResponse confirms the address is on the list.
type JoinResponse struct {
	Message string `json:"message" example:"You're on the list."`
}

// Controller is the HTTP edge for the public waitlist.
type Controller struct {
	svc *Service
}

// NewController builds the HTTP controller.
func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

// Mount registers the public waitlist route.
func (h *Controller) Mount(r *gin.Engine) {
	r.POST("/v1/waitlist", h.Join)
}

// Join godoc
//
//	@Summary		Join the waitlist
//	@Description	Saves an email for the public coming-soon list. A repeat address gets the same response.
//	@Tags			waitlist
//	@Accept			json
//	@Produce		json
//	@Param			body	body		JoinRequest	true	"Email"
//	@Success		200		{object}	JoinResponse
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Router			/v1/waitlist [post]
func (h *Controller) Join(c *gin.Context) {
	var req JoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "Enter the email where we should write to you."))
		return
	}
	if err := h.svc.Join(c.Request.Context(), req.Email); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, JoinResponse{Message: "You're on the list."})
}
