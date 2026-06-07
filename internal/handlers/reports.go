package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"hizlisatis-backend/internal/models"
)

type ReportHandler struct {
	DB *sql.DB
}

func (h *ReportHandler) Daily(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT created_at::date::text AS date,
		       COUNT(*) AS sale_count,
		       COALESCE(SUM(total_amount), 0) AS total_amount
		FROM sales
		GROUP BY created_at::date
		ORDER BY created_at::date DESC
		LIMIT 30
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "daily report could not be generated"})
		return
	}
	defer rows.Close()

	report := make([]models.DailyReport, 0)
	for rows.Next() {
		var item models.DailyReport
		if err := rows.Scan(&item.Date, &item.SaleCount, &item.TotalAmount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "daily report row could not be read"})
			return
		}
		report = append(report, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "daily report could not be generated"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (h *ReportHandler) TopProducts(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT product_id,
		       barcode,
		       product_name,
		       COALESCE(SUM(quantity), 0) AS total_quantity,
		       COALESCE(SUM(line_total), 0) AS total_revenue
		FROM sale_items
		GROUP BY product_id, barcode, product_name
		ORDER BY total_quantity DESC, total_revenue DESC
		LIMIT 20
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "top products report could not be generated"})
		return
	}
	defer rows.Close()

	report := make([]models.TopProductReport, 0)
	for rows.Next() {
		var item models.TopProductReport
		if err := rows.Scan(&item.ProductID, &item.Barcode, &item.ProductName, &item.TotalQty, &item.TotalRevenue); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "top products report row could not be read"})
			return
		}
		report = append(report, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "top products report could not be generated"})
		return
	}

	c.JSON(http.StatusOK, report)
}
