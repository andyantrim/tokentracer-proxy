package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/crypto"
	"tokentracer-proxy/pkg/db"
	"tokentracer-proxy/pkg/handler"
	"tokentracer-proxy/pkg/management"
	"tokentracer-proxy/pkg/ratelimit"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Init Auth & Crypto
	auth.Init()
	crypto.Init()

	// Init DB
	if err := db.InitDB(); err != nil {
		fmt.Printf("Failed to init DB: %v\n", err)
		os.Exit(1)
	}
	defer db.CloseDB()

	// Background: Fetch models for all provider keys every 12 hours
	management.StartModelPolling(context.Background())

	// Serve static UI
	fs := http.FileServer(http.Dir("./web"))
	r.Handle("/*", fs)

	r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/dashboard.html")
	})

	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/docs.html")
	})

	// Auth Routes
	r.Post("/auth/signup", auth.SignupHandler)
	r.Post("/auth/login", auth.LoginHandler)

	// Protected Routes
	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)

		// User info and key generation
		r.Get("/auth/me", auth.UserInfoHandler)
		r.Post("/auth/key", auth.GenerateAPIKeyHandler)

		// Management API
		r.Route("/manage", management.RegisterRoutes)

		// The main proxy endpoint - now protected and rate limited
		ps := handler.NewProxyServer(db.Repo)
		r.With(ratelimit.RateLimitMiddleware).Post("/v1/chat/completions", ps.ProxyHandler)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// TODO: add ping to db?
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("health check: write response error: %v", err)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting server on :%s\n", port)
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("Server failed to start: %v", err)
	}
}
