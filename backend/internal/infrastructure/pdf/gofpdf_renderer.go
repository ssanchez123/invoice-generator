package pdf

import (
	"bytes"
	"fmt"
	"strings"

	gofpdf "github.com/phpdave11/gofpdf"

	pdfapp "github.com/santiagossaa/invoice-generator/internal/application/pdf"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

type GoFPDFRenderer struct{}

func NewGoFPDFRenderer() *GoFPDFRenderer {
	return &GoFPDFRenderer{}
}

func (r *GoFPDFRenderer) Render(doc pdfapp.Document) ([]byte, error) {
	if doc.Invoice == nil || doc.Customer == nil || doc.Tenant == nil {
		return nil, fmt.Errorf("invoice, customer, and tenant are required")
	}

	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(15, 15, 15)
	p.SetAutoPageBreak(true, 18)
	p.SetTitle("Invoice "+doc.Invoice.Number.String(), true)
	p.SetAuthor(doc.Tenant.Name, true)
	p.AddPage()

	translate := p.UnicodeTranslatorFromDescriptor("cp1252")
	write := func(text string) string { return translate(text) }

	p.SetFont("Arial", "B", 22)
	p.SetTextColor(31, 41, 55)
	p.CellFormat(95, 12, write("INVOICE"), "", 0, "L", false, 0, "")
	p.SetFont("Arial", "B", 12)
	p.CellFormat(85, 12, write(doc.Invoice.Number.String()), "", 1, "R", false, 0, "")

	p.SetFont("Arial", "B", 10)
	p.CellFormat(90, 6, write(doc.Tenant.Name), "", 0, "L", false, 0, "")
	p.SetFont("Arial", "", 10)
	p.CellFormat(90, 6, write("Issue date: "+doc.Invoice.IssueDate.Format("2006-01-02")), "", 1, "R", false, 0, "")
	p.SetFont("Arial", "", 9)
	writeAddress(p, write, doc.Tenant.Address)
	if doc.Tenant.TaxID != "" {
		p.CellFormat(90, 5, write("Tax ID: "+doc.Tenant.TaxID), "", 0, "L", false, 0, "")
	}
	if !doc.Invoice.DueDate.IsZero() {
		p.CellFormat(90, 5, write("Due date: "+doc.Invoice.DueDate.Format("2006-01-02")), "", 1, "R", false, 0, "")
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
	p.CellFormat(90, 5, write(doc.Customer.Name), "", 1, "L", false, 0, "")
	if doc.Customer.TaxID != "" {
		p.CellFormat(90, 5, write("Tax ID: "+doc.Customer.TaxID), "", 1, "L", false, 0, "")
	}
	if doc.Customer.Email != "" {
		p.CellFormat(90, 5, write(doc.Customer.Email), "", 1, "L", false, 0, "")
	}
	writeAddress(p, write, doc.Customer.Address)
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

	for _, item := range doc.Invoice.Items {
		total, err := item.LineTotal()
		if err != nil {
			return nil, fmt.Errorf("calculate line total: %w", err)
		}

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
			item.UnitPrice.String(),
			formatBasisPoints(item.DiscountBPS),
			formatBasisPoints(item.TaxRateBPS),
			total.String(),
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
	writeTotal("Subtotal", doc.Invoice.Subtotal, false)
	writeTotal("Tax", doc.Invoice.TaxTotal, false)
	p.SetDrawColor(107, 114, 128)
	p.Line(150, p.GetY(), 195, p.GetY())
	p.Ln(2)
	writeTotal("Total due", doc.Invoice.Total, true)

	if strings.TrimSpace(doc.Invoice.Notes) != "" {
		p.Ln(8)
		p.SetFont("Arial", "B", 9)
		p.CellFormat(180, 6, write("Notes"), "", 1, "L", false, 0, "")
		p.SetFont("Arial", "", 9)
		p.MultiCell(180, 5, write(doc.Invoice.Notes), "", "L", false)
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
