package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"hizlisatis-backend/internal/models"
)

type ProductHandler struct {
	DB *sql.DB
}

func (h *ProductHandler) List(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, name, barcode, price, stock, category, brand, description, image_url, is_bestseller, bestseller_order, created_at, updated_at
		FROM products
		ORDER BY name ASC
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
		SELECT id, name, barcode, price, stock, category, brand, description, image_url, is_bestseller, bestseller_order, created_at, updated_at
		FROM products
		WHERE is_bestseller = TRUE
		ORDER BY bestseller_order ASC, name ASC
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

	var product models.Product
	err := h.DB.QueryRow(`
		INSERT INTO products (name, barcode, price, stock, category, brand, description, image_url, is_bestseller, bestseller_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, name, barcode, price, stock, category, brand, description, image_url, is_bestseller, bestseller_order, created_at, updated_at
	`,
		req.Name,
		req.Barcode,
		req.Price,
		req.Stock,
		models.NullString(req.Category),
		models.NullString(req.Brand),
		models.NullString(req.Description),
		models.NullString(req.ImageURL),
		req.IsBestseller,
		req.BestsellerOrder,
	).Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock, &product.Category, &product.Brand, &product.Description, &product.ImageURL, &product.IsBestseller, &product.BestsellerOrder, &product.CreatedAt, &product.UpdatedAt)
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "barcode already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be created"})
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

	var product models.Product
	err = h.DB.QueryRow(`
		UPDATE products
		SET name = $1,
		    barcode = $2,
		    price = $3,
		    stock = $4,
		    category = $5,
		    brand = $6,
		    description = $7,
		    image_url = $8,
		    is_bestseller = $9,
		    bestseller_order = $10,
		    updated_at = NOW()
		WHERE id = $11
		RETURNING id, name, barcode, price, stock, category, brand, description, image_url, is_bestseller, bestseller_order, created_at, updated_at
	`,
		req.Name,
		req.Barcode,
		req.Price,
		req.Stock,
		models.NullString(req.Category),
		models.NullString(req.Brand),
		models.NullString(req.Description),
		models.NullString(req.ImageURL),
		req.IsBestseller,
		req.BestsellerOrder,
		id,
	).Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock, &product.Category, &product.Brand, &product.Description, &product.ImageURL, &product.IsBestseller, &product.BestsellerOrder, &product.CreatedAt, &product.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "barcode already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product could not be updated"})
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

	result, err := h.DB.Exec("DELETE FROM products WHERE id = $1", id)
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
		SELECT id, name, barcode, price, stock, category, brand, description, image_url, is_bestseller, bestseller_order, created_at, updated_at
		FROM products
		WHERE id = $1
	`, id).Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock, &product.Category, &product.Brand, &product.Description, &product.ImageURL, &product.IsBestseller, &product.BestsellerOrder, &product.CreatedAt, &product.UpdatedAt)
	return product, err
}

func (h *ProductHandler) findProductByBarcode(barcode string) (models.Product, error) {
	var product models.Product
	err := h.DB.QueryRow(`
		SELECT id, name, barcode, price, stock, category, brand, description, image_url, is_bestseller, bestseller_order, created_at, updated_at
		FROM products
		WHERE barcode = $1
	`, barcode).Scan(&product.ID, &product.Name, &product.Barcode, &product.Price, &product.Stock, &product.Category, &product.Brand, &product.Description, &product.ImageURL, &product.IsBestseller, &product.BestsellerOrder, &product.CreatedAt, &product.UpdatedAt)
	return product, err
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
