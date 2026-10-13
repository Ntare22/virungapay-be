package mail

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatementPDFNamesTheWalletAndTheMovements(t *testing.T) {
	pdf := StatementPDF(Statement{
		Title: "VirungaPass Wallet", Currency: "UGX", Balance: "800.00", Generated: "13 Oct 2026",
		Rows: []StatementRow{{When: "2 Oct 2026", Kind: "Tip out", Out: "200.00", Balance: "800.00"}},
	})
	text := string(pdf)
	require.True(t, strings.HasPrefix(text, "%PDF-1.4"))
	require.Contains(t, text, "VirungaPass Wallet")
	require.Contains(t, text, "UGX 800.00")
	require.Contains(t, text, "Tip out")
}
