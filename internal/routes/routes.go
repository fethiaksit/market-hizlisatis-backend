package routes

import (
	"database/sql"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"hizlisatis-backend/internal/config"
	"hizlisatis-backend/internal/handlers"
	"hizlisatis-backend/internal/middleware"
)

func Setup(db *sql.DB, cfg config.Config) *gin.Engine {
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authHandler := &handlers.AuthHandler{DB: db, JWTSecret: cfg.JWTSecret}
	productHandler := &handlers.ProductHandler{DB: db}
	saleHandler := &handlers.SaleHandler{DB: db}
	reportHandler := &handlers.ReportHandler{DB: db}

	api := router.Group("/api")
	{
		api.POST("/auth/login", authHandler.Login)

		protected := api.Group("")
		protected.Use(middleware.AuthRequired(cfg.JWTSecret))
		{
			products := protected.Group("/products")
			{
				products.GET("", productHandler.List)
				products.GET("/bestsellers", productHandler.Bestsellers)
				products.GET("/barcode/:barcode", productHandler.GetByBarcode)
				products.GET("/:id", productHandler.GetByID)
				products.POST("", productHandler.Create)
				products.PUT("/:id", productHandler.Update)
				products.DELETE("/:id", productHandler.Delete)
			}

			sales := protected.Group("/sales")
			{
				sales.POST("", saleHandler.Create)
				sales.GET("", saleHandler.List)
				sales.GET("/today", saleHandler.Today)
				sales.GET("/:id", saleHandler.GetByID)
			}

			reports := protected.Group("/reports")
			{
				reports.GET("/daily", reportHandler.Daily)
				reports.GET("/top-products", reportHandler.TopProducts)
			}
		}
	}

	return router
}
