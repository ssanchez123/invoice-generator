package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/santiagossaa/invoice-generator/internal/api/handler"
	apimiddleware "github.com/santiagossaa/invoice-generator/internal/api/middleware"
	"github.com/santiagossaa/invoice-generator/internal/api/swagger"
	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/application/query"
	"github.com/santiagossaa/invoice-generator/internal/config"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/auth"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// ========== Dependency Wiring ==========
	// Connect to database
	db, err := repository.NewDB(context.Background(), cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	log.Println("connected to database")

	// Repositories (adapters)
	invoiceRepo := repository.NewInvoiceRepository(db)
	customerRepo := repository.NewCustomerRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	userRepo := repository.NewUserRepository(db)
	idempotencyRepo := repository.NewIdempotencyRepository(db)
	pdfStore, err := storage.NewPDFArtifactStoreFromEnv(context.Background())
	if err != nil {
		log.Fatalf("failed to configure PDF storage: %v", err)
	}

	// Auth services
	passwordHasher := auth.NewBcryptPasswordHasher()
	tokenService := auth.NewJWTTokenService(auth.JWTConfig{
		Secret:      cfg.JWTSecret,
		TokenExpiry: cfg.JWTExpiry,
	})

	// Application layer handlers (CQRS)
	createInvoiceH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	issueInvoiceH := command.NewIssueInvoiceHandler(invoiceRepo)
	cancelInvoiceH := command.NewCancelInvoiceHandler(invoiceRepo)
	// Unit of Work for transactional operations
	uow := repository.NewUnitOfWork(db)

	recordPaymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo, uow)

	listInvoicesH := query.NewListInvoicesHandler(invoiceRepo)
	getInvoiceH := query.NewGetInvoiceHandler(invoiceRepo)
	listCustomersH := query.NewListCustomersHandler(customerRepo)
	listPaymentsH := query.NewListPaymentsHandler(paymentRepo, invoiceRepo)

	// Inject dependencies into HTTP handlers
	handler.SetDeps(&handler.HandlerDeps{
		CreateInvoice: createInvoiceH,
		IssueInvoice:  issueInvoiceH,
		CancelInvoice: cancelInvoiceH,
		RecordPayment: recordPaymentH,
		ListInvoices:  listInvoicesH,
		GetInvoice:    getInvoiceH,
		ListCustomers: listCustomersH,
		ListPayments:  listPaymentsH,
		CustomerRepo:  customerRepo,
		PDFStore:      pdfStore,
	})

	// Auth handlers
	loginH := command.NewLoginHandler(userRepo, passwordHasher, tokenService)

	handler.SetAuthDeps(&handler.AuthDeps{
		Login: loginH,
	})

	// ========== HTTP Server ==========
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(apimiddleware.CaptureClientAddress)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Liveness and dependency readiness
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/health/ready", handler.ReadinessHandler(db))

	// Swagger UI — interactive API docs at /swagger/*
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.PersistAuthorization(true),
	))

	// OpenAPI spec endpoints
	r.Get("/swagger/doc.json", swagger.SpecJSONHandler)
	r.Get("/swagger/openapi.yaml", swagger.SpecYAMLHandler)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth (no JWT required)
		r.Route("/auth", func(r chi.Router) {
			r.With(apimiddleware.LoginRateLimit(cfg.LoginRateLimit, cfg.LoginRateWindow)).Post("/login", handler.Login)
			r.Post("/register", handler.RegisterDisabled)
		})

		registerProtectedRoutes(r, tokenService, idempotencyRepo)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("API server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("server stopped")
}

func registerProtectedRoutes(r chi.Router, tokenService port.TokenService, idempotencyRepo port.IdempotencyRepository) {
	readRoles := apimiddleware.RequireAnyRole("admin", "member", "viewer")
	memberRole := apimiddleware.RequireRole("member")

	r.Group(func(r chi.Router) {
		r.Use(apimiddleware.JWTAuth(tokenService))
		r.Use(apimiddleware.TenantContext)

		r.With(memberRole, apimiddleware.Idempotency(idempotencyRepo)).Post("/invoices", handler.CreateInvoice)
		r.With(readRoles).Get("/invoices", handler.ListInvoices)
		r.With(readRoles).Get("/invoices/{id}", handler.GetInvoice)
		r.With(memberRole).Patch("/invoices/{id}", handler.UpdateInvoice)
		r.With(memberRole, apimiddleware.Idempotency(idempotencyRepo)).Post("/invoices/{id}/issue", handler.IssueInvoice)
		r.With(memberRole, apimiddleware.Idempotency(idempotencyRepo)).Post("/invoices/{id}/cancel", handler.CancelInvoice)
		r.With(memberRole, apimiddleware.Idempotency(idempotencyRepo)).Post("/invoices/{id}/payments", handler.RecordPayment)
		r.With(readRoles).Get("/invoices/{id}/payments", handler.ListPayments)
		r.With(readRoles).Get("/invoices/{id}/pdf", handler.DownloadPDF)

		r.With(memberRole, apimiddleware.Idempotency(idempotencyRepo)).Post("/customers", handler.CreateCustomer)
		r.With(readRoles).Get("/customers", handler.ListCustomers)
		r.With(readRoles).Get("/customers/{id}", handler.GetCustomer)

		r.With(memberRole).Post("/subscriptions", handler.CreateSubscription)
		r.With(readRoles).Get("/subscriptions", handler.ListSubscriptions)
		r.With(memberRole).Delete("/subscriptions/{id}", handler.CancelSubscription)

		r.With(apimiddleware.RequireRole("admin")).Get("/audit", handler.ListAuditEntries)
	})
}
