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

func (h *Controller) LookupWallet(c *gin.Context) {
	found, err := h.svc.LookupWallet(c.Request.Context(), middleware.Role(c), c.Query("username"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, found)
}

func (h *Controller) Crew(c *gin.Context) {
	list, err := h.svc.ListCrew(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"crew": list})
}

func (h *Controller) ListPersonWallets(c *gin.Context) {
	list, err := h.svc.ListPersonWallets(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wallets": list})
}

func (h *Controller) ListTourists(c *gin.Context) {
	list, err := h.svc.ListTourists(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tourists": list})
}

func (h *Controller) ListBusinesses(c *gin.Context) {
	list, err := h.svc.ListBusinesses(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Controller) Approve(c *gin.Context) {
	business, err := h.svc.Approve(c.Request.Context(), middleware.UserID(c), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, business)
}

func (h *Controller) Disable(c *gin.Context) {
	business, err := h.svc.Disable(c.Request.Context(), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, business)
}

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

func (h *Controller) SetDefaultWallet(c *gin.Context) {
	business, err := h.svc.SetDefaultWallet(c.Request.Context(), middleware.Role(c), c.Param("id"), c.Param("walletId"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, business)
}

func (h *Controller) Tariff(c *gin.Context) {
	h.tariff(c)
}

func (h *Controller) StaffTariff(c *gin.Context) {
	h.tariff(c)
}

func (h *Controller) tariff(c *gin.Context) {
	tariffs, err := h.svc.TodayTariffs(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tariffs": tariffs})
}

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

func (h *Controller) ListBureaus(c *gin.Context) {
	list, err := h.svc.ListBureaus(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Controller) ListFees(c *gin.Context) {
	schedules, err := h.svc.ListFeeSchedules(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"fees": schedules})
}

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

func (h *Controller) PlatformWallets(c *gin.Context) {
	wallets, err := h.svc.ListPlatformWallets(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wallets": wallets})
}

func (h *Controller) PlatformStatement(c *gin.Context) {
	wallet, lines, err := h.svc.PlatformStatement(c.Request.Context(), middleware.Role(c), c.Param("id"))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	if lines == nil {
		lines = []StatementLine{}
	}
	c.JSON(http.StatusOK, gin.H{"wallet": wallet, "movements": lines})
}

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

func (h *Controller) Transactions(c *gin.Context) {
	list, err := h.svc.ListTransactions(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": list})
}

func (h *Controller) OperatorPayments(c *gin.Context) {
	list, err := h.svc.ListOperatorPayments(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"payments": list})
}

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
	c.JSON(http.StatusOK, gin.H{"wallet": wallet, "movements": lines})
}

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

func (h *Controller) MyWallets(c *gin.Context) {
	wallets, err := h.svc.MyWallets(c.Request.Context(), middleware.UserID(c), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wallets": wallets})
}

func (h *Controller) ApprovedBureaus(c *gin.Context) {
	list, err := h.svc.ApprovedBureaus(c.Request.Context(), middleware.Role(c))
	if err != nil {
		middleware.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

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
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

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
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

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
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

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
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

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
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

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
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

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
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}
