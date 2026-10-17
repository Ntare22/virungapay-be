package mail

import (
	"fmt"
	"strings"
)

// Statement is a VirungaPass Wallet statement.
type Statement struct {
	Title     string
	Currency  string
	Balance   string
	Generated string
	Rows      []StatementRow
}

// StatementRow is one movement on the statement.
type StatementRow struct {
	When    string
	Kind    string
	In      string
	Out     string
	Balance string
}

// WalletsStatementPDF is one statement covering every wallet of a company.
func WalletsStatementPDF(title, generated string, wallets []Statement) []byte {
	lines := []string{"VirungaPay", title, "Generated " + generated, ""}
	if len(wallets) == 0 {
		lines = append(lines, "No wallets yet.")
		return textPDF(lines)
	}
	for _, wallet := range wallets {
		balance := wallet.Balance
		if balance == "" {
			balance = "0.00"
		}
		lines = append(lines, wallet.Title, wallet.Currency+" "+balance, "When                  Type            In           Out        Balance")
		if len(wallet.Rows) == 0 {
			lines = append(lines, "No transactions yet.", "")
			continue
		}
		for _, row := range wallet.Rows {
			lines = append(lines, fmt.Sprintf("%-20s  %-12s  %10s  %10s  %10s", row.When, row.Kind, row.In, row.Out, row.Balance))
		}
		lines = append(lines, "")
	}
	return textPDF(lines)
}

// StatementPDF is the downloadable wallet statement.
func StatementPDF(data Statement) []byte {
	balance := data.Balance
	if balance == "" {
		balance = "0.00"
	}
	header := []string{
		"VirungaPay",
		data.Title,
		data.Currency + " " + balance,
		"Generated " + data.Generated,
		"",
		"When                  Type            In           Out        Balance",
	}
	body := append([]string{}, header...)
	if len(data.Rows) == 0 {
		body = append(body, "No transactions yet.")
	}
	for _, row := range data.Rows {
		body = append(body, fmt.Sprintf("%-20s  %-12s  %10s  %10s  %10s", row.When, row.Kind, row.In, row.Out, row.Balance))
	}
	return textPDF(body)
}

func textPDF(lines []string) []byte {
	const perPage = 40
	pages := make([][]string, 0, (len(lines)/perPage)+1)
	for start := 0; start < len(lines); start += perPage {
		end := start + perPage
		if end > len(lines) {
			end = len(lines)
		}
		pages = append(pages, lines[start:end])
	}
	if len(pages) == 0 {
		pages = [][]string{{"VirungaPay"}}
	}
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>"}
	kids := make([]string, 0, len(pages))
	fontID := 3 + len(pages)*2
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i*2))
	}
	objects = append(objects, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	for _, page := range pages {
		var stream strings.Builder
		stream.WriteString("BT\n/F1 10 Tf\n40 800 Td\n14 TL\n")
		for _, line := range page {
			stream.WriteString("(")
			stream.WriteString(pdfEscape(line))
			stream.WriteString(") '\n")
		}
		stream.WriteString("ET\n")
		content := stream.String()
		pageID := len(objects) + 1
		contentsID := pageID + 1
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>", contentsID, fontID),
			fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
		)
	}
	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>")
	return pdfFile(objects)
}

func pdfFile(objects []string) []byte {
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
