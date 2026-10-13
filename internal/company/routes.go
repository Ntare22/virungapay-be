package company

import (
	"github.com/gin-gonic/gin"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/middleware"
)

// Mount registers operator signup and the company routes.
// Operator signup is public. The other routes require the auth middleware passed in.
func (h *Controller) Mount(r *gin.Engine, authenticate gin.HandlerFunc) {
	v1 := r.Group("/v1")
	v1.POST("/operators", h.RegisterOperator)
	v1.POST("/bureaus", h.RegisterForexBureau)

	authed := v1.Group("")
	authed.Use(authenticate)
	authed.POST("/companies", h.Create)
	authed.GET("/companies/mine", h.Mine)
	authed.GET("/tariff", h.Tariff)
	authed.GET("/transactions", h.OperatorPayments)
	authed.GET("/wallets/mine", h.MyWallets)
	authed.GET("/wallets/mine/statement", h.WalletStatement)
	authed.GET("/bureaus", h.ApprovedBureaus)
	authed.POST("/withdrawals", h.WithdrawOwn)
	authed.POST("/wallets/moves", h.MoveWallets)
	authed.POST("/wallets", h.RequestWallet)
	authed.POST("/companies/invites", h.Invite)
}

// MountAdmin registers staff approval, wallets, and the day's tariff.
func (h *Controller) MountAdmin(r *gin.Engine, authenticate gin.HandlerFunc) {
	staff := r.Group("/admin/v1")
	staff.Use(authenticate)
	staff.Use(middleware.RequireRoles(auth.RoleVAOps, auth.RoleVAFinance, auth.RoleVASuperadmin))
	staff.GET("/businesses", h.ListBusinesses)
	staff.GET("/bureaus", h.ListBureaus)
	staff.POST("/bureaus", h.AddBureau)
	staff.POST("/businesses/:id/approve", h.Approve)
	staff.POST("/businesses/:id/disable", h.Disable)
	staff.POST("/businesses/:id/wallets", h.OpenWallet)
	staff.POST("/businesses/:id/wallets/:walletId/default", h.SetDefaultWallet)
	staff.GET("/tariff", h.StaffTariff)
	staff.PUT("/tariff", h.SetTariff)
	staff.GET("/fees", h.ListFees)
	staff.PUT("/fees", h.SaveFee)
	staff.GET("/transactions", h.Transactions)
	staff.GET("/platform-wallets", h.PlatformWallets)
	staff.GET("/platform-wallets/:id/statement", h.PlatformStatement)
	staff.POST("/collections", h.Collect)
	staff.POST("/withdrawals", h.Withdraw)
	staff.POST("/tips", h.Tip)
}
