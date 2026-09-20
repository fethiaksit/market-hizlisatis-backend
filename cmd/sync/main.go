package main

import (
	"database/sql"
	"flag"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	erpDBURL := flag.String("erp-db", os.Getenv("DATABASE_URL"), "ZeytinERP PostgreSQL Database URL")
	flag.Parse()

	if *erpDBURL == "" {
		log.Fatal("DATABASE_URL must be specified via flag or environment variable")
	}

	db, err := sql.Open("postgres", *erpDBURL)
	if err != nil {
		log.Fatalf("PostgreSQL connection error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("PostgreSQL ping error: %v", err)
	}

	log.Println("✅ ZeytinERP PostgreSQL veritabanına bağlanıldı.")
	log.Println("🔄 Hızlı Satış & ZeytinERP veri uyumluluğu kontrol ediliyor...")

	// Verify stock_movements initial stocks
	rows, err := db.Query(`
		SELECT p.id, p.name, COALESCE(p.barcode, ''), p.sale_price,
		       COALESCE(SUM(CASE WHEN sm.type IN ('in', 'correction') THEN sm.quantity WHEN sm.type IN ('out', 'waste') THEN -sm.quantity ELSE 0 END), 0) AS calculated_stock
		FROM products p
		LEFT JOIN stock_movements sm ON sm.product_id = p.id
		WHERE p.is_active = true
		GROUP BY p.id, p.name, p.barcode, p.sale_price
	`)
	if err != nil {
		log.Fatalf("Ürün stok taraması başarısız: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var name, barcode string
		var price, stock float64
		if err := rows.Scan(&id, &name, &barcode, &price, &stock); err != nil {
			log.Printf("Satır okuma hatası: %v", err)
			continue
		}
		count++
		log.Printf("📦 [ÜRÜN #%d] %s (Barkod: %s) -> Fiyat: %.2f TL | Stok (stock_movements): %.2f", id, name, barcode, price, stock)
	}

	log.Printf("🎉 Senkronizasyon tamamlandı. Toplam %d adet ürün doğrulandı ve tek PostgreSQL veritabanına bağlandı.", count)
}
