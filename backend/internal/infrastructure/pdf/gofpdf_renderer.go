package pdf

import (
	"bytes"
	"fmt"
	"strings"

	gofpdf "github.com/phpdave11/gofpdf"

	pdfapp "github.com/santiagossaa/invoice-generator/internal/application/pdf"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

type GoFPDFRenderer struct{}

func NewGoFPDFRenderer() *GoFPDFRenderer {
	return &GoFPDFRenderer{}
}

func (r *GoFPDFRenderer) Render(doc pdfapp.Document) ([]byte, error) {
	snapshot := doc.Snapshot
	if snapshot.Version != entity.InvoiceSnapshotVersion ||
		snapshot.InvoiceID == "" ||
		snapshot.Number == "" ||
		snapshot.Currency == "" ||
		snapshot.Seller.Name == "" ||
		snapshot.Customer.Name == "" ||
		len(snapshot.Items) == 0 {
		return nil, fmt.Errorf("valid immutable invoice snapshot is required")
	}

	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(15, 15, 15)
	p.SetAutoPageBreak(true, 18)
	p.SetTitle("Invoice "+snapshot.Number, true)
	p.SetAuthor(snapshot.Seller.Name, true)
	p.AddPage()

	translate := p.UnicodeTranslatorFromDescriptor("cp1252")
	write := func(text string) string { return translate(text) }

	p.SetFont("Arial", "B", 22)
	p.SetTextColor(31, 41, 55)
	p.CellFormat(95, 12, write("INVOICE"), "", 0, "L", false, 0, "")
	p.SetFont("Arial", "B", 12)
	p.CellFormat(85, 12, write(snapshot.Number), "", 1, "R", false, 0, "")

	p.SetFont("Arial", "B", 10)
	p.CellFormat(90, 6, write(snapshot.Seller.Name), "", 0, "L", false, 0, "")
	p.SetFont("Arial", "", 10)
	p.CellFormat(90, 6, write("Issue date: "+snapshot.IssueDate.Format("2006-01-02")), "", 1, "R", false, 0, "")
	p.SetFont("Arial", "", 9)
	writeAddress(p, write, snapshot.Seller.Address)
	if snapshot.Seller.TaxID != "" {
		p.CellFormat(90, 5, write("Tax ID: "+snapshot.Seller.TaxID), "", 0, "L", false, 0, "")
	}
	if !snapshot.DueDate.IsZero() {
		p.CellFormat(90, 5, write("Due date: "+snapshot.DueDate.Format("2006-01-02")), "", 1, "R", false, 0, "")
	} else {
		p.Ln(5)
	}

	p.Ln(8)
	p.SetDrawColor(209, 213, 219)
	y := p.GetY()
	p.Line(15, y, 195, y)
	p.Ln(5)

	p.SetFont("Arial", "B", 10)
	p.CellFormat(90, 6, write("Bill to"), "", 1, "L", false, 0, "")
	p.SetFont("Arial", "", 10)
	p.CellFormat(90, 5, write(snapshot.Customer.Name), "", 1, "L", false, 0, "")
	if snapshot.Customer.TaxID != "" {
		p.CellFormat(90, 5, write("Tax ID: "+snapshot.Customer.TaxID), "", 1, "L", false, 0, "")
	}
	if snapshot.Customer.Email != "" {
		p.CellFormat(90, 5, write(snapshot.Customer.Email), "", 1, "L", false, 0, "")
	}
	writeAddress(p, write, snapshot.Customer.Address)
	p.Ln(8)

	columns := []struct {
		title string
		width float64
		align string
	}{
		{title: "Description", width: 58, align: "L"},
		{title: "Qty", width: 12, align: "R"},
		{title: "Unit price", width: 28, align: "R"},
		{title: "Discount", width: 25, align: "R"},
		{title: "Tax", width: 22, align: "R"},
		{title: "Total", width: 35, align: "R"},
	}
	drawTableHeader := func() {
		p.SetFont("Arial", "B", 9)
		p.SetFillColor(243, 244, 246)
		for _, col := range columns {
			p.CellFormat(col.width, 8, write(col.title), "B", 0, col.align, true, 0, "")
		}
		p.Ln(-1)
		p.SetFont("Arial", "", 9)
	}
	drawTableHeader()

	for _, item := range snapshot.Items {
		description := write(item.Description)
		lines := p.SplitLines([]byte(description), columns[0].width-4)
		rowHeight := float64(len(lines)) * 5
		if rowHeight < 7 {
			rowHeight = 7
		}
		_, pageHeight := p.GetPageSize()
		if p.GetY()+rowHeight > pageHeight-18 {
			p.AddPage()
			drawTableHeader()
		}

		y := p.GetY()
		x := 15.0
		p.SetXY(x+2, y+1)
		p.MultiCell(columns[0].width-4, 5, description, "", "L", false)
		x += columns[0].width

		values := []string{
			fmt.Sprintf("%d", item.Quantity),
			valueobject.Money{Amount: item.UnitPrice, Currency: snapshot.Currency}.String(),
			formatBasisPoints(item.DiscountBPS),
			formatBasisPoints(item.TaxRateBPS),
			valueobject.Money{Amount: item.Total, Currency: snapshot.Currency}.String(),
		}
		for i, value := range values {
			p.SetXY(x, y)
			p.CellFormat(columns[i+1].width, rowHeight, write(value), "B", 0, columns[i+1].align, false, 0, "")
			x += columns[i+1].width
		}
		p.SetXY(15, y+rowHeight)
	}

	p.Ln(6)
	writeTotal := func(label string, amount valueobject.Money, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		p.SetFont("Arial", style, 10)
		p.CellFormat(135, 7, write(label), "", 0, "R", false, 0, "")
		p.CellFormat(45, 7, write(amount.String()), "", 1, "R", false, 0, "")
	}
	writeTotal("Subtotal", valueobject.Money{Amount: snapshot.Subtotal, Currency: snapshot.Currency}, false)
	writeTotal("Tax", valueobject.Money{Amount: snapshot.TaxTotal, Currency: snapshot.Currency}, false)
	p.SetDrawColor(107, 114, 128)
	p.Line(150, p.GetY(), 195, p.GetY())
	p.Ln(2)
	writeTotal("Total due", valueobject.Money{Amount: snapshot.Total, Currency: snapshot.Currency}, true)

	if strings.TrimSpace(snapshot.Notes) != "" {
		p.Ln(8)
		p.SetFont("Arial", "B", 9)
		p.CellFormat(180, 6, write("Notes"), "", 1, "L", false, 0, "")
		p.SetFont("Arial", "", 9)
		p.MultiCell(180, 5, write(snapshot.Notes), "", "L", false)
	}

	var output bytes.Buffer
	if err := p.Output(&output); err != nil {
		return nil, fmt.Errorf("write PDF: %w", err)
	}
	return output.Bytes(), nil
}

func writeAddress(p *gofpdf.Fpdf, translate func(string) string, address valueobject.Address) {
	lines := []string{address.Line1, address.Line2}
	cityLine := strings.TrimSpace(strings.Join([]string{address.City, address.State, address.Postcode}, " "))
	if cityLine != "" {
		lines = append(lines, cityLine)
	}
	if address.Country != "" {
		lines = append(lines, address.Country)
	}
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			p.CellFormat(90, 5, translate(line), "", 1, "L", false, 0, "")
		}
	}
}

func formatBasisPoints(bps int64) string {
	sign := ""
	if bps < 0 {
		sign = "-"
		bps = -bps
	}
	return fmt.Sprintf("%s%d.%02d%%", sign, bps/100, bps%100)
}

var _ pdfapp.Renderer = (*GoFPDFRenderer)(nil)
