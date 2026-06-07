# hizlisatis-backend

Go, Gin, PostgreSQL ve JWT ile hazırlanmış Hızlı Satış POS backend projesi.

## Gereksinimler

- Go 1.22+
- PostgreSQL 13+
- `psql` komut satırı aracı

## Kurulum

```bash
cp .env.example .env
go mod download
```

`.env` içindeki `DATABASE_URL` ve `JWT_SECRET` değerlerini kendi ortamınıza göre düzenleyin.

## Veritabanı Oluşturma

```bash
createdb hizlisatis
```

## Migration Çalıştırma

```bash
psql "postgres://postgres:password@localhost:5432/hizlisatis?sslmode=disable" -f migrations/001_init.sql
```

Migration varsayılan kullanıcıyı oluşturur:

- username: `admin`
- password: `123456`

## Backend Başlatma

```bash
go run ./cmd/server
```

Server varsayılan olarak `http://localhost:8082` adresinde çalışır.

## API Kullanımı

### Login

```bash
curl -X POST http://localhost:8082/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'
```

Frontend token değerini `localStorage` içinde `pos_token` anahtarıyla saklayabilir.

Komut satırı örneklerinde token almak için:

```bash
TOKEN=$(curl -s -X POST http://localhost:8082/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}' | jq -r .token)
```

### Ürün Oluşturma

```bash
curl -X POST http://localhost:8082/api/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Süt 1L",
    "barcode": "869000000001",
    "price": 32.50,
    "stock": 50,
    "category": "Süt Ürünleri",
    "brand": "Örnek Marka",
    "description": "1 litre günlük süt",
    "image_url": "https://example.com/sut.jpg"
  }'
```

### Ürün Listeleme

```bash
curl http://localhost:8082/api/products \
  -H "Authorization: Bearer $TOKEN"
```

### ID ile Ürün Getirme

```bash
curl http://localhost:8082/api/products/1 \
  -H "Authorization: Bearer $TOKEN"
```

### Barkod ile Ürün Getirme

```bash
curl http://localhost:8082/api/products/barcode/869000000001 \
  -H "Authorization: Bearer $TOKEN"
```

### Ürün Güncelleme

```bash
curl -X PUT http://localhost:8082/api/products/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Süt 1L",
    "barcode": "869000000001",
    "price": 35.00,
    "stock": 45,
    "category": "Süt Ürünleri",
    "brand": "Örnek Marka",
    "description": "Güncel fiyat",
    "image_url": "https://example.com/sut.jpg"
  }'
```

### Ürün Silme

```bash
curl -X DELETE http://localhost:8082/api/products/1 \
  -H "Authorization: Bearer $TOKEN"
```

### Satış Oluşturma

Backend ürün fiyatını veritabanından alır. Frontend fiyat gönderse bile dikkate alınmaz.

```bash
curl -X POST http://localhost:8082/api/sales \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "payment_method": "cash",
    "items": [
      { "barcode": "869000000001", "quantity": 2 }
    ]
  }'
```

Geçerli `payment_method` değerleri:

- `cash`
- `card`
- `current`
- `other`

### Satış Listeleme

```bash
curl http://localhost:8082/api/sales \
  -H "Authorization: Bearer $TOKEN"
```

### Satış Detayı

```bash
curl http://localhost:8082/api/sales/1 \
  -H "Authorization: Bearer $TOKEN"
```

### Bugünkü Satışlar

```bash
curl http://localhost:8082/api/sales/today \
  -H "Authorization: Bearer $TOKEN"
```

### Günlük Rapor

```bash
curl http://localhost:8082/api/reports/daily \
  -H "Authorization: Bearer $TOKEN"
```

### En Çok Satan Ürünler

```bash
curl http://localhost:8082/api/reports/top-products \
  -H "Authorization: Bearer $TOKEN"
```

## Satış Kuralları

- Satış JWT korumalıdır.
- Satış transaction içinde yapılır.
- Stok yeterli değilse satış oluşturulmaz.
- `sales`, `sale_items`, `stock_movements` kayıtları aynı transaction içinde yazılır.
- Ürün stoğu satış miktarı kadar düşürülür.
- Satış numarası otomatik üretilir.
