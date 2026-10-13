package mail

import (
	"fmt"
	"strings"

	_ "embed"

	"github.com/aymerick/raymond"
)

//go:embed templates/receipt.hbs
var receiptTemplate string

// Receipt is the payment receipt emailed with a PDF.
type Receipt struct {
	InvoiceNumber string
	PaidOn        string
	CustomerName  string
	BusinessName  string
	Currency      string
	Amount        string
}

// RenderReceipt fills the receipt email.
func RenderReceipt(data Receipt) (string, error) {
	return raymond.Render(receiptTemplate, map[string]any{
		"invoiceNumber": data.InvoiceNumber, "paidOn": data.PaidOn, "customerName": data.CustomerName,
		"businessName": data.BusinessName, "currency": data.Currency, "amount": data.Amount,
	})
}

// ReceiptPDF is a one-page receipt.
func ReceiptPDF(data Receipt) []byte {
	lines := []string{
		"VirungaPay",
		"Payment processor",
		"hello@virungapay.demo",
		"East Africa",
		"",
		"Receipt " + data.InvoiceNumber,
		"Date paid " + data.PaidOn,
		"Billed to " + data.CustomerName,
		"Received by " + data.BusinessName,
		"Processed by VirungaPay",
		data.Currency + " " + data.Amount,
		"Paid",
		"",
		"This is a simulated card payment.",
	}
	var body strings.Builder
	body.WriteString("BT\n/F1 12 Tf\n50 780 Td\n14 TL\n")
	for _, line := range lines {
		body.WriteString("(")
		body.WriteString(pdfEscape(line))
		body.WriteString(") '\n")
	}
	body.WriteString("ET\n")
	stream := body.String()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var file strings.Builder
	file.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, len(objects))
	for i, object := range objects {
		offsets = append(offsets, file.Len())
		fmt.Fprintf(&file, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := file.Len()
	fmt.Fprintf(&file, "xref\n0 %d\n", len(objects)+1)
	file.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets {
		fmt.Fprintf(&file, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&file, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return []byte(file.String())
}

func pdfEscape(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`)
	return replacer.Replace(value)
}
