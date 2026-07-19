package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"connectrpc.com/vanguard"
	"github.com/joho/godotenv"
	"github.com/rs/cors"

	"github.com/sushiAlii/torogan-be/gen/addressv1/addressv1connect"
	"github.com/sushiAlii/torogan-be/gen/authv1/authv1connect"
	"github.com/sushiAlii/torogan-be/gen/featurev1/featurev1connect"
	"github.com/sushiAlii/torogan-be/gen/propertyv1/propertyv1connect"
	"github.com/sushiAlii/torogan-be/gen/uploadv1/uploadv1connect"
	"github.com/sushiAlii/torogan-be/gen/userv1/userv1connect"
	"github.com/sushiAlii/torogan-be/internal/database"
	"github.com/sushiAlii/torogan-be/pkg/handlers"
	"github.com/sushiAlii/torogan-be/pkg/interceptors"
	"github.com/sushiAlii/torogan-be/pkg/services"

	utils "github.com/sushiAlii/torogan-be/pkg"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	ctx := context.Background()

	database.ConnectDB()

	db := database.GetDB()

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get database instance:", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("Database connection test successful!")

	// Auth Service
	as := services.NewAuthService(db)
	ah := handlers.NewAuthHandler(as)

	// authInterceptor reads the Authorization header on every RPC (across
	// all services below) and, if it carries a valid access token, injects
	// the caller's user ID/role into the request context. It does not
	// reject unauthenticated requests itself — handlers that require auth
	// call interceptors.MustUserID and return CodeUnauthenticated.
	authInterceptor := interceptors.NewAuthInterceptor(as)
	opts := connect.WithInterceptors(authInterceptor)

	authPath, authHandler := authv1connect.NewAuthServiceHandler(ah, opts)
	authVS := vanguard.NewService(authPath, authHandler)

	// Property Service
	ps := services.NewPropertyService(db)
	ph := handlers.NewPropertiesHandler(ps)
	propertyPath, propertyHandler := propertyv1connect.NewPropertyServiceHandler(ph, opts)
	propertyVS := vanguard.NewService(propertyPath, propertyHandler)

	// Feature Service
	fs := services.NewFeatureService(db)
	fh := handlers.NewFeaturesHandler(fs)
	featurePath, featureHandler := featurev1connect.NewFeatureServiceHandler(fh, opts)
	featureVS := vanguard.NewService(featurePath, featureHandler)

	// Address Service
	addrs := services.NewAddressService(db)
	addrh := handlers.NewAddressesHandler(addrs)
	addressPath, addressHandler := addressv1connect.NewAddressServiceHandler(addrh, opts)
	addressVS := vanguard.NewService(addressPath, addressHandler)

	// User Service
	us := services.NewUserService(db)
	uh := handlers.NewUserHandler(us)
	userPath, userHandler := userv1connect.NewUserServiceHandler(uh, opts)
	userVS := vanguard.NewService(userPath, userHandler)

	// Upload Service — requires AWS_REGION/S3_BUCKET (and AWS credentials)
	// to be configured. Skipped (not fatal) when absent, so the rest of the
	// API stays usable in local dev before the S3 bucket is set up.
	registeredServices := []*vanguard.Service{authVS, propertyVS, featureVS, addressVS, userVS}
	uploadSvc, err := services.NewUploadService(ctx)
	if err != nil {
		log.Printf("⚠️  Upload service disabled (%v) — photo upload endpoints will not be reachable until AWS_REGION/S3_BUCKET are configured", err)
	} else {
		uploadh := handlers.NewUploadHandler(uploadSvc)
		uploadPath, uploadHandler := uploadv1connect.NewUploadServiceHandler(uploadh, opts)
		registeredServices = append(registeredServices, vanguard.NewService(uploadPath, uploadHandler))
	}

	gateway, err := vanguard.NewTranscoder(registeredServices)
	if err != nil {
		log.Fatalf("Failed to create vanguard gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	// CORS_ALLOWED_ORIGINS is a comma-separated list (e.g.
	// "https://torogan.com,https://www.torogan.com"). Credentialed
	// requests (the refresh-token cookie) require echoing back the exact
	// matched origin rather than "*", which rs/cors handles for us.
	rawOrigins := utils.GetEnv("CORS_ALLOWED_ORIGINS", "")
	var allowedOrigins []string
	for _, o := range strings.Split(rawOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowedOrigins = append(allowedOrigins, o)
		}
	}

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           3600,
	})

	port := utils.GetEnv("PORT", "8080")
	serverAddr := fmt.Sprintf(":%s", port)
	log.Printf("🚀 Torogan API engine online and listening on port %s", port)

	// Native unencrypted-HTTP/2 support (Go 1.24+) replaces the old
	// golang.org/x/net/http2/h2c.NewHandler wrapper; HTTP1 stays enabled
	// alongside it so plain REST/JSON clients keep working.
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Addr:      serverAddr,
		Handler:   corsMiddleware.Handler(mux),
		Protocols: protocols,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	select {}
}
