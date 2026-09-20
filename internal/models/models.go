package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type Product struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	Barcode         string         `json:"barcode"`
	Price           float64        `json:"price"`
	PurchasePrice   float64        `json:"purchasePrice"`
	Stock           float64        `json:"stock"`
	Category        sql.NullString `json:"-"`
	Brand           sql.NullString `json:"-"`
	Description     sql.NullString `json:"-"`
	ImageURL        sql.NullString `json:"-"`
	IsBestseller    bool           `json:"is_bestseller"`
	BestsellerOrder int            `json:"bestseller_order"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type ProductResponse struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Barcode         string    `json:"barcode"`
	Price           float64   `json:"price"`
	PurchasePrice   float64   `json:"purchasePrice"`
	Stock           float64   `json:"stock"`
	Category        string    `json:"category"`
	Brand           string    `json:"brand"`
	Description     string    `json:"description"`
	ImageURL        string    `json:"image_url"`
	IsBestseller    bool      `json:"is_bestseller"`
	BestsellerOrder int       `json:"bestseller_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ProductRequest struct {
	Name            string  `json:"name" binding:"required"`
	Barcode         string  `json:"barcode" binding:"required"`
	Price           float64 `json:"price" binding:"required,gte=0"`
	PurchasePrice   float64 `json:"purchasePrice" binding:"gte=0"`
	Stock           float64 `json:"stock" binding:"gte=0"`
	Category        string  `json:"category"`
	Brand           string  `json:"brand"`
	Description     string  `json:"description"`
	ImageURL        string  `json:"image_url"`
	IsBestseller    bool    `json:"is_bestseller"`
	BestsellerOrder int     `json:"bestseller_order" binding:"gte=0"`
}

type Sale struct {
	ID            int64      `json:"id"`
	SaleNo        string     `json:"sale_no"`
	PaymentMethod string     `json:"payment_method"`
	TotalAmount   float64    `json:"total_amount"`
	CreatedBy     int64      `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	Items         []SaleItem `json:"items,omitempty"`
}

type SaleItem struct {
	ID          int64   `json:"id"`
	SaleID      int64   `json:"sale_id"`
	ProductID   int64   `json:"product_id"`
	Barcode     string  `json:"barcode"`
	ProductName string  `json:"product_name"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	LineTotal   float64 `json:"line_total"`
}

type CreateSaleRequest struct {
	PaymentMethod string                  `json:"payment_method" binding:"required"`
	Items         []CreateSaleItemRequest `json:"items" binding:"required,min=1,dive"`
}

type CreateSaleItemRequest struct {
	ProductID int64   `json:"product_id"`
	Barcode   string  `json:"barcode"`
	Quantity  float64 `json:"quantity" binding:"required,gt=0"`
}

type DailyReport struct {
	Date        string  `json:"date"`
	SaleCount   int64   `json:"sale_count"`
	TotalAmount float64 `json:"total_amount"`
}

type TopProductReport struct {
	ProductID    int64   `json:"product_id"`
	Barcode      string  `json:"barcode"`
	ProductName  string  `json:"product_name"`
	TotalQty     float64 `json:"total_quantity"`
	TotalRevenue float64 `json:"total_revenue"`
}

func (p Product) ToResponse() ProductResponse {
	return ProductResponse{
		ID:              p.ID,
		Name:            p.Name,
		Barcode:         p.Barcode,
		Price:           p.Price,
		PurchasePrice:   p.PurchasePrice,
		Stock:           p.Stock,
		Category:        nullStringValue(p.Category),
		Brand:           nullStringValue(p.Brand),
		Description:     nullStringValue(p.Description),
		ImageURL:        nullStringValue(p.ImageURL),
		IsBestseller:    p.IsBestseller,
		BestsellerOrder: p.BestsellerOrder,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

func NullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
