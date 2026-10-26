package pdf

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	p.SetTitle("Factura "+snapshot.Number, true)
	p.SetAuthor(snapshot.Seller.Name, true)
	p.AddPage()

	translate := p.UnicodeTranslatorFromDescriptor("cp1252")
	write := func(text string) string { return translate(text) }

	p.SetFont("Arial", "B", 22)
	p.SetTextColor(31, 41, 55)
	p.CellFormat(95, 12, write("FACTURA"), "", 0, "L", false, 0, "")
	p.SetFont("Arial", "B", 12)
	p.CellFormat(85, 12, write(snapshot.Number), "", 1, "R", false, 0, "")

	p.SetFont("Arial", "B", 10)
	p.CellFormat(90, 6, write(snapshot.Seller.Name), "", 0, "L", false, 0, "")
	p.SetFont("Arial", "", 10)
	p.CellFormat(90, 6, write("Fecha de emisión: "+formatColombianDate(snapshot.IssueDate)), "", 1, "R", false, 0, "")
	p.SetFont("Arial", "", 9)
	writeAddress(p, write, snapshot.Seller.Address)
	if snapshot.Seller.TaxID != "" {
		p.CellFormat(90, 5, write("NIT: "+snapshot.Seller.TaxID), "", 0, "L", false, 0, "")
	}
	if snapshot.Seller.Email != "" {
		p.CellFormat(90, 5, write(snapshot.Seller.Email), "", 1, "L", false, 0, "")
	} else if snapshot.Seller.TaxID != "" {
		p.Ln(5)
	}
	if snapshot.Seller.Phone != "" {
		p.CellFormat(90, 5, write("Teléfono: "+snapshot.Seller.Phone), "", 0, "L", false, 0, "")
	}
	if !snapshot.DueDate.IsZero() {
		p.CellFormat(90, 5, write("Fecha de vencimiento: "+formatColombianDate(snapshot.DueDate)), "", 1, "R", false, 0, "")
	} else {
		p.Ln(5)
	}

	p.Ln(8)
	p.SetDrawColor(209, 213, 219)
	y := p.GetY()
	p.Line(15, y, 195, y)
	p.Ln(5)

	p.SetFont("Arial", "B", 10)
	p.CellFormat(90, 6, write("CLIENTE / ADQUIRENTE"), "", 1, "L", false, 0, "")
	p.SetFont("Arial", "", 10)
	p.CellFormat(90, 5, write(snapshot.Customer.Name), "", 1, "L", false, 0, "")
	if snapshot.Customer.TaxID != "" {
		p.CellFormat(90, 5, write("NIT / documento: "+snapshot.Customer.TaxID), "", 1, "L", false, 0, "")
	}
	if snapshot.Customer.Email != "" {
		p.CellFormat(90, 5, write(snapshot.Customer.Email), "", 1, "L", false, 0, "")
	}
	if snapshot.Customer.Phone != "" {
		p.CellFormat(90, 5, write("Teléfono: "+snapshot.Customer.Phone), "", 1, "L", false, 0, "")
	}
	writeAddress(p, write, snapshot.Customer.Address)
	p.Ln(8)

	columns := []struct {
		title string
		width float64
		align string
	}{
		{title: "Concepto", width: 58, align: "L"},
		{title: "Cant.", width: 12, align: "R"},
		{title: "Valor unitario", width: 28, align: "R"},
		{title: "Dto. (%)", width: 25, align: "R"},
		{title: "Imp. (%)", width: 22, align: "R"},
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
			formatColombianMoney(item.UnitPrice, snapshot.Currency),
			formatBasisPoints(item.DiscountBPS),
			formatBasisPoints(item.TaxRateBPS),
			formatColombianMoney(item.Total, snapshot.Currency),
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
		p.CellFormat(45, 7, write(formatColombianMoney(amount.Amount, amount.Currency)), "", 1, "R", false, 0, "")
	}
	writeTotal("Subtotal", valueobject.Money{Amount: snapshot.Subtotal, Currency: snapshot.Currency}, false)
	writeTotal("Impuestos estimados", valueobject.Money{Amount: snapshot.TaxTotal, Currency: snapshot.Currency}, false)
	p.SetDrawColor(107, 114, 128)
	p.Line(150, p.GetY(), 195, p.GetY())
	p.Ln(2)
	writeTotal("Total a pagar", valueobject.Money{Amount: snapshot.Total, Currency: snapshot.Currency}, true)

	if strings.TrimSpace(snapshot.Notes) != "" {
		p.Ln(8)
		p.SetFont("Arial", "B", 9)
		p.CellFormat(180, 6, write("Observaciones"), "", 1, "L", false, 0, "")
		p.SetFont("Arial", "", 9)
		p.MultiCell(180, 5, write(snapshot.Notes), "", "L", false)
	}

	p.Ln(8)
	p.SetTextColor(107, 114, 128)
	p.SetFont("Arial", "", 8)
	p.MultiCell(180, 4, write("Documento de gestión interno. No ha sido transmitido ni validado ante la DIAN. Los impuestos son valores de cálculo genéricos."), "", "L", false)

	var output bytes.Buffer
	if err := p.Output(&output); err != nil {
		return nil, fmt.Errorf("write PDF: %w", err)
	}
	return output.Bytes(), nil
}

func formatColombianDate(value time.Time) string {
	return value.In(time.FixedZone("COT", -5*60*60)).Format("02/01/2006")
}

func formatColombianMoney(amount int64, currency string) string {
	negative := amount < 0
	magnitude := uint64(amount)
	if negative {
		magnitude = uint64(-(amount + 1)) + 1
	}

	whole := strconv.FormatUint(magnitude/100, 10)
	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteRune(digit)
	}

	sign := ""
	if negative {
		sign = "-"
	}
	return fmt.Sprintf("%s %s%s,%02d", strings.ToUpper(currency), sign, grouped.String(), magnitude%100)
}

func writeAddress(p *gofpdf.Fpdf, translate func(string) string, address valueobject.Address) {
	lines := []string{address.Line1, address.Line2}
	location := make([]string, 0, 3)
	if strings.TrimSpace(address.City) != "" {
		location = append(location, strings.TrimSpace(address.City))
	}
	if strings.TrimSpace(address.State) != "" {
		location = append(location, strings.TrimSpace(address.State))
	}
	if strings.TrimSpace(address.Postcode) != "" {
		location = append(location, "C.P. "+strings.TrimSpace(address.Postcode))
	}
	if len(location) > 0 {
		lines = append(lines, strings.Join(location, ", "))
	}
	if address.Country != "" {
		country := address.Country
		if strings.EqualFold(strings.TrimSpace(country), "CO") {
			country = "Colombia"
		}
		lines = append(lines, country)
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
	return fmt.Sprintf("%s%d,%02d %%", sign, bps/100, bps%100)
}

var _ pdfapp.Renderer = (*GoFPDFRenderer)(nil)
