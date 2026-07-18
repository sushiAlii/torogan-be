package main

import (
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/vanguard"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/sushiAlii/torogan-be/gen/addressv1/addressv1connect"
	"github.com/sushiAlii/torogan-be/gen/authv1/authv1connect"
	"github.com/sushiAlii/torogan-be/gen/featurev1/featurev1connect"
	"github.com/sushiAlii/torogan-be/gen/propertyv1/propertyv1connect"
	"github.com/sushiAlii/torogan-be/internal/database"
	"github.com/sushiAlii/torogan-be/pkg/handlers"
	"github.com/sushiAlii/torogan-be/pkg/services"

	utils "github.com/sushiAlii/torogan-be/pkg"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

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
	authPath, authHandler := authv1connect.NewAuthServiceHandler(ah)
	authVS := vanguard.NewService(authPath, authHandler)

	// Property Service
	ps := services.NewPropertyService(db)
	ph := handlers.NewPropertiesHandler(ps)
	propertyPath, propertyHandler := propertyv1connect.NewPropertyServiceHandler(ph)
	propertyVS := vanguard.NewService(propertyPath, propertyHandler)

	// Feature Service
	fs := services.NewFeatureService(db)
	fh := handlers.NewFeaturesHandler(fs)
	featurePath, featureHandler := featurev1connect.NewFeatureServiceHandler(fh)
	featureVS := vanguard.NewService(featurePath, featureHandler)

	// Address Service
	addrs := services.NewAddressService(db)
	addrh := handlers.NewAddressesHandler(addrs)
	addressPath, addressHandler := addressv1connect.NewAddressServiceHandler(addrh)
	addressVS := vanguard.NewService(addressPath, addressHandler)

	gateway, err := vanguard.NewTranscoder([]*vanguard.Service{authVS, propertyVS, featureVS, addressVS})
	if err != nil {
		log.Fatalf("Failed to create vanguard gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", gateway)

	port := utils.GetEnv("PORT", "8080")
	serverAddr := fmt.Sprintf(":%s", port)
	log.Printf("🚀 Torogan API engine online and listening on port %s", port)

	err = http.ListenAndServe(
		serverAddr,
		h2c.NewHandler(mux, &http2.Server{}),
	)

	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	select {}
}
