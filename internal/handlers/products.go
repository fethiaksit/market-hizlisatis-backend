package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"hizlisatis-backend/internal/models"
)

type ProductHandler struct {
	DB *sql.DB
}

func (h *ProductHandler) List(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT 
			p.id, 
			p.name, 
			COALESCE(p.barcode, '') AS barcode, 
			p.sale_price AS price, 
			COALESCE(SUM(CASE WHEN sm.type IN ('in', 'correction') THEN sm.quantity WHEN sm.type IN ('out', 'waste') THEN -sm.quantity ELSE 0 END), 0) AS stock, 
			p.category, 
			p.brand, 
			p.description, 
			p.image_url, 
			p.is_bestseller, 
			p.bestseller_order, 
			p.created_at, 
			p.updated_at
		FROM products p
		LEFT JOIN stock_movements sm ON sm.product_id = p.id
		WHERE p.is_active = TRUE
		GROUP BY p.id, p.name, p.barcode, p.sale_price, p.category, p.brand, p.description, p.image_url, p.is_bestseller, p.bestseller_order, p.created_at, p.updated_at
		ORDER BY p.name ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "products could not be listed"})
		return
	}
	defer rows.Close()

	products := make([]models.ProductResponse, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be read"})
			return
		}
		products = append(products, product.ToResponse())
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "products could not be listed"})
		return
	}

	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) Bestsellers(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT 
			p.id, 
			p.name, 
			COALESCE(p.barcode, '') AS barcode, 
			p.sale_price AS price, 
			COALESCE(SUM(CASE WHEN sm.type IN ('in', 'correction') THEN sm.quantity WHEN sm.type IN ('out', 'waste') THEN -sm.quantity ELSE 0 END), 0) AS stock, 
			p.category, 
			p.brand, 
			p.description, 
			p.image_url, 
			p.is_bestseller, 
			p.bestseller_order, 
			p.created_at, 
			p.updated_at
		FROM products p
		LEFT JOIN stock_movements sm ON sm.product_id = p.id
		WHERE p.is_active = TRUE AND p.is_bestseller = TRUE
		GROUP BY p.id, p.name, p.barcode, p.sale_price, p.category, p.brand, p.description, p.image_url, p.is_bestseller, p.bestseller_order, p.created_at, p.updated_at
		ORDER BY p.bestseller_order ASC, p.name ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bestseller products could not be listed"})
		return
	}
	defer rows.Close()

	products := make([]models.ProductResponse, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "bestseller product could not be read"})
			return
		}
		products = append(products, product.ToResponse())
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bestseller products could not be listed"})
		return
	}

	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	product, err := h.findProductByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be fetched"})
		return
	}

	c.JSON(http.StatusOK, product.ToResponse())
}

func (h *ProductHandler) GetByBarcode(c *gin.Context) {
	product, err := h.findProductByBarcode(c.Param("barcode"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be fetched"})
		return
	}

	c.JSON(http.StatusOK, product.ToResponse())
}

func (h *ProductHandler) Create(c *gin.Context) {
	var req models.ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	categoryName, err := h.canonicalCategoryName(req.Category)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category not found", "category": req.Category})
		return
	}
	req.Category = categoryName

	var productID int64
	err := h.DB.QueryRow(`
		INSERT INTO products (name, barcode, sale_price, purchase_price, category, brand, description, image_url, is_bestseller, bestseller_order, is_active)
		VALUES ($1, $2, $3, 0, $4, $5, $6, $7, $8, $9, TRUE)
		RETURNING id
	`,
		req.Name,
		req.Barcode,
		req.Price,
		req.Category,
		req.Brand,
		req.Description,
		req.ImageURL,
		req.IsBestseller,
		req.BestsellerOrder,
	).Scan(&productID)
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "barcode already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be created"})
		return
	}

	// Initial stock movement if stock > 0
	if req.Stock > 0 {
		h.DB.Exec(`
			INSERT INTO stock_movements (product_id, movement_date, type, quantity, note)
			VALUES ($1, CURRENT_DATE, 'in', $2, 'Hızlı Satış Başlangıç Stoğu')
		`, productID, req.Stock)
	}

	product, err := h.findProductByID(productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product fetch failed after creation"})
		return
	}

	c.JSON(http.StatusCreated, product.ToResponse())
}

