package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/sushiAlii/torogan-be/internal/database"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Connect to database
	database.ConnectDB()

	// Get database instance
	db := database.GetDB()

	// Test database connection
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get database instance:", err)
	}

	// Ping database to test connection
	if err := sqlDB.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("Database connection test successful!")

	// Your application logic goes here
	// For now, we'll just keep the connection alive
	select {}
}
