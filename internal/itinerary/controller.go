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

// ListPackages godoc
//
//	@Summary		List packages
//	@Description	Returns the signed-in operator's curated packages. A package has no price.
//	@Tags			itineraries
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	PackageList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/packages [get]
func (h *Controller) ListPackages(c *gin.Context) {
	list, err := h.svc.ListPackages(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, PackageList{Packages: list})
}

// SavePackage godoc
//
//	@Summary		Save a package
//	@Description	Creates a package, or replaces the package when the path includes an id.
//	@Tags			itineraries
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string	false	"Package id. Omit this on create."
//	@Param			body	body		Package	true	"Package"
//	@Success		200		{object}	Package
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Router			/v1/packages [post]
//	@Router			/v1/packages/{id} [put]
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

// ListInquiries godoc
//
//	@Summary		List inquiries
//	@Tags			itineraries
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	InquiryList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/inquiries [get]
func (h *Controller) ListInquiries(c *gin.Context) {
	list, err := h.svc.ListInquiries(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, InquiryList{Inquiries: list})
}

// CreateInquiry godoc
//
//	@Summary		Record an inquiry
//	@Tags			itineraries
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		Inquiry	true	"Inquiry"
//	@Success		200		{object}	Inquiry
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Router			/v1/inquiries [post]
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

// ListItineraries godoc
//
//	@Summary		List itineraries
//	@Description	Returns itineraries for the signed-in operator.
//	@Tags			itineraries
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	ItineraryList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/itineraries [get]
func (h *Controller) ListItineraries(c *gin.Context) {
	list, err := h.svc.ListItineraries(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ItineraryList{Itineraries: list})
}

// GetItinerary godoc
//
//	@Summary		Read an itinerary
//	@Tags			itineraries
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Itinerary id"
//	@Success		200	{object}	Itinerary
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Failure		404	{object}	middleware.ErrorResponse
//	@Router			/v1/itineraries/{id} [get]
func (h *Controller) GetItinerary(c *gin.Context) {
	item, err := h.svc.GetItinerary(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// Mine godoc
//
//	@Summary		My itineraries
//	@Description	Returns itineraries attached to the signed-in tourist.
//	@Tags			itineraries
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	ItineraryList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/itineraries/mine [get]
func (h *Controller) Mine(c *gin.Context) {
	list, err := h.svc.Mine(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ItineraryList{Itineraries: list})
}

// SaveItinerary godoc
//
//	@Summary		Save an itinerary
//	@Description	Creates an itinerary, or replaces it when the path includes an id. Create responds with 201.
//	@Tags			itineraries
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string		false	"Itinerary id. Omit this on create."
//	@Param			body	body		Itinerary	true	"Itinerary"
//	@Success		200		{object}	Itinerary
//	@Success		201		{object}	Itinerary
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Router			/v1/itineraries [post]
//	@Router			/v1/itineraries/{id} [put]
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

// Send godoc
//
//	@Summary		Email an itinerary
//	@Description	Sends the itinerary and its pay link to the customer. The amount due is in USD.
//	@Tags			itineraries
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Itinerary id"
//	@Success		200	{object}	Itinerary
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Failure		404	{object}	middleware.ErrorResponse
//	@Failure		422	{object}	middleware.ErrorResponse
//	@Router			/v1/itineraries/{id}/send [post]
func (h *Controller) Send(c *gin.Context) {
	item, err := h.svc.Send(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// Assign godoc
//
//	@Summary		Attach an itinerary to a tourist
//	@Tags			itineraries
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string			true	"Itinerary id"
//	@Param			body	body		AssignRequest	true	"Tourist"
//	@Success		200		{object}	Itinerary
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		404		{object}	middleware.ErrorResponse
//	@Router			/v1/itineraries/{id}/assign [post]
func (h *Controller) Assign(c *gin.Context) {
	var req AssignRequest
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

// PublicItinerary godoc
//
//	@Summary		Read an itinerary from its pay link
//	@Description	No session is required. The token is the secret in the emailed link.
//	@Tags			itineraries
//	@Produce		json
//	@Param			token	path		string	true	"Pay token"
//	@Success		200		{object}	Itinerary
//	@Failure		404		{object}	middleware.ErrorResponse
//	@Router			/v1/pay/{token} [get]
func (h *Controller) PublicItinerary(c *gin.Context) {
	item, err := h.svc.PublicItinerary(c.Request.Context(), c.Param("token"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// Pay godoc
//
//	@Summary		Pay an itinerary by card
//	@Description	Records the USD card payment for the itinerary behind this pay link. No session is required.
//	@Tags			itineraries
//	@Produce		json
//	@Param			token	path		string	true	"Pay token"
//	@Success		200		{object}	Itinerary
//	@Failure		404		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/pay/{token} [post]
func (h *Controller) Pay(c *gin.Context) {
	item, err := h.svc.Pay(c.Request.Context(), c.Param("token"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
