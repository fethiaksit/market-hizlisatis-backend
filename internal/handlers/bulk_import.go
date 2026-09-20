package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type bulkImportItem struct {
	Barcode       string  `json:"barcode"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	PurchasePrice float64 `json:"purchasePrice"`
	Stock         float64 `json:"stock"`
	Category      string  `json:"category"`
	Unit          string  `json:"unit"`
	Status        string  `json:"status"`
}

type bulkImportRequest struct {
	Items          []bulkImportItem `json:"items" binding:"required"`
	ExistingAction string           `json:"existingAction" binding:"required"`
	CreatedBy      string           `json:"createdBy"`
}

func (h *ProductHandler) BulkImport(c *gin.Context) {
	var req bulkImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("bulk import: begin transaction failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bulk import could not start"})
		return
	}
	defer tx.Rollback()

	created, updated, skipped, stockAdded := 0, 0, 0, 0

	for _, item := range req.Items {
		if strings.EqualFold(item.Status, "ERROR") {
			skipped++
			continue
		}
		if strings.TrimSpace(item.Barcode) == "" || strings.TrimSpace(item.Name) == "" || item.Price <= 0 {
			skipped++
			continue
		}

		categoryName, err := canonicalCategory(tx, item.Category)
		if err != nil {
			log.Printf("bulk import: category lookup failed barcode=%q category=%q: %v", item.Barcode, item.Category, err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "category not found",
				"category": item.Category,
				"barcode": item.Barcode,
			})
			return
		}

		var productID int64
		err = tx.QueryRow(`SELECT id FROM products WHERE barcode = $1`, strings.TrimSpace(item.Barcode)).Scan(&productID)
		if err == sql.ErrNoRows {
			err = tx.QueryRow(`
				INSERT INTO products
					(name, barcode, sale_price, purchase_price, category, brand, description, image_url, is_bestseller, bestseller_order, is_active)
				VALUES ($1, $2, $3, $4, $5, '', '', '', FALSE, 0, TRUE)
				RETURNING id
			`,
				strings.TrimSpace(item.Name),
				strings.TrimSpace(item.Barcode),
				item.Price,
				item.PurchasePrice,
				categoryName,
			).Scan(&productID)
			if err != nil {
				log.Printf("bulk import: product create failed barcode=%q name=%q category=%q sale_price=%v purchase_price=%v: %v", item.Barcode, item.Name, categoryName, item.Price, item.PurchasePrice, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be created", "barcode": item.Barcode})
				return
			}
			if item.Stock > 0 {
				if _, err := tx.Exec(`
					INSERT INTO stock_movements (product_id, movement_date, type, quantity, note)
					VALUES ($1, CURRENT_DATE, 'in', $2, 'CSV toplu ürün aktarımı')
				`, productID, item.Stock); err != nil {
					log.Printf("bulk import: initial stock insert failed barcode=%q product_id=%d stock=%v: %v", item.Barcode, productID, item.Stock, err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "initial stock could not be added", "barcode": item.Barcode})
					return
				}
				stockAdded++
			}
			created++
			continue
		}
		if err != nil {
			log.Printf("bulk import: existing product lookup failed barcode=%q: %v", item.Barcode, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "existing product could not be checked", "barcode": item.Barcode})
			return
		}

		switch req.ExistingAction {
		case "SKIP":
			skipped++
		case "ADD_STOCK_ONLY":
			if item.Stock > 0 {
				if _, err := tx.Exec(`
					INSERT INTO stock_movements (product_id, movement_date, type, quantity, note)
					VALUES ($1, CURRENT_DATE, 'in', $2, 'CSV toplu stok girişi')
				`, productID, item.Stock); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "stock could not be added", "barcode": item.Barcode})
					return
				}
				stockAdded++
			} else {
				skipped++
			}
		case "UPDATE_INFO":
			if _, err := tx.Exec(`
				UPDATE products
				SET name = $1,
				    sale_price = $2,
				    purchase_price = CASE WHEN $3 > 0 THEN $3 ELSE purchase_price END,
				    category = $4,
				    updated_at = NOW()
				WHERE id = $5
			`, strings.TrimSpace(item.Name), item.Price, item.PurchasePrice, categoryName, productID); err != nil {
				log.Printf("bulk import: product update failed barcode=%q product_id=%d category=%q sale_price=%v purchase_price=%v: %v", item.Barcode, productID, categoryName, item.Price, item.PurchasePrice, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be updated", "barcode": item.Barcode})
				return
			}
			updated++
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid existingAction"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("bulk import: commit failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bulk import could not be committed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"updated": updated,
		"skipped": skipped,
		"stockAdded": stockAdded,
	})
}

func normalizeCategoryKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ToLower(value)

	replacer := strings.NewReplacer(
		"ı", "i",
		"ç", "c",
		"ğ", "g",
		"ö", "o",
		"ş", "s",
		"ü", "u",
	)
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func canonicalCategory(tx *sql.Tx, raw string) (string, error) {
	key := normalizeCategoryKey(raw)
	if key == "" {
		return "", nil
	}

	rows, err := tx.Query(`
		SELECT name
		FROM categories
		WHERE is_active = TRUE
	`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", err
		}
		if normalizeCategoryKey(name) == key {
			return name, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "", sql.ErrNoRows
}
