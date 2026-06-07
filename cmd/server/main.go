package main

import (
	"log"

	"github.com/joho/godotenv"

	"hizlisatis-backend/internal/config"
	"hizlisatis-backend/internal/database"
	"hizlisatis-backend/internal/routes"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using environment variables")
	}

	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	router := routes.Setup(db, cfg)

	log.Printf("server listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
