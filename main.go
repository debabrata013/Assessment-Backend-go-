package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gobackend/config"
	"gobackend/controllers"
	"gobackend/db"
	"gobackend/routes"
	"gobackend/services"
)

func main() {
	log.Println("Starting Assessment Go Backend service...")

	// Load configuration
	cfg := config.LoadConfig()

	// Connect to MongoDB
	client, err := db.ConnectMongo(cfg)
	if err != nil {
		log.Printf("Warning: Failed to connect to MongoDB: %v\n", err)
		log.Println("Please make sure local MongoDB is running on mongodb://localhost:27017")
	} else {
		defer db.DisconnectMongo()
	}
	_ = client

	// Initialize Services
	emailService := services.NewEmailService(cfg)
	authService := services.NewAuthService(cfg, emailService)
	groqService := services.NewGroqService(cfg)
	assessmentService := services.NewAssessmentService(groqService)

	// Initialize Controllers
	authController := controllers.NewAuthController(authService)
	assessmentController := controllers.NewAssessmentController(assessmentService)

	// Setup Router
	router := routes.SetupRouter(authController, assessmentController, authService)

	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	// Run HTTP server in goroutine
	go func() {
		log.Printf("Server running on http://localhost:%s\n", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to listen and serve: %v\n", err)
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v\n", err)
	}

	log.Println("Server stopped cleanly")
}
