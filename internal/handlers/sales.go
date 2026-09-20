package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hizlisatis-backend/internal/models"
)

type SaleHandler struct {
	DB *sql.DB
}

type saleProduct struct {
	ID      int64
	Name    string
	Barcode string
	Price   float64
	Stock   float64
}

func (h *SaleHandler) Create(c *gin.Context) {
	var req models.CreateSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !validPaymentMethod(req.PaymentMethod) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment_method"})
		return
	}

	userIDValue, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in token"})
		return
	}
	userID, ok := userIDValue.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token user"})
		return
	}

	ctx := c.Request.Context()
	tx, err := h.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sale transaction could not start"})
		return
	}
	defer tx.Rollback()

	createdSale, err := h.createSaleTx(ctx, tx, req, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errProductNotFound) || errors.Is(err, errStockInsufficient) || errors.Is(err, errInvalidSaleItem) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sale transaction could not commit"})
		return
	}

	c.JSON(http.StatusCreated, createdSale)
}

func (h *SaleHandler) List(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, sale_no, payment_method, total_amount, COALESCE(created_by, 1), created_at
		FROM sales
		ORDER BY created_at DESC
		LIMIT 500
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sales could not be listed"})
		return
	}
	defer rows.Close()

	sales := make([]models.Sale, 0)
	for rows.Next() {
		sale, err := scanSale(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "sale could not be read"})
			return
		}
		sales = append(sales, sale)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sales could not be listed"})
		return
	}

	c.JSON(http.StatusOK, sales)
}

func (h *SaleHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return
	}

	sale, err := h.findSaleWithItems(id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "sale not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sale could not be fetched"})
		return
	}

	c.JSON(http.StatusOK, sale)
}

func (h *SaleHandler) Today(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, sale_no, payment_method, total_amount, COALESCE(created_by, 1), created_at
		FROM sales
		WHERE created_at >= CURRENT_DATE
		  AND created_at < CURRENT_DATE + INTERVAL '1 day'
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "today sales could not be listed"})
		return
	}
	defer rows.Close()

	sales := make([]models.Sale, 0)
	for rows.Next() {
		sale, err := scanSale(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "sale could not be read"})
			return
		}
		sales = append(sales, sale)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "today sales could not be listed"})
		return
	}

	c.JSON(http.StatusOK, sales)
}

var (
	errProductNotFound   = errors.New("product not found")
	errStockInsufficient = errors.New("stock is insufficient")
	errInvalidSaleItem   = errors.New("sale item must include product_id or barcode")
)

