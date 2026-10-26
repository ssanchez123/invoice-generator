package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/auth"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
	"golang.org/x/term"
)

const (
	minOwnerPasswordBytes = 12
	maxOwnerPasswordBytes = 72
)

type provisionInput struct {
	TenantName string
	TaxID      string
	Address    valueobject.Address
	OwnerEmail string
	FirstName  string
	LastName   string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tenant-ops: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}
	if args[0] != "provision" {
		return fmt.Errorf("unknown command %q; run tenant-ops --help", args[0])
	}

	flags := flag.NewFlagSet("provision", flag.ContinueOnError)
	input := provisionInput{}
	flags.StringVar(&input.TenantName, "tenant-name", "", "business or workspace name")
	flags.StringVar(&input.TaxID, "tax-id", "", "business tax ID")
	flags.StringVar(&input.Address.Line1, "address-line1", "", "business address line")
	flags.StringVar(&input.Address.Line2, "address-line2", "", "additional address details")
	flags.StringVar(&input.Address.City, "city", "", "business city")
	flags.StringVar(&input.Address.State, "state", "", "business department or region")
	flags.StringVar(&input.Address.Postcode, "postcode", "", "business postal code")
	flags.StringVar(&input.Address.Country, "country", "", "two-letter ISO country code")
	flags.StringVar(&input.OwnerEmail, "owner-email", "", "first tenant administrator email")
	flags.StringVar(&input.FirstName, "owner-first-name", "", "first administrator given name")
	flags.StringVar(&input.LastName, "owner-last-name", "", "first administrator family name")
	flags.Usage = printUsage
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if err := validateProvisionInput(&input); err != nil {
		return err
	}

	databaseURL := os.Getenv("DB_OPERATOR_URL")
	if strings.TrimSpace(databaseURL) == "" {
		return errors.New("DB_OPERATOR_URL is required; use a dedicated non-superuser invoicegen_operator login")
	}
	if err := validateOperatorDatabaseURL(databaseURL, os.Getenv("ENVIRONMENT")); err != nil {
		return err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("owner password must be entered in an interactive terminal")
	}

	password, err := readOwnerPassword()
	if err != nil {
		return err
	}
	defer clear(password)
	hasher := auth.NewBcryptPasswordHasher()
	passwordHash, err := hasher.Hash(string(password))
	if err != nil {
		return fmt.Errorf("hash owner password: %w", err)
	}

	addressJSON, err := json.Marshal(input.Address)
	if err != nil {
		return fmt.Errorf("encode business address: %w", err)
	}
	settingsJSON, err := json.Marshal(map[string]any{
		"default_currency": "COP",
		"invoice_prefix":   "FAC",
	})
	if err != nil {
		return fmt.Errorf("encode tenant settings: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := repository.NewDB(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to operator database: %w", err)
	}
	defer db.Close()

	var tenantID, ownerID string
	err = db.Pool().QueryRow(ctx, `
		SELECT created_tenant_id, created_owner_user_id
		FROM public.provision_tenant_with_owner($1, $2, $3::jsonb, $4::jsonb, $5, $6, $7, $8)
	`,
		strings.TrimSpace(input.TenantName),
		strings.TrimSpace(input.TaxID),
		string(addressJSON),
		string(settingsJSON),
		input.OwnerEmail,
		passwordHash,
		strings.TrimSpace(input.FirstName),
		strings.TrimSpace(input.LastName),
	).Scan(&tenantID, &ownerID)
	if err != nil {
		return fmt.Errorf("provision tenant and first owner: %w", err)
	}

	fmt.Printf("Provisioned tenant %s with owner %s.\n", tenantID, ownerID)
	fmt.Printf("The owner signs in with tenant ID %s and email %s.\n", tenantID, input.OwnerEmail)
	return nil
}

func validateProvisionInput(input *provisionInput) error {
	input.TenantName = strings.TrimSpace(input.TenantName)
	input.TaxID = strings.TrimSpace(input.TaxID)
	input.OwnerEmail = strings.ToLower(strings.TrimSpace(input.OwnerEmail))
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Address.Line1 = strings.TrimSpace(input.Address.Line1)
	input.Address.Line2 = strings.TrimSpace(input.Address.Line2)
	input.Address.City = strings.TrimSpace(input.Address.City)
	input.Address.State = strings.TrimSpace(input.Address.State)
	input.Address.Postcode = strings.TrimSpace(input.Address.Postcode)
	input.Address.Country = strings.ToUpper(strings.TrimSpace(input.Address.Country))

	if len(input.TenantName) == 0 || len(input.TenantName) > 200 {
		return errors.New("--tenant-name must contain 1 to 200 characters")
	}
	if len(input.TaxID) > 64 {
		return errors.New("--tax-id must be at most 64 characters")
	}
	if len(input.FirstName) == 0 || len(input.FirstName) > 100 || len(input.LastName) == 0 || len(input.LastName) > 100 {
		return errors.New("--owner-first-name and --owner-last-name must contain 1 to 100 characters")
	}
	address := input.Address
	if err := address.Validate(); err != nil {
		return err
	}
	parsedEmail, err := mail.ParseAddress(input.OwnerEmail)
	if err != nil || parsedEmail.Address != input.OwnerEmail || len(input.OwnerEmail) > 254 {
		return errors.New("--owner-email must be a valid email address without a display name")
	}
	return nil
}

func readOwnerPassword() ([]byte, error) {
	fmt.Fprint(os.Stderr, "Initial owner password (minimum 12 characters): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("read owner password securely: %w", err)
	}
	if len(password) < minOwnerPasswordBytes || len(password) > maxOwnerPasswordBytes {
		clear(password)
		return nil, fmt.Errorf("owner password must contain %d to %d bytes", minOwnerPasswordBytes, maxOwnerPasswordBytes)
	}

	fmt.Fprint(os.Stderr, "Confirm owner password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		clear(password)
		return nil, fmt.Errorf("confirm owner password securely: %w", err)
	}
	defer clear(confirmation)
	if string(password) != string(confirmation) {
		clear(password)
		return nil, errors.New("passwords do not match")
	}
	return password, nil
}

func validateOperatorDatabaseURL(databaseURL, environment string) error {
	if !strings.EqualFold(strings.TrimSpace(environment), "production") {
		return nil
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return errors.New("DB_OPERATOR_URL must be a PostgreSQL URL in production")
	}
	switch strings.ToLower(parsed.Query().Get("sslmode")) {
	case "verify-full", "verify-ca":
		return nil
	default:
		return errors.New("DB_OPERATOR_URL must enable TLS in production")
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage:
  tenant-ops provision --tenant-name NAME --owner-email EMAIL --owner-first-name NAME --owner-last-name NAME \
    --address-line1 TEXT --city CITY --country ISO2 [--tax-id ID] [--address-line2 TEXT] [--state REGION] [--postcode CODE]

Requires DB_OPERATOR_URL for a dedicated non-superuser login granted the invoicegen_operator role.
The first owner password is read twice from an interactive terminal and never accepted as a command argument.
Public self-registration remains disabled.
`)
}
