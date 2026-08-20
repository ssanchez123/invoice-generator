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
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// ========== Dependency Wiring ==========
	// Connect to database
	db, err := repository.NewDB(context.Background(), repository.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	log.Println("connected to database")

	// Repositories (adapters)
	invoiceRepo := repository.NewInvoiceRepository(db)
	customerRepo := repository.NewCustomerRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)

	// Application layer handlers (CQRS)
	createInvoiceH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	issueInvoiceH := command.NewIssueInvoiceHandler(invoiceRepo)
	cancelInvoiceH := command.NewCancelInvoiceHandler(invoiceRepo)
	recordPaymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo)

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
	})

	// ========== HTTP Server ==========
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key", "X-Tenant-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Custom middleware
	r.Use(apimiddleware.TenantContext)
	r.Use(apimiddleware.Idempotency)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

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
			r.Post("/login", handler.Login)
			r.Post("/register", handler.Register)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(apimiddleware.JWTAuth(cfg.JWTSecret))

			// Invoices
			r.Post("/invoices", handler.CreateInvoice)
			r.Get("/invoices", handler.ListInvoices)
			r.Get("/invoices/{id}", handler.GetInvoice)
			r.Patch("/invoices/{id}", handler.UpdateInvoice)
			r.Post("/invoices/{id}/issue", handler.IssueInvoice)
			r.Post("/invoices/{id}/cancel", handler.CancelInvoice)
			r.Post("/invoices/{id}/payments", handler.RecordPayment)
			r.Get("/invoices/{id}/payments", handler.ListPayments)
			r.Get("/invoices/{id}/pdf", handler.DownloadPDF)

			// Customers
			r.Post("/customers", handler.CreateCustomer)
			r.Get("/customers", handler.ListCustomers)
			r.Get("/customers/{id}", handler.GetCustomer)

			// Subscriptions
			r.Post("/subscriptions", handler.CreateSubscription)
			r.Get("/subscriptions", handler.ListSubscriptions)
			r.Delete("/subscriptions/{id}", handler.CancelSubscription)

			// Audit
			r.Get("/audit", handler.ListAuditEntries)
		})
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
