package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	DB *sql.DB
}

type categoryRequest struct {
	Name string `json:"name" binding:"required"`
}

type categoryResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	IsActive     bool   `json:"is_active"`
	ProductCount int64  `json:"product_count"`
}

func (h *CategoryHandler) List(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT
			c.id,
			c.name,
			c.is_active,
			COUNT(p.id) AS product_count
		FROM categories c
		LEFT JOIN products p
		  ON LOWER(BTRIM(p.category)) = LOWER(BTRIM(c.name))
		 AND p.is_active = TRUE
		WHERE c.is_active = TRUE
		GROUP BY c.id, c.name, c.is_active
		ORDER BY c.name ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "categories could not be listed"})
		return
	}
	defer rows.Close()

	result := make([]categoryResponse, 0)
	for rows.Next() {
		var item categoryResponse
		if err := rows.Scan(&item.ID, &item.Name, &item.IsActive, &item.ProductCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "category could not be read"})
			return
		}
		result = append(result, item)
	}
	c.JSON(http.StatusOK, result)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category name is required"})
		return
	}

	var item categoryResponse
	err := h.DB.QueryRow(`
		INSERT INTO categories (name)
		VALUES ($1)
		ON CONFLICT (LOWER(BTRIM(name))) DO UPDATE
		SET is_active = TRUE, updated_at = NOW()
		RETURNING id, name, is_active
	`, name).Scan(&item.ID, &item.Name, &item.IsActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "category could not be created"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
		return
	}

	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category name is required"})
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "category update could not start"})
		return
	}
	defer tx.Rollback()

	var oldName string
	if err := tx.QueryRow(`SELECT name FROM categories WHERE id = $1 FOR UPDATE`, id).Scan(&oldName); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "category could not be read"})
		}
		return
	}

	var item categoryResponse
	if err := tx.QueryRow(`
		UPDATE categories
		SET name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, name, is_active
	`, name, id).Scan(&item.ID, &item.Name, &item.IsActive); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "category name already exists"})
		return
	}

	if _, err := tx.Exec(`
		UPDATE products
		SET category = $1, updated_at = NOW()
		WHERE LOWER(BTRIM(COALESCE(category, ''))) = LOWER(BTRIM($2))
	`, name, oldName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "products could not be updated"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "category update could not be committed"})
		return
	}
	c.JSON(http.StatusOK, item)
}
