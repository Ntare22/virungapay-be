package company

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/mail"
	"virungapay-be/internal/middleware"
)

// Controller is the HTTP edge for operator signup and the tour company.
type Controller struct {
	svc *Service
}

// NewController mounts nothing. The server attaches these handlers.
func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

// RegisterOperator godoc
//
//	@Summary		Register an operator owner
//	@Description	Creates an unverified operator owner and emails a verification link. This is separate from tourist signup. The response has no session token and no role field.
//	@Tags			companies
//	@Accept			json
//	@Produce		json
//	@Param			body	body		auth.SignupRequest	true	"Operator owner"
//	@Success		201		{object}	auth.RegisterResponse
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		409		{object}	middleware.ErrorResponse
//	@Router			/v1/operators [post]
func (h *Controller) RegisterOperator(c *gin.Context) {
	var req auth.SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	user, err := h.svc.RegisterOperator(c.Request.Context(), auth.Signup{
		FirstName: req.FirstName, LastName: req.LastName, Email: req.Email, Phone: req.Phone, Password: req.Password,
	})
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, auth.RegisterResponse{User: auth.PublicUser{
		ID: user.ID, Role: user.Role, FirstName: user.FirstName, LastName: user.LastName, Name: user.FullName(), Email: user.Email, Status: user.Status,
	}})
}

// Create godoc
//
//	@Summary		Create the tour company
//	@Description	An operator owner creates one company. A second company is rejected.
//	@Tags			companies
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CreateRequest	true	"Company"
//	@Success		201		{object}	Company
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		409		{object}	middleware.ErrorResponse
//	@Router			/v1/companies [post]
func (h *Controller) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	created, err := h.svc.Create(c.Request.Context(), middleware.UserID(c), middleware.Role(c), CreateInput{
		Name: req.Name, Country: req.Country, Association: req.Association,
	})
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// Mine godoc
//
//	@Summary		Current company
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	Company
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		404	{object}	middleware.ErrorResponse
//	@Router			/v1/companies/mine [get]
func (h *Controller) Mine(c *gin.Context) {
	current, err := h.svc.Mine(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, current)
}

// Invite godoc
//
//	@Summary		Invite a guide or driver
//	@Description	Emails an invite. The response does not include the token. Guides and drivers cannot self-register.
//	@Tags			companies
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		InviteRequest	true	"Invite"
//	@Success		201		{object}	InviteResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/companies/invites [post]
func (h *Controller) Invite(c *gin.Context) {
	var req InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	invited, err := h.svc.Invite(c.Request.Context(), middleware.UserID(c), middleware.Role(c), InviteInput{
		Name: req.Name, Email: req.Email, Phone: req.Phone, Role: req.Role,
	})
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, invited)
}