func (h *SaleHandler) createSaleTx(ctx context.Context, tx *sql.Tx, req models.CreateSaleRequest, userID int64) (models.Sale, error) {
	saleNo := generateSaleNo()
	var saleID int64
	var createdAt time.Time

	if err := tx.QueryRowContext(ctx, `
		INSERT INTO sales (sale_no, payment_method, total_amount, created_by)
		VALUES ($1, $2, 0, $3)
		RETURNING id, created_at
	`, saleNo, req.PaymentMethod, userID).Scan(&saleID, &createdAt); err != nil {
		return models.Sale{}, err
	}

	items := make([]models.SaleItem, 0, len(req.Items))
	totalAmount := 0.0

	for _, itemReq := range req.Items {
		product, err := lockProduct(ctx, tx, itemReq)
		if err != nil {
			return models.Sale{}, err
		}

		// STOK KONTROLU (stock_movements UZERINDEN)
		if product.Stock < itemReq.Quantity {
			return models.Sale{}, fmt.Errorf("%w for product %s (Available: %.2f, Requested: %.2f)", errStockInsufficient, product.Barcode, product.Stock, itemReq.Quantity)
		}

		lineTotal := itemReq.Quantity * product.Price
		totalAmount += lineTotal

		var item models.SaleItem
		err = tx.QueryRowContext(ctx, `
			INSERT INTO sale_items (sale_id, product_id, barcode, product_name, quantity, unit_price, line_total)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, sale_id, product_id, barcode, product_name, quantity, unit_price, line_total
		`, saleID, product.ID, product.Barcode, product.Name, itemReq.Quantity, product.Price, lineTotal).
			Scan(&item.ID, &item.SaleID, &item.ProductID, &item.Barcode, &item.ProductName, &item.Quantity, &item.UnitPrice, &item.LineTotal)
		if err != nil {
			return models.Sale{}, err
		}

		// STOK HAREKETI KAYDI (type = 'out') - TEK STOK KAYNAGI ZEYTINERP
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_movements (product_id, movement_date, type, quantity, unit_price, note)
			VALUES ($1, CURRENT_DATE, 'out', $2, $3, $4)
		`, product.ID, itemReq.Quantity, product.Price, "Hızlı Satış POS: Fiş No "+saleNo); err != nil {
			return models.Sale{}, err
		}

		items = append(items, item)
	}

	if _, err := tx.ExecContext(ctx, "UPDATE sales SET total_amount = $1 WHERE id = $2", totalAmount, saleID); err != nil {
		return models.Sale{}, err
	}

	return models.Sale{
		ID:            saleID,
		SaleNo:        saleNo,
		PaymentMethod: req.PaymentMethod,
		TotalAmount:   totalAmount,
		CreatedBy:     userID,
		CreatedAt:     createdAt,
		Items:         items,
	}, nil
}

func lockProduct(ctx context.Context, tx *sql.Tx, itemReq models.CreateSaleItemRequest) (saleProduct, error) {
	var product saleProduct

	query := `
		SELECT 
			p.id, 
			p.name, 
			COALESCE(p.barcode, '') AS barcode, 
			p.sale_price AS price, 
			COALESCE(SUM(CASE WHEN sm.type IN ('in', 'correction') THEN sm.quantity WHEN sm.type IN ('out', 'waste') THEN -sm.quantity ELSE 0 END), 0) AS stock
		FROM products p
		LEFT JOIN stock_movements sm ON sm.product_id = p.id
		WHERE `

	var row *sql.Row
	if itemReq.ProductID > 0 {
		row = tx.QueryRowContext(ctx, query+`p.id = $1 GROUP BY p.id, p.name, p.barcode, p.sale_price FOR UPDATE OF p`, itemReq.ProductID)
	} else if itemReq.Barcode != "" {
		row = tx.QueryRowContext(ctx, query+`p.barcode = $1 GROUP BY p.id, p.name, p.barcode, p.sale_price FOR UPDATE OF p`, itemReq.Barcode)
	} else {
		return product, errInvalidSaleItem
	}

	if err := row.Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock); errors.Is(err, sql.ErrNoRows) {
		return product, errProductNotFound
	} else if err != nil {
		return product, err
	}

	return product, nil
}

func (h *SaleHandler) findSaleWithItems(id int64) (models.Sale, error) {
	var sale models.Sale
	err := h.DB.QueryRow(`
		SELECT id, sale_no, payment_method, total_amount, COALESCE(created_by, 1), created_at
		FROM sales
		WHERE id = $1
	`, id).Scan(&sale.ID, &sale.SaleNo, &sale.PaymentMethod, &sale.TotalAmount, &sale.CreatedBy, &sale.CreatedAt)
	if err != nil {
		return sale, err
	}

	rows, err := h.DB.Query(`
		SELECT id, sale_id, product_id, barcode, product_name, quantity, unit_price, line_total
		FROM sale_items
		WHERE sale_id = $1
		ORDER BY id ASC
	`, id)
	if err != nil {
		return sale, err
	}
	defer rows.Close()

	sale.Items = make([]models.SaleItem, 0)
	for rows.Next() {
		var item models.SaleItem
		if err := rows.Scan(&item.ID, &item.SaleID, &item.ProductID, &item.Barcode, &item.ProductName, &item.Quantity, &item.UnitPrice, &item.LineTotal); err != nil {
			return sale, err
		}
		sale.Items = append(sale.Items, item)
	}
	if err := rows.Err(); err != nil {
		return sale, err
	}

	return sale, nil
}

type saleScanner interface {
	Scan(dest ...interface{}) error
}

func scanSale(scanner saleScanner) (models.Sale, error) {
	var sale models.Sale
	err := scanner.Scan(&sale.ID, &sale.SaleNo, &sale.PaymentMethod, &sale.TotalAmount, &sale.CreatedBy, &sale.CreatedAt)
	return sale, err
}

func validPaymentMethod(method string) bool {
	switch method {
	case "cash", "card", "current", "other":
		return true
	default:
		return false
	}
}

func generateSaleNo() string {
	return fmt.Sprintf("S%s", time.Now().Format("20060102150405.000000000"))
}