func (h *ProductHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	var req models.ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	categoryName, err := h.canonicalCategoryName(req.Category)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category not found", "category": req.Category})
		return
	}
	req.Category = categoryName

	res, err := h.DB.Exec(`
		UPDATE products
		SET name = $1,
		    barcode = $2,
		    sale_price = $3,
		    category = $4,
		    brand = $5,
		    description = $6,
		    image_url = $7,
		    is_bestseller = $8,
		    bestseller_order = $9,
		    updated_at = NOW()
		WHERE id = $10
	`,
		req.Name,
		req.Barcode,
		req.Price,
		req.Category,
		req.Brand,
		req.Description,
		req.ImageURL,
		req.IsBestseller,
		req.BestsellerOrder,
		id,
	)
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "barcode already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be updated"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	product, err := h.findProductByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product fetch failed"})
		return
	}

	c.JSON(http.StatusOK, product.ToResponse())
}

func (h *ProductHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	result, err := h.DB.Exec("UPDATE products SET is_active = FALSE WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be deleted"})
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete result could not be read"})
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ProductHandler) findProductByID(id int64) (models.Product, error) {
	var product models.Product
	err := h.DB.QueryRow(`
		SELECT 
			p.id, p.name, COALESCE(p.barcode, '') AS barcode, p.sale_price AS price, 
			COALESCE(SUM(CASE WHEN sm.type IN ('in', 'correction') THEN sm.quantity WHEN sm.type IN ('out', 'waste') THEN -sm.quantity ELSE 0 END), 0) AS stock, 
			p.category, p.brand, p.description, p.image_url, p.is_bestseller, p.bestseller_order, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN stock_movements sm ON sm.product_id = p.id
		WHERE p.id = $1
		GROUP BY p.id, p.name, p.barcode, p.sale_price, p.category, p.brand, p.description, p.image_url, p.is_bestseller, p.bestseller_order, p.created_at, p.updated_at
	`, id).Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock, &product.Category, &product.Brand, &product.Description, &product.ImageURL, &product.IsBestseller, &product.BestsellerOrder, &product.CreatedAt, &product.UpdatedAt)
	return product, err
}

func (h *ProductHandler) findProductByBarcode(barcode string) (models.Product, error) {
	var product models.Product
	err := h.DB.QueryRow(`
		SELECT 
			p.id, p.name, COALESCE(p.barcode, '') AS barcode, p.sale_price AS price, 
			COALESCE(SUM(CASE WHEN sm.type IN ('in', 'correction') THEN sm.quantity WHEN sm.type IN ('out', 'waste') THEN -sm.quantity ELSE 0 END), 0) AS stock, 
			p.category, p.brand, p.description, p.image_url, p.is_bestseller, p.bestseller_order, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN stock_movements sm ON sm.product_id = p.id
		WHERE p.barcode = $1
		GROUP BY p.id, p.name, p.barcode, p.sale_price, p.category, p.brand, p.description, p.image_url, p.is_bestseller, p.bestseller_order, p.created_at, p.updated_at
	`, barcode).Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock, &product.Category, &product.Brand, &product.Description, &product.ImageURL, &product.IsBestseller, &product.BestsellerOrder, &product.CreatedAt, &product.UpdatedAt)
	return product, err
}

func (h *ProductHandler) canonicalCategoryName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", nil
	}

	var canonical string
	err := h.DB.QueryRow(`
		SELECT name
		FROM categories
		WHERE is_active = TRUE
		  AND LOWER(BTRIM(name)) = LOWER(BTRIM($1))
	`, name).Scan(&canonical)
	return canonical, err
}

type productScanner interface {
	Scan(dest ...interface{}) error
}

func scanProduct(scanner productScanner) (models.Product, error) {
	var product models.Product
	err := scanner.Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock, &product.Category, &product.Brand, &product.Description, &product.ImageURL, &product.IsBestseller, &product.BestsellerOrder, &product.CreatedAt, &product.UpdatedAt)
	return product, err
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
