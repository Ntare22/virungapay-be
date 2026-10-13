package itinerary

import "github.com/gin-gonic/gin"

// Mount registers public pay routes and the signed-in operator routes.
func (h *Controller) Mount(r *gin.Engine, authenticate gin.HandlerFunc) {
	v1 := r.Group("/v1")
	v1.GET("/pay/:token", h.PublicItinerary)
	v1.POST("/pay/:token", h.Pay)

	authed := v1.Group("")
	authed.Use(authenticate)
	authed.GET("/packages", h.ListPackages)
	authed.POST("/packages", h.SavePackage)
	authed.PUT("/packages/:id", h.SavePackage)
	authed.GET("/inquiries", h.ListInquiries)
	authed.POST("/inquiries", h.CreateInquiry)
	authed.GET("/itineraries", h.ListItineraries)
	authed.POST("/itineraries", h.SaveItinerary)
	authed.GET("/itineraries/mine", h.Mine)
	authed.GET("/itineraries/:id", h.GetItinerary)
	authed.PUT("/itineraries/:id", h.SaveItinerary)
	authed.POST("/itineraries/:id/send", h.Send)
	authed.POST("/itineraries/:id/assign", h.Assign)
}
