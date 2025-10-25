package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	utils "github.com/sushiAlii/torogan-be/pkg"
)

var DB *gorm.DB

// ConnectDB establishes connection to PostgreSQL database
func ConnectDB() {
	var err error

	// Get database configuration from environment variables
	dbHost := utils.GetEnv("DB_HOST", "postgres")
	dbPort := utils.GetEnv("DB_PORT", "5432")
	dbName := utils.GetEnv("DB_NAME", "torogan_db")
	dbUser := utils.GetEnv("DB_USER", "torogan_user")
	dbPassword := utils.GetEnv("DB_PASSWORD", "torogan_password")
	dbSSLMode := utils.GetEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		dbHost, dbUser, dbPassword, dbName, dbPort, dbSSLMode)

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	log.Println("Successfully connected to PostgreSQL database!")
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