// LookupWallet godoc
//
//	@Summary		Find a wallet by username
//	@Description	Returns the name, currency, and kind a sender sees before money moves. It does not include the balance.
//	@Tags			wallets
//	@Produce		json
//	@Security		BearerAuth
//	@Param			username	query		string	true	"Wallet username"
//	@Success		200			{object}	WalletLookup
//	@Failure		401			{object}	middleware.ErrorResponse
//	@Failure		403			{object}	middleware.ErrorResponse
//	@Failure		404			{object}	middleware.ErrorResponse
//	@Router			/v1/wallets/lookup [get]
func (h *Controller) LookupWallet(c *gin.Context) {
	found, err := h.svc.LookupWallet(c.Request.Context(), middleware.Role(c), c.Query("username"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, found)
}

// Crew godoc
//
//	@Summary		Guides and drivers
//	@Description	Lists the signed-in operator's crew and each person's duty wallet.
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	CrewList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/companies/crew [get]
func (h *Controller) Crew(c *gin.Context) {
	list, err := h.svc.ListCrew(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, CrewList{Crew: list})
}

// ListPersonWallets godoc
//
//	@Summary		Person wallets
//	@Description	Lists tourist, guide, and driver wallets for staff.
//	@Tags			wallets
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	PersonWalletList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/person-wallets [get]
func (h *Controller) ListPersonWallets(c *gin.Context) {
	list, err := h.svc.ListPersonWallets(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, PersonWalletList{Wallets: list})
}

// ListTourists godoc
//
//	@Summary		Tourist accounts
//	@Description	Lists tourists an operator can attach to an itinerary.
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	TouristList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/tourists [get]
func (h *Controller) ListTourists(c *gin.Context) {
	list, err := h.svc.ListTourists(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, TouristList{Tourists: list})
}

// ListBusinesses godoc
//
//	@Summary		List businesses
//	@Description	Lists operator companies and forex bureaus for staff.
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	BusinessList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/businesses [get]
func (h *Controller) ListBusinesses(c *gin.Context) {
	list, err := h.svc.ListBusinesses(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// Approve godoc
//
//	@Summary		Approve a business
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Business id"
//	@Success		200	{object}	Business
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Failure		404	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/businesses/{id}/approve [post]
func (h *Controller) Approve(c *gin.Context) {
	business, err := h.svc.Approve(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, business)
}

// Disable godoc
//
//	@Summary		Disable a business
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Business id"
//	@Success		200	{object}	Business
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Failure		404	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/businesses/{id}/disable [post]
func (h *Controller) Disable(c *gin.Context) {
	business, err := h.svc.Disable(c.Request.Context(), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, business)
}

// OpenWallet godoc
//
//	@Summary		Open a USD wallet
//	@Description	Staff open a USD wallet on an approved business. UGX wallets are created with the business.
//	@Tags			wallets
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string			true	"Business id"
//	@Param			body	body		WalletRequest	true	"Currency"
//	@Success		201		{object}	Business
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		404		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/businesses/{id}/wallets [post]
func (h *Controller) OpenWallet(c *gin.Context) {
	var req WalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	business, err := h.svc.OpenWallet(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"), req.Currency)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, business)
}

// SetDefaultWallet godoc
//
//	@Summary		Choose the default wallet
//	@Tags			wallets
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		string	true	"Business id"
//	@Param			walletId	path		string	true	"Wallet id"
//	@Success		200			{object}	Business
//	@Failure		401			{object}	middleware.ErrorResponse
//	@Failure		403			{object}	middleware.ErrorResponse
//	@Failure		404			{object}	middleware.ErrorResponse
//	@Router			/admin/v1/businesses/{id}/wallets/{walletId}/default [post]
func (h *Controller) SetDefaultWallet(c *gin.Context) {
	business, err := h.svc.SetDefaultWallet(c.Request.Context(), middleware.Role(c), c.Param("id"), c.Param("walletId"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, business)
}

// Tariff godoc
//
//	@Summary		Today's customer rate
//	@Description	Returns the USD/UGX customer rate for the signed-in operator.
//	@Tags			tariffs
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	TariffList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/tariff [get]
func (h *Controller) Tariff(c *gin.Context) {
	h.tariff(c)
}

// StaffTariff godoc
//
//	@Summary		Today's customer rate for staff
//	@Tags			tariffs
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	TariffList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/tariff [get]
func (h *Controller) StaffTariff(c *gin.Context) {
	h.tariff(c)
}

func (h *Controller) tariff(c *gin.Context) {
	tariffs, err := h.svc.TodayTariffs(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, TariffList{Tariffs: tariffs})
}

// SetTariff godoc
//
//	@Summary		Set today's USD/UGX rate
//	@Tags			tariffs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		TariffRequest	true	"Rate"
//	@Success		200		{object}	Tariff
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/tariff [put]
func (h *Controller) SetTariff(c *gin.Context) {
	var req TariffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	tariff, err := h.svc.SetTariff(c.Request.Context(), middleware.UserID(c), middleware.Role(c), req.Pair, req.IntegratorRate, req.MarkupBps)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, tariff)
}

// RegisterForexBureau godoc
//
//	@Summary		Register a forex bureau owner
//	@Description	Creates an unverified bureau owner and emails a verification link. The response has no session token.
//	@Tags			companies
//	@Accept			json
//	@Produce		json
//	@Param			body	body		auth.SignupRequest		true	"Bureau owner"
//	@Success		201		{object}	auth.RegisterResponse	"Created bureau owner"
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		409		{object}	middleware.ErrorResponse
//	@Router			/v1/bureaus [post]
func (h *Controller) RegisterForexBureau(c *gin.Context) {
	var req auth.SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	user, err := h.svc.RegisterForexBureau(c.Request.Context(), auth.Signup{
		FirstName: req.FirstName, LastName: req.LastName, Email: req.Email, Phone: req.Phone, Password: req.Password,
	})
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, auth.RegisterResponse{User: auth.PublicUser{
		ID: user.ID, Role: user.Role, FirstName: user.FirstName, LastName: user.LastName, Name: user.FullName(), Email: user.Email, Status: user.Status,
	}})
}

// AddBureau godoc
//
//	@Summary		Add a forex bureau
//	@Description	Staff create a bureau that is already approved.
//	@Tags			companies
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BureauRequest	true	"Bureau"
//	@Success		201		{object}	Business
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		409		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/bureaus [post]
func (h *Controller) AddBureau(c *gin.Context) {
	var req BureauRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	created, err := h.svc.AddBureau(c.Request.Context(), middleware.Role(c), req.Name, req.Country, req.Owner, req.Email, req.Phone)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// ListBureaus godoc
//
//	@Summary		List forex bureaus
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	BusinessList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/bureaus [get]
func (h *Controller) ListBureaus(c *gin.Context) {
	list, err := h.svc.ListBureaus(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// ListFees godoc
//
//	@Summary		List fee schedules
//	@Tags			tariffs
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	FeeList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/fees [get]
func (h *Controller) ListFees(c *gin.Context) {
	schedules, err := h.svc.ListFeeSchedules(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, FeeList{Fees: schedules})
}

// SaveFee godoc
//
//	@Summary		Save a fee schedule
//	@Tags			tariffs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		FeeScheduleRequest	true	"Fee"
//	@Success		200		{object}	FeeSchedule
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/fees [put]
func (h *Controller) SaveFee(c *gin.Context) {
	var req FeeScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	schedule, err := h.svc.SaveFeeSchedule(c.Request.Context(), middleware.UserID(c), middleware.Role(c), FeeSchedule{
		Kind: req.Kind, Currency: req.Currency, Flat: req.Flat, MarkupBps: req.MarkupBps,
	})
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, schedule)
}

// PlatformWallets godoc
//
//	@Summary		Platform wallets
//	@Description	Lists the VirungaPay operating wallets staff use for fees and float.
//	@Tags			wallets
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	WalletList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/platform-wallets [get]
func (h *Controller) PlatformWallets(c *gin.Context) {
	wallets, err := h.svc.ListPlatformWallets(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, WalletList{Wallets: wallets})
}

// PlatformStatement godoc
//
//	@Summary		Platform wallet statement
//	@Tags			wallets
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Wallet id"
//	@Success		200	{object}	StatementView
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Failure		404	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/platform-wallets/{id}/statement [get]
func (h *Controller) PlatformStatement(c *gin.Context) {
	wallet, lines, err := h.svc.PlatformStatement(c.Request.Context(), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	if lines == nil {
		lines = []StatementLine{}
	}
	c.JSON(http.StatusOK, StatementView{Wallet: wallet, Movements: lines})
}

// MoveWallets godoc
//
//	@Summary		Move money between wallets
//	@Description	Exchanges between the signed-in company's UGX and USD wallets at today's customer rate.
//	@Tags			wallets
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		MoveRequest	true	"Move"
//	@Success		200		{object}	Company
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/wallets/moves [post]
func (h *Controller) MoveWallets(c *gin.Context) {
	var req MoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	company, err := h.svc.MoveWallets(c.Request.Context(), middleware.UserID(c), middleware.Role(c), req.FromWalletID, req.ToWalletID, req.Amount)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, company)
}

// RenameWallet godoc
//
//	@Summary		Rename a wallet
//	@Description	Updates the label and the username senders use to find the wallet.
//	@Tags			wallets
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"Wallet id"
//	@Param			body	body		RenameWalletRequest	true	"Name"
//	@Success		200		{object}	Wallet
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		404		{object}	middleware.ErrorResponse
//	@Failure		409		{object}	middleware.ErrorResponse
//	@Router			/v1/wallets/{id} [patch]
func (h *Controller) RenameWallet(c *gin.Context) {
	var req RenameWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	wallet, err := h.svc.RenameWallet(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"), req.Name, req.Username)
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, wallet)
}

// Transactions godoc
//
//	@Summary		All transactions
//	@Description	Lists recorded payments for staff.
//	@Tags			payments
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	TransactionList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/admin/v1/transactions [get]
func (h *Controller) Transactions(c *gin.Context) {
	list, err := h.svc.ListTransactions(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, TransactionList{Transactions: list})
}

// OperatorPayments godoc
//
//	@Summary		Operator payments
//	@Description	Lists money in and out of the signed-in operator's wallets.
//	@Tags			payments
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	PaymentList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/transactions [get]
func (h *Controller) OperatorPayments(c *gin.Context) {
	list, err := h.svc.ListOperatorPayments(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, PaymentList{Payments: list})
}

// WalletStatement godoc
//
//	@Summary		My wallet statement
//	@Description	Returns the signed-in person's wallet and its movements. Pass format=pdf to download a PDF instead of JSON.
//	@Tags			wallets
//	@Produce		json
//	@Produce		application/pdf
//	@Security		BearerAuth
//	@Param			format	query		string	false	"pdf"	Enums(pdf)
//	@Success		200		{object}	StatementView
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Router			/v1/wallets/mine/statement [get]
func (h *Controller) WalletStatement(c *gin.Context) {
	wallet, lines, err := h.svc.WalletStatement(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	if c.Query("format") == "pdf" {
		rows := make([]mail.StatementRow, 0, len(lines))
		for _, line := range lines {
			row := mail.StatementRow{When: line.CreatedAt.UTC().Format("2 Jan 2006 15:04"), Kind: statementKind(line.Kind, line.Direction), Balance: line.RunningBalance}
			if line.Direction == "out" {
				row.Out = line.Amount
			} else {
				row.In = line.Amount
			}
			rows = append(rows, row)
		}
		name := wallet.Name
		if name == "" {
			name = "VirungaPass Wallet"
		}
		pdf := mail.StatementPDF(mail.Statement{
			Title: name, Currency: wallet.Currency, Balance: wallet.Balance,
			Generated: time.Now().UTC().Format("2 Jan 2006 15:04 UTC"), Rows: rows,
		})
		c.Header("Content-Disposition", `attachment; filename="VirungaPass-statement.pdf"`)
		c.Data(http.StatusOK, "application/pdf", pdf)
		return
	}
	c.JSON(http.StatusOK, StatementView{Wallet: wallet, Movements: lines})
}

// Statement godoc
//
//	@Summary		Download a wallet statement
//	@Description	Downloads a PDF for one wallet the caller owns. format must be pdf.
//	@Tags			wallets
//	@Produce		application/pdf
//	@Security		BearerAuth
//	@Param			id		path		string	true	"Wallet id"
//	@Param			format	query		string	true	"pdf"	Enums(pdf)
//	@Success		200		{string}	string	"PDF statement"
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		404		{object}	middleware.ErrorResponse
//	@Router			/v1/wallets/{id}/statement [get]
func (h *Controller) Statement(c *gin.Context) {
	if c.Query("format") != "pdf" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "choose a PDF statement"}})
		return
	}
	wallet, lines, err := h.svc.Statement(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	rows := make([]mail.StatementRow, 0, len(lines))
	for _, line := range lines {
		row := mail.StatementRow{When: line.CreatedAt.UTC().Format("2 Jan 2006 15:04"), Kind: statementKind(line.Kind, line.Direction), Balance: line.RunningBalance}
		if line.Direction == "out" {
			row.Out = line.Amount
		} else {
			row.In = line.Amount
		}
		rows = append(rows, row)
	}
	name := wallet.Name
	if name == "" {
		name = wallet.Currency + " wallet"
	}
	pdf := mail.StatementPDF(mail.Statement{
		Title: name, Currency: wallet.Currency, Balance: wallet.Balance,
		Generated: time.Now().UTC().Format("2 Jan 2006 15:04 UTC"), Rows: rows,
	})
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-statement.pdf"`, wallet.Currency))
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// CompanyStatement godoc
//
//	@Summary		Download the company statement
//	@Description	Downloads one PDF covering each of the signed-in operator's wallets.
//	@Tags			wallets
//	@Produce		application/pdf
//	@Security		BearerAuth
//	@Success		200	{string}	string	"PDF statement"
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Failure		404	{object}	middleware.ErrorResponse
//	@Router			/v1/companies/mine/statement [get]
func (h *Controller) CompanyStatement(c *gin.Context) {
	name, wallets, groups, err := h.svc.CompanyStatement(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	sections := make([]mail.Statement, 0, len(wallets))
	for i, wallet := range wallets {
		rows := make([]mail.StatementRow, 0, len(groups[i]))
		for _, line := range groups[i] {
			row := mail.StatementRow{When: line.CreatedAt.UTC().Format("2 Jan 2006 15:04"), Kind: statementKind(line.Kind, line.Direction), Balance: line.RunningBalance}
			if line.Direction == "out" {
				row.Out = line.Amount
			} else {
				row.In = line.Amount
			}
			rows = append(rows, row)
		}
		title := wallet.Name
		if title == "" {
			title = wallet.Currency + " wallet"
		}
		sections = append(sections, mail.Statement{Title: title, Currency: wallet.Currency, Balance: wallet.Balance, Rows: rows})
	}
	pdf := mail.WalletsStatementPDF(name, time.Now().UTC().Format("2 Jan 2006 15:04 UTC"), sections)
	c.Header("Content-Disposition", `attachment; filename="VirungaPay-statement.pdf"`)
	c.Data(http.StatusOK, "application/pdf", pdf)
}

func statementKind(kind, direction string) string {
	switch kind {
	case "collection":
		return "Payment in"
	case "withdraw":
		return "Withdrawal"
	case "p2p":
		if direction == "in" {
			return "Tip in"
		}
		return "Tip out"
	case "conversion":
		return "Exchange"
	default:
		if direction == "out" {
			return "Money out"
		}
		return "Money in"
	}
}

// MyWallets godoc
//
//	@Summary		My wallets
//	@Tags			wallets
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	WalletList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/wallets/mine [get]
func (h *Controller) MyWallets(c *gin.Context) {
	wallets, err := h.svc.MyWallets(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, WalletList{Wallets: wallets})
}

// ApprovedBureaus godoc
//
//	@Summary		Approved forex bureaus
//	@Description	Lists bureaus a person can withdraw to.
//	@Tags			companies
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	BusinessList
//	@Failure		401	{object}	middleware.ErrorResponse
//	@Failure		403	{object}	middleware.ErrorResponse
//	@Router			/v1/bureaus [get]
func (h *Controller) ApprovedBureaus(c *gin.Context) {
	list, err := h.svc.ApprovedBureaus(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// WithdrawOwn godoc
//
//	@Summary		Withdraw to a forex bureau
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		OwnWithdrawalRequest	true	"Withdrawal"
//	@Success		200		{object}	Recorded
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/withdrawals [post]
func (h *Controller) WithdrawOwn(c *gin.Context) {
	var req OwnWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if err := h.svc.WithdrawOwn(c.Request.Context(), middleware.UserID(c), middleware.Role(c), req.BureauID, req.Currency, req.Amount); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, Recorded{Status: "recorded"})
}

// Collect godoc
//
//	@Summary		Record a card collection
//	@Description	Staff record a USD card payment collected for an operator.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CollectionRequest	true	"Collection"
//	@Success		200		{object}	Recorded
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/collections [post]
func (h *Controller) Collect(c *gin.Context) {
	var req CollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if err := h.svc.RecordCollection(c.Request.Context(), middleware.Role(c), req.BusinessID, req.Currency, req.Amount); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, Recorded{Status: "recorded"})
}

// Withdraw godoc
//
//	@Summary		Record a withdrawal
//	@Description	Staff move money from an operator wallet to a forex bureau.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		WithdrawalRequest	true	"Withdrawal"
//	@Success		200		{object}	Recorded
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/withdrawals [post]
func (h *Controller) Withdraw(c *gin.Context) {
	var req WithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if err := h.svc.RecordWithdrawal(c.Request.Context(), middleware.Role(c), req.BusinessID, req.BureauID, req.Currency, req.Amount); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, Recorded{Status: "recorded"})
}

// TopUp godoc
//
//	@Summary		Top up from a card
//	@Description	Adds USD to the signed-in tourist's VirungaPass Wallet. Only the last four digits of the card are sent.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		TopUpRequest	true	"Top up"
//	@Success		200		{object}	Recorded
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/wallets/topup [post]
func (h *Controller) TopUp(c *gin.Context) {
	var req TopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if err := h.svc.TopUpCard(c.Request.Context(), middleware.UserID(c), middleware.Role(c), req.Amount, req.Last4); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, Recorded{Status: "recorded"})
}

// MobileMoney godoc
//
//	@Summary		Send mobile money
//	@Description	Sends UGX from the signed-in tourist's wallet to Airtel or MTN in Uganda.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		MobileMoneyRequest	true	"Mobile money"
//	@Success		200		{object}	Recorded
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/mobile-money [post]
func (h *Controller) MobileMoney(c *gin.Context) {
	var req MobileMoneyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if err := h.svc.SendMobileMoney(c.Request.Context(), middleware.UserID(c), middleware.Role(c), req.Network, req.Phone, req.Amount); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, Recorded{Status: "recorded"})
}

// SendTip godoc
//
//	@Summary		Send money to a username
//	@Description	Moves money from the signed-in tourist to a guide, driver, or operator wallet.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		OwnTipRequest	true	"Payment"
//	@Success		200		{object}	Recorded
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		404		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/v1/tips [post]
func (h *Controller) SendTip(c *gin.Context) {
	var req OwnTipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if err := h.svc.TipCrew(c.Request.Context(), middleware.UserID(c), middleware.Role(c), req.Username, req.Amount, req.Purpose); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, Recorded{Status: "recorded"})
}

// Tip godoc
//
//	@Summary		Record a tip
//	@Description	Staff move money from a tourist wallet to a guide or driver.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		TipRequest	true	"Tip"
//	@Success		200		{object}	Recorded
//	@Failure		400		{object}	middleware.ErrorResponse
//	@Failure		401		{object}	middleware.ErrorResponse
//	@Failure		403		{object}	middleware.ErrorResponse
//	@Failure		422		{object}	middleware.ErrorResponse
//	@Router			/admin/v1/tips [post]
func (h *Controller) Tip(c *gin.Context) {
	var req TipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.Invalid("invalid_request", "request body is invalid"))
		return
	}
	if err := h.svc.RecordTip(c.Request.Context(), middleware.Role(c), req.FromUserID, req.ToUserID, req.Currency, req.Amount); err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, Recorded{Status: "recorded"})
}
