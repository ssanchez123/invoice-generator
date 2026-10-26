package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "outbox-ops: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}
	if args[0] != "retry-failed" {
		return fmt.Errorf("unknown command %q; run outbox-ops --help", args[0])
	}

	flags := flag.NewFlagSet("retry-failed", flag.ContinueOnError)
	tenantID := flags.String("tenant-id", "", "tenant UUID")
	eventID := flags.String("event-id", "", "outbox event UUID")
	consumer := flags.String("consumer", "", "consumer name")
	reason := flags.String("reason", "", "short operational reason (do not include secrets or personal data)")
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

	parsedTenantID, err := uuid.Parse(*tenantID)
	if err != nil {
		return fmt.Errorf("--tenant-id must be a UUID")
	}
	parsedEventID, err := uuid.Parse(*eventID)
	if err != nil {
		return fmt.Errorf("--event-id must be a UUID")
	}
	if strings.TrimSpace(*consumer) == "" || len(*consumer) > 128 {
		return fmt.Errorf("--consumer must contain 1 to 128 characters")
	}
	if strings.TrimSpace(*reason) == "" || len(*reason) > 500 {
		return fmt.Errorf("--reason must contain 1 to 500 characters")
	}

	databaseURL := os.Getenv("DB_OPERATOR_URL")
	if databaseURL == "" {
		return fmt.Errorf("DB_OPERATOR_URL is required; use a dedicated operator login, not invoicegen_app")
	}
	if err := validateOperatorDatabaseURL(databaseURL, os.Getenv("ENVIRONMENT")); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := repository.NewDB(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to operator database: %w", err)
	}
	defer db.Close()

	tx, err := db.TenantTx(ctx, parsedTenantID.String())
	if err != nil {
		return fmt.Errorf("begin tenant-scoped operation: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		SELECT public.requeue_failed_outbox_delivery($1, $2, $3, $4)
	`, parsedTenantID, parsedEventID, *consumer, *reason); err != nil {
		return fmt.Errorf("requeue failed delivery: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delivery replay: %w", err)
	}

	fmt.Printf("Requeued delivery %s for consumer %s in tenant %s.\n", parsedEventID, *consumer, parsedTenantID)
	return nil
}

func validateOperatorDatabaseURL(databaseURL, environment string) error {
	if strings.EqualFold(strings.TrimSpace(environment), "production") {
		parsed, err := url.Parse(databaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
			return fmt.Errorf("DB_OPERATOR_URL must be a PostgreSQL URL in production")
		}
		switch strings.ToLower(parsed.Query().Get("sslmode")) {
		case "require", "verify-ca", "verify-full":
		default:
			return fmt.Errorf("DB_OPERATOR_URL must enable TLS in production")
		}
	}
	return nil
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage:
  outbox-ops retry-failed --tenant-id UUID --event-id UUID --consumer NAME --reason TEXT

Requires DB_OPERATOR_URL for a unique, non-superuser login granted the invoicegen_operator role.
The database records the authenticated login, reason, and prior attempt count.
`)
}
