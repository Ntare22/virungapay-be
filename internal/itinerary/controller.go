package itinerary

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"virungapay-be/internal/middleware"
)

// Controller serves package, inquiry, itinerary, and public pay routes.
type Controller struct {
	svc *Service
}

// NewController builds the HTTP controller.
func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

func (h *Controller) ListPackages(c *gin.Context) {
	list, err := h.svc.ListPackages(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"packages": list})
}

func (h *Controller) SavePackage(c *gin.Context) {
	var pkg Package
	if err := c.ShouldBindJSON(&pkg); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if id := c.Param("id"); id != "" {
		pkg.ID = id
	}
	saved, err := h.svc.SavePackage(c.Request.Context(), middleware.UserID(c), middleware.Role(c), pkg)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, saved)
}

func (h *Controller) ListInquiries(c *gin.Context) {
	list, err := h.svc.ListInquiries(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"inquiries": list})
}

func (h *Controller) CreateInquiry(c *gin.Context) {
	var inquiry Inquiry
	if err := c.ShouldBindJSON(&inquiry); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	saved, err := h.svc.CreateInquiry(c.Request.Context(), middleware.UserID(c), middleware.Role(c), inquiry)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, saved)
}

func (h *Controller) ListItineraries(c *gin.Context) {
	list, err := h.svc.ListItineraries(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"itineraries": list})
}

func (h *Controller) GetItinerary(c *gin.Context) {
	item, err := h.svc.GetItinerary(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Controller) Mine(c *gin.Context) {
	list, err := h.svc.Mine(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"itineraries": list})
}

func (h *Controller) SaveItinerary(c *gin.Context) {
	var item Itinerary
	if err := c.ShouldBindJSON(&item); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if id := c.Param("id"); id != "" {
		item.ID = id
	}
	saved, err := h.svc.SaveItinerary(c.Request.Context(), middleware.UserID(c), middleware.Role(c), item)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	status := http.StatusOK
	if c.Request.Method == http.MethodPost {
		status = http.StatusCreated
	}
	c.JSON(status, saved)
}

func (h *Controller) Send(c *gin.Context) {
	item, err := h.svc.Send(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Controller) Assign(c *gin.Context) {
	var req struct {
		TouristID string `json:"tourist_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.TouristID) == "" {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "choose a tourist account"))
		return
	}
	item, err := h.svc.Assign(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"), req.TouristID)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Controller) PublicItinerary(c *gin.Context) {
	item, err := h.svc.PublicItinerary(c.Request.Context(), c.Param("token"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Controller) Pay(c *gin.Context) {
	item, err := h.svc.Pay(c.Request.Context(), c.Param("token"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
