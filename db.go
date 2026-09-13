package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

const defaultNeonDSN = "postgresql://neondb_owner:npg_edB0iohNLFa9@ep-sparkling-breeze-az12xbxn-pooler.c-3.ap-southeast-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require"

// NeonRepo manages Neon PostgreSQL operations and synchronization with NilLang Alap
type NeonRepo struct {
	pool *data.RealDBPool
	dsn  string
	mu   sync.RWMutex
}

// InitNeonDB connects to Neon PostgreSQL and runs migrations and seeding
func InitNeonDB() (*NeonRepo, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultNeonDSN
	}

	fmt.Println("🔌 [Alap Data] Connecting to Neon PostgreSQL (AWS Southeast Asia)...")
	realPool, err := data.OpenPostgres(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	repo := &NeonRepo{
		pool: realPool,
		dsn:  dsn,
	}

	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("neon migration failed: %w", err)
	}

	if err := repo.seedIfEmpty(); err != nil {
		return nil, fmt.Errorf("neon seeding failed: %w", err)
	}

	fmt.Println("✅ [Alap Data] Neon PostgreSQL connected and synchronized successfully!")
	return repo, nil
}

func (r *NeonRepo) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS categories (
		"id" VARCHAR(64) PRIMARY KEY,
		"name" VARCHAR(255) NOT NULL,
		"nameBn" VARCHAR(255),
		"icon" VARCHAR(64)
	);

	CREATE TABLE IF NOT EXISTS products (
		"id" VARCHAR(64) PRIMARY KEY,
		"name" VARCHAR(255) NOT NULL,
		"nameBn" VARCHAR(255),
		"sku" VARCHAR(128),
		"barcode" VARCHAR(128),
		"categoryId" VARCHAR(64),
		"category" VARCHAR(255),
		"categoryBn" VARCHAR(255),
		"unit" VARCHAR(64),
		"priceMinor" BIGINT NOT NULL DEFAULT 0,
		"wacMinor" BIGINT NOT NULL DEFAULT 0,
		"stock" BIGINT NOT NULL DEFAULT 0,
		"lowStock" BIGINT NOT NULL DEFAULT 5
	);

	CREATE TABLE IF NOT EXISTS customers (
		"id" VARCHAR(64) PRIMARY KEY,
		"name" VARCHAR(255) NOT NULL,
		"nameBn" VARCHAR(255),
		"phone" VARCHAR(64),
		"dueMinor" BIGINT NOT NULL DEFAULT 0,
		"prepaidMinor" BIGINT NOT NULL DEFAULT 0,
		"creditLimitMinor" BIGINT NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS suppliers (
		"id" VARCHAR(64) PRIMARY KEY,
		"name" VARCHAR(255) NOT NULL,
		"nameBn" VARCHAR(255),
		"phone" VARCHAR(64),
		"address" TEXT,
		"dueMinor" BIGINT NOT NULL DEFAULT 0,
		"creditLimitMinor" BIGINT NOT NULL DEFAULT 0,
		"totalPurchaseMinor" BIGINT NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS expenses (
		"id" VARCHAR(64) PRIMARY KEY,
		"description" TEXT NOT NULL,
		"category" VARCHAR(128),
		"categoryBn" VARCHAR(128),
		"amountMinor" BIGINT NOT NULL DEFAULT 0,
		"paidBy" VARCHAR(128),
		"timestamp" BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS purchase_orders (
		"id" VARCHAR(64) PRIMARY KEY,
		"supplierId" VARCHAR(64),
		"supplierName" VARCHAR(255),
		"status" VARCHAR(64),
		"totalMinor" BIGINT NOT NULL DEFAULT 0,
		"itemCount" BIGINT NOT NULL DEFAULT 0,
		"notes" TEXT,
		"timestamp" BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sales (
		"id" VARCHAR(64) PRIMARY KEY,
		"invoiceNo" VARCHAR(64) UNIQUE NOT NULL,
		"customerId" VARCHAR(64),
		"customerName" VARCHAR(255),
		"cashierName" VARCHAR(255),
		"subtotalMinor" BIGINT NOT NULL DEFAULT 0,
		"discountMinor" BIGINT NOT NULL DEFAULT 0,
		"taxMinor" BIGINT NOT NULL DEFAULT 0,
		"grandMinor" BIGINT NOT NULL DEFAULT 0,
		"cogsMinor" BIGINT NOT NULL DEFAULT 0,
		"cashPaidMinor" BIGINT NOT NULL DEFAULT 0,
		"upiPaidMinor" BIGINT NOT NULL DEFAULT 0,
		"prepaidPaidMinor" BIGINT NOT NULL DEFAULT 0,
		"duePaidMinor" BIGINT NOT NULL DEFAULT 0,
		"changeMinor" BIGINT NOT NULL DEFAULT 0,
		"timestamp" BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sale_items (
		"id" VARCHAR(64) PRIMARY KEY,
		"saleId" VARCHAR(64) REFERENCES sales("id") ON DELETE CASCADE,
		"productId" VARCHAR(64),
		"productName" VARCHAR(255),
		"sku" VARCHAR(128),
		"unitPriceMinor" BIGINT NOT NULL DEFAULT 0,
		"costMinor" BIGINT NOT NULL DEFAULT 0,
		"qty" BIGINT NOT NULL DEFAULT 0,
		"lineTotalMinor" BIGINT NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS stock_history (
		"id" VARCHAR(64) PRIMARY KEY,
		"productId" VARCHAR(64),
		"type" VARCHAR(64),
		"qtyChange" BIGINT NOT NULL DEFAULT 0,
		"stockAfter" BIGINT NOT NULL DEFAULT 0,
		"reason" TEXT,
		"timestamp" BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS ledger_entries (
		"id" VARCHAR(64) PRIMARY KEY,
		"customerId" VARCHAR(64) REFERENCES customers("id") ON DELETE CASCADE,
		"saleId" VARCHAR(64),
		"dueChange" BIGINT NOT NULL DEFAULT 0,
		"prepaidChange" BIGINT NOT NULL DEFAULT 0,
		"dueAfter" BIGINT NOT NULL DEFAULT 0,
		"prepaidAfter" BIGINT NOT NULL DEFAULT 0,
		"note" TEXT,
		"timestamp" BIGINT NOT NULL
	);
	`
	_, err := r.pool.Exec(schema)
	return err
}

func (r *NeonRepo) seedIfEmpty() error {
	var count int
	err := r.pool.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		fmt.Printf("📦 [Alap Data] Neon PostgreSQL already contains %d products. Skipping initial seed.\n", count)
		return nil
	}

	fmt.Println("🌱 [Alap Data] Seeding catalog and enterprise data into Neon PostgreSQL...")

	return r.pool.Transaction(func(tx *data.RealTx) error {
		// Categories
		for _, c := range initialCategories() {
			_, err := tx.Exec(`INSERT INTO categories ("id", "name", "nameBn", "icon") VALUES ($1, $2, $3, $4) ON CONFLICT ("id") DO NOTHING`,
				c["id"], c["name"], c["nameBn"], c["icon"])
			if err != nil {
				return err
			}
		}

		// Products
		for _, p := range initialProducts() {
			_, err := tx.Exec(`INSERT INTO products ("id", "name", "nameBn", "sku", "barcode", "categoryId", "category", "categoryBn", "unit", "priceMinor", "wacMinor", "stock", "lowStock")
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) ON CONFLICT ("id") DO NOTHING`,
				p["id"], p["name"], p["nameBn"], p["sku"], p["barcode"], p["categoryId"], p["category"], p["categoryBn"], p["unit"], p["priceMinor"], p["wacMinor"], p["stock"], p["lowStock"])
			if err != nil {
				return err
			}
		}

		// Customers
		for _, c := range initialCustomers() {
			_, err := tx.Exec(`INSERT INTO customers ("id", "name", "nameBn", "phone", "dueMinor", "prepaidMinor", "creditLimitMinor")
				VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT ("id") DO NOTHING`,
				c["id"], c["name"], c["nameBn"], c["phone"], c["dueMinor"], c["prepaidMinor"], c["creditLimitMinor"])
			if err != nil {
				return err
			}
		}

		// Suppliers
		for _, s := range initialSuppliers() {
			_, err := tx.Exec(`INSERT INTO suppliers ("id", "name", "nameBn", "phone", "address", "dueMinor", "creditLimitMinor", "totalPurchaseMinor")
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT ("id") DO NOTHING`,
				s["id"], s["name"], s["nameBn"], s["phone"], s["address"], s["dueMinor"], s["creditLimitMinor"], s["totalPurchaseMinor"])
			if err != nil {
				return err
			}
		}

		// Expenses
		for _, e := range initialExpenses() {
			_, err := tx.Exec(`INSERT INTO expenses ("id", "description", "category", "categoryBn", "amountMinor", "paidBy", "timestamp")
				VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT ("id") DO NOTHING`,
				e["id"], e["description"], e["category"], e["categoryBn"], e["amountMinor"], e["paidBy"], e["timestamp"])
			if err != nil {
				return err
			}
		}

		// Purchase Orders
		for _, po := range initialPurchaseOrders() {
			_, err := tx.Exec(`INSERT INTO purchase_orders ("id", "supplierId", "supplierName", "status", "totalMinor", "itemCount", "notes", "timestamp")
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT ("id") DO NOTHING`,
				po["id"], po["supplierId"], po["supplierName"], po["status"], po["totalMinor"], po["itemCount"], po["notes"], po["timestamp"])
			if err != nil {
				return err
			}
		}

		// Initial Sale
		sale := initialSale()
		_, err = tx.Exec(`INSERT INTO sales ("id", "invoiceNo", "customerId", "customerName", "cashierName", "subtotalMinor", "discountMinor", "taxMinor", "grandMinor", "cogsMinor", "cashPaidMinor", "upiPaidMinor", "prepaidPaidMinor", "duePaidMinor", "changeMinor", "timestamp")
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) ON CONFLICT ("id") DO NOTHING`,
			sale["id"], sale["invoiceNo"], sale["customerId"], sale["customerName"], sale["cashierName"], sale["subtotalMinor"], sale["discountMinor"], sale["taxMinor"], sale["grandMinor"], sale["cogsMinor"], sale["cashPaidMinor"], sale["upiPaidMinor"], sale["prepaidPaidMinor"], sale["duePaidMinor"], sale["changeMinor"], sale["timestamp"])
		if err != nil {
			return err
		}

		item := initialSaleItem()
		_, err = tx.Exec(`INSERT INTO sale_items ("id", "saleId", "productId", "productName", "sku", "unitPriceMinor", "costMinor", "qty", "lineTotalMinor")
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT ("id") DO NOTHING`,
			item["id"], item["saleId"], item["productId"], item["productName"], item["sku"], item["unitPriceMinor"], item["costMinor"], item["qty"], item["lineTotalMinor"])
		return err
	})
}

// PopulatePool loads all tables from Neon PostgreSQL into the memory pool
func (r *NeonRepo) PopulatePool(pool *data.DBPool) error {
	tables := []string{
		"categories", "products", "customers", "suppliers",
		"expenses", "purchase_orders", "sales", "sale_items",
		"stock_history", "ledger_entries",
	}

	for _, tbl := range tables {
		rows, err := data.Table(tbl).WithDriver(data.DriverPostgres).RealGet(r.pool)
		if err != nil {
			log.Printf("⚠️ Warning: could not load %s from Neon: %v", tbl, err)
			continue
		}
		for _, row := range rows {
			_, _ = data.Table(tbl).Insert(pool, row)
		}
	}
	return nil
}

// ─── PERSISTENCE MUTATIONS ──────────────────────────────────────────────────

func (r *NeonRepo) PersistCustomerPayment(customerID string, newDue int64, paidAmount int64, ledgerEntry map[string]interface{}) error {
	return r.pool.Transaction(func(tx *data.RealTx) error {
		_, err := tx.Exec(`UPDATE customers SET "dueMinor" = $1 WHERE "id" = $2`, newDue, customerID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO ledger_entries ("id", "customerId", "dueChange", "dueAfter", "note", "timestamp")
			VALUES ($1, $2, $3, $4, $5, $6)`,
			ledgerEntry["id"], ledgerEntry["customerId"], ledgerEntry["dueChange"], ledgerEntry["dueAfter"], ledgerEntry["note"], ledgerEntry["timestamp"])
		return err
	})
}

func (r *NeonRepo) PersistCheckout(
	sale map[string]interface{},
	saleItems []map[string]interface{},
	stockUpdates []map[string]interface{},
	stockHistories []map[string]interface{},
	customerUpdate map[string]interface{},
	ledgerEntry map[string]interface{},
) error {
	return r.pool.Transaction(func(tx *data.RealTx) error {
		// 1. Insert Sale
		_, err := tx.Exec(`INSERT INTO sales ("id", "invoiceNo", "customerId", "customerName", "cashierName", "subtotalMinor", "discountMinor", "taxMinor", "grandMinor", "cogsMinor", "cashPaidMinor", "upiPaidMinor", "prepaidPaidMinor", "duePaidMinor", "changeMinor", "timestamp")
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			sale["id"], sale["invoiceNo"], sale["customerId"], sale["customerName"], sale["cashierName"],
			sale["subtotalMinor"], sale["discountMinor"], sale["taxMinor"], sale["grandMinor"], sale["cogsMinor"],
			sale["cashPaidMinor"], sale["upiPaidMinor"], sale["prepaidPaidMinor"], sale["duePaidMinor"],
			sale["changeMinor"], sale["timestamp"])
		if err != nil {
			return fmt.Errorf("insert sale: %w", err)
		}

		// 2. Insert Sale Items
		for _, item := range saleItems {
			_, err := tx.Exec(`INSERT INTO sale_items ("id", "saleId", "productId", "productName", "sku", "unitPriceMinor", "costMinor", "qty", "lineTotalMinor")
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				item["id"], item["saleId"], item["productId"], item["productName"], item["sku"],
				item["unitPriceMinor"], item["costMinor"], item["qty"], item["lineTotalMinor"])
			if err != nil {
				return fmt.Errorf("insert sale item: %w", err)
			}
		}

		// 3. Update Product Stocks
		for _, su := range stockUpdates {
			_, err := tx.Exec(`UPDATE products SET "stock" = $1 WHERE "id" = $2`, su["stock"], su["id"])
			if err != nil {
				return fmt.Errorf("update stock: %w", err)
			}
		}

		// 4. Insert Stock Histories
		for _, sh := range stockHistories {
			_, err := tx.Exec(`INSERT INTO stock_history ("id", "productId", "type", "qtyChange", "stockAfter", "reason", "timestamp")
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				sh["id"], sh["productId"], sh["type"], sh["qtyChange"], sh["stockAfter"], sh["reason"], sh["timestamp"])
			if err != nil {
				return fmt.Errorf("insert stock history: %w", err)
			}
		}

		// 5. Update Customer if needed
		if customerUpdate != nil {
			_, err := tx.Exec(`UPDATE customers SET "dueMinor" = $1, "prepaidMinor" = $2 WHERE "id" = $3`,
				customerUpdate["dueMinor"], customerUpdate["prepaidMinor"], customerUpdate["id"])
			if err != nil {
				return fmt.Errorf("update customer: %w", err)
			}
		}

		// 6. Insert Ledger Entry if needed
		if ledgerEntry != nil {
			_, err := tx.Exec(`INSERT INTO ledger_entries ("id", "customerId", "saleId", "dueChange", "prepaidChange", "dueAfter", "prepaidAfter", "note", "timestamp")
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				ledgerEntry["id"], ledgerEntry["customerId"], ledgerEntry["saleId"],
				ledgerEntry["dueChange"], ledgerEntry["prepaidChange"],
				ledgerEntry["dueAfter"], ledgerEntry["prepaidAfter"],
				ledgerEntry["note"], ledgerEntry["timestamp"])
			if err != nil {
				return fmt.Errorf("insert ledger: %w", err)
			}
		}

		return nil
	})
}

func (r *NeonRepo) PersistRefund(productID string, newStock int64, stockHistory map[string]interface{}) error {
	return r.pool.Transaction(func(tx *data.RealTx) error {
		_, err := tx.Exec(`UPDATE products SET "stock" = $1 WHERE "id" = $2`, newStock, productID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO stock_history ("id", "productId", "type", "qtyChange", "stockAfter", "reason", "timestamp")
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			stockHistory["id"], stockHistory["productId"], stockHistory["type"],
			stockHistory["qtyChange"], stockHistory["stockAfter"], stockHistory["reason"], stockHistory["timestamp"])
		return err
	})
}

func (r *NeonRepo) PersistInventoryAdjust(productID string, newStock, newWac int64, stockHistory map[string]interface{}) error {
	return r.pool.Transaction(func(tx *data.RealTx) error {
		_, err := tx.Exec(`UPDATE products SET "stock" = $1, "wacMinor" = $2 WHERE "id" = $3`, newStock, newWac, productID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO stock_history ("id", "productId", "type", "qtyChange", "stockAfter", "reason", "timestamp")
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			stockHistory["id"], stockHistory["productId"], stockHistory["type"],
			stockHistory["qtyChange"], stockHistory["stockAfter"], stockHistory["reason"], stockHistory["timestamp"])
		return err
	})
}

func (r *NeonRepo) PersistExpense(exp map[string]interface{}) error {
	_, err := r.pool.Exec(`INSERT INTO expenses ("id", "description", "category", "categoryBn", "amountMinor", "paidBy", "timestamp")
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		exp["id"], exp["description"], exp["category"], exp["categoryBn"], exp["amountMinor"], exp["paidBy"], exp["timestamp"])
	return err
}

func (r *NeonRepo) PersistSupplier(sup map[string]interface{}) error {
	_, err := r.pool.Exec(`INSERT INTO suppliers ("id", "name", "nameBn", "phone", "address", "dueMinor", "creditLimitMinor", "totalPurchaseMinor")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sup["id"], sup["name"], sup["nameBn"], sup["phone"], sup["address"], sup["dueMinor"], sup["creditLimitMinor"], sup["totalPurchaseMinor"])
	return err
}

func (r *NeonRepo) PersistPurchaseOrder(po map[string]interface{}) error {
	_, err := r.pool.Exec(`INSERT INTO purchase_orders ("id", "supplierId", "supplierName", "status", "totalMinor", "itemCount", "notes", "timestamp")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		po["id"], po["supplierId"], po["supplierName"], po["status"], po["totalMinor"], po["itemCount"], po["notes"], po["timestamp"])
	return err
}

// ─── INITIAL SEED DEFINITIONS ───────────────────────────────────────────────

func initialCategories() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "cat-groceries", "name": "Groceries", "nameBn": "মুদি ও চাল-ডাল", "icon": "🌾"},
		{"id": "cat-snacks", "name": "Packaged Snacks", "nameBn": "প্যাকেটজাত খাবার", "icon": "🍪"},
		{"id": "cat-beverages", "name": "Beverages", "nameBn": "পানীয়", "icon": "🥤"},
		{"id": "cat-dairy", "name": "Dairy & Frozen", "nameBn": "দুগ্ধজাত ও হিমায়িত", "icon": "🥛"},
		{"id": "cat-personal", "name": "Personal Care", "nameBn": "ব্যক্তিগত যত্ন", "icon": "🧼"},
		{"id": "cat-cleaning", "name": "Household & Cleaning", "nameBn": "গৃহস্থালি ও পরিষ্কার", "icon": "🧹"},
		{"id": "cat-confectionery", "name": "Confectionery", "nameBn": "মিষ্টান্ন ও চকোলেট", "icon": "🍫"},
		{"id": "cat-general", "name": "General", "nameBn": "সাধারণ", "icon": "📦"},
	}
}

func initialProducts() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id": "p-01", "name": "Anmol Marie 30 Taka", "nameBn": "আনমোল মেরি ৩০ টাকা",
			"sku": "SNK-ANMOL-30", "barcode": "890103001001", "categoryId": "cat-snacks",
			"category": "Packaged Snacks", "categoryBn": "প্যাকেটজাত খাবার",
			"unit": "piece", "priceMinor": int64(3000), "wacMinor": int64(2400),
			"stock": int64(53), "lowStock": int64(10),
		},
		{
			"id": "p-02", "name": "Nutri Choice Cracker 50 Taka", "nameBn": "নিউট্রি চয়েস ক্র্যাকার ৫০ টাকা",
			"sku": "SNK-NUTRI-50", "barcode": "890103001002", "categoryId": "cat-snacks",
			"category": "Packaged Snacks", "categoryBn": "প্যাকেটজাত খাবার",
			"unit": "piece", "priceMinor": int64(5000), "wacMinor": int64(4100),
			"stock": int64(7), "lowStock": int64(10),
		},
		{
			"id": "p-03", "name": "Mundoba", "nameBn": "মুন্ডোবা",
			"sku": "GEN-MUNDOBA-5", "barcode": "890103001003", "categoryId": "cat-general",
			"category": "General", "categoryBn": "সাধারণ",
			"unit": "piece", "priceMinor": int64(500), "wacMinor": int64(350),
			"stock": int64(1119), "lowStock": int64(50),
		},
		{
			"id": "p-04", "name": "Tata Tea Gold 100g", "nameBn": "টাটা টি গোল্ড ১০০ গ্রা",
			"sku": "GRO-TATAGOLD-100", "barcode": "890103001004", "categoryId": "cat-groceries",
			"category": "Groceries", "categoryBn": "মুদি ও চাল-ডাল",
			"unit": "piece", "priceMinor": int64(4500), "wacMinor": int64(3800),
			"stock": int64(53), "lowStock": int64(10),
		},
		{
			"id": "p-05", "name": "Amul Butter 100g", "nameBn": "আমুল মাখন ১০০ গ্রা",
			"sku": "DAI-AMULBUT-100", "barcode": "890103001005", "categoryId": "cat-dairy",
			"category": "Dairy & Frozen", "categoryBn": "দুগ্ধজাত ও হিমায়িত",
			"unit": "piece", "priceMinor": int64(5600), "wacMinor": int64(4800),
			"stock": int64(24), "lowStock": int64(5),
		},
		{
			"id": "p-06", "name": "Dairy Milk Silk 60g", "nameBn": "ডেইরি মিল্ক সিল্ক ৬০ গ্রা",
			"sku": "CNF-DMSILK-60", "barcode": "890103001006", "categoryId": "cat-confectionery",
			"category": "Confectionery", "categoryBn": "মিষ্টান্ন ও চকোলেট",
			"unit": "piece", "priceMinor": int64(8000), "wacMinor": int64(6800),
			"stock": int64(40), "lowStock": int64(10),
		},
		{
			"id": "p-07", "name": "Dettol Soap Original (75g)", "nameBn": "ডেটোল সাবান অরিজিনাল (৭৫ গ্রাম)",
			"sku": "PC-DETTOL-75", "barcode": "890103001007", "categoryId": "cat-personal",
			"category": "Personal Care", "categoryBn": "ব্যক্তিগত যত্ন",
			"unit": "piece", "priceMinor": int64(4000), "wacMinor": int64(3200),
			"stock": int64(85), "lowStock": int64(15),
		},
		{
			"id": "p-08", "name": "Surf Excel Quick Wash (1kg)", "nameBn": "সার্ফ এক্সেল ডিটারজেন্ট (১ কেজি)",
			"sku": "HC-SURF-1K", "barcode": "890103001008", "categoryId": "cat-cleaning",
			"category": "Household & Cleaning", "categoryBn": "গৃহস্থালি ও পরিষ্কার",
			"unit": "piece", "priceMinor": int64(14000), "wacMinor": int64(11800),
			"stock": int64(35), "lowStock": int64(8),
		},
		{
			"id": "p-09", "name": "Teer Refined Sugar (1kg)", "nameBn": "তীর চিনি রিফাইনড (১ কেজি)",
			"sku": "GRO-SUGAR-1K", "barcode": "890103001021", "categoryId": "cat-groceries",
			"category": "Groceries", "categoryBn": "মুদি ও চাল-ডাল",
			"unit": "kg", "priceMinor": int64(13500), "wacMinor": int64(12000),
			"stock": int64(80), "lowStock": int64(15),
		},
		{
			"id": "p-10", "name": "Aarong Pure Mustard Oil (500ml)", "nameBn": "আড়ং খাঁটি সরিষার তেল (৫০০ মিলি)",
			"sku": "GRO-MUSTARD-500", "barcode": "890103001022", "categoryId": "cat-groceries",
			"category": "Groceries", "categoryBn": "মুদি ও চাল-ডাল",
			"unit": "piece", "priceMinor": int64(16000), "wacMinor": int64(13800),
			"stock": int64(65), "lowStock": int64(12),
		},
		{
			"id": "p-11", "name": "Fortune Sunflower Oil (1L)", "nameBn": "ফরচুন সূর্যমুখী তেল (১ লিটার)",
			"sku": "GRO-FORTUNE-1L", "barcode": "890103001009", "categoryId": "cat-groceries",
			"category": "Groceries", "categoryBn": "মুদি ও চাল-ডাল",
			"unit": "piece", "priceMinor": int64(16500), "wacMinor": int64(14000),
			"stock": int64(30), "lowStock": int64(8),
		},
		{
			"id": "p-12", "name": "Miniket Premium Rice (25kg)", "nameBn": "মিনিকেট চাল প্রিমিয়াম (২৫ কেজি)",
			"sku": "GRO-RICE-MIN-25", "barcode": "890103001010", "categoryId": "cat-groceries",
			"category": "Groceries", "categoryBn": "মুদি ও চাল-ডাল",
			"unit": "bag", "priceMinor": int64(125000), "wacMinor": int64(110000),
			"stock": int64(20), "lowStock": int64(5),
		},
		{
			"id": "p-13", "name": "Deshi Masoor Dal (1kg)", "nameBn": "দেশি মসুর ডাল (১ কেজি)",
			"sku": "GRO-DAL-MAS-1K", "barcode": "890103001011", "categoryId": "cat-groceries",
			"category": "Groceries", "categoryBn": "মুদি ও চাল-ডাল",
			"unit": "kg", "priceMinor": int64(13000), "wacMinor": int64(11000),
			"stock": int64(50), "lowStock": int64(10),
		},
		{
			"id": "p-14", "name": "Coca Cola Bottle (2L)", "nameBn": "কোকাকোলা (২ লিটার)",
			"sku": "BEV-COCA-2L", "barcode": "890103001012", "categoryId": "cat-beverages",
			"category": "Beverages", "categoryBn": "পানীয়",
			"unit": "piece", "priceMinor": int64(9500), "wacMinor": int64(7800),
			"stock": int64(28), "lowStock": int64(6),
		},
		{
			"id": "p-15", "name": "Horlicks Classic Malt (500g)", "nameBn": "হরলিক্স স্বাস্থ্য পানীয় (৫০০ গ্রাম)",
			"sku": "BEV-HORLICKS-500", "barcode": "890103001013", "categoryId": "cat-beverages",
			"category": "Beverages", "categoryBn": "পানীয়",
			"unit": "piece", "priceMinor": int64(26500), "wacMinor": int64(22500),
			"stock": int64(18), "lowStock": int64(5),
		},
		{
			"id": "p-16", "name": "Maggi 2-Minute Noodles (4-Pack)", "nameBn": "ম্যাগি ২-মিনিট নুডলস (৪ প্যাক)",
			"sku": "SNK-MAGGI-4P", "barcode": "890103001014", "categoryId": "cat-snacks",
			"category": "Packaged Snacks", "categoryBn": "প্যাকেটজাত খাবার",
			"unit": "piece", "priceMinor": int64(6000), "wacMinor": int64(5000),
			"stock": int64(42), "lowStock": int64(10),
		},
		{
			"id": "p-17", "name": "Lays American Style Cream & Onion", "nameBn": "লেইস আমেরিকান স্টাইল চিপস",
			"sku": "SNK-LAYS-CHIPS", "barcode": "890103001015", "categoryId": "cat-snacks",
			"category": "Packaged Snacks", "categoryBn": "প্যাকেটজাত খাবার",
			"unit": "piece", "priceMinor": int64(2000), "wacMinor": int64(1600),
			"stock": int64(6), "lowStock": int64(10),
		},
		{
			"id": "p-18", "name": "Colgate MaxFresh Toothpaste (150g)", "nameBn": "কোলগেট ম্যাক্সফ্রেশ পেস্ট (১৫০ গ্রাম)",
			"sku": "PC-COLGATE-150", "barcode": "890103001016", "categoryId": "cat-personal",
			"category": "Personal Care", "categoryBn": "ব্যক্তিগত যত্ন",
			"unit": "piece", "priceMinor": int64(11000), "wacMinor": int64(9000),
			"stock": int64(38), "lowStock": int64(8),
		},
		{
			"id": "p-19", "name": "Harpic Power Plus (1L)", "nameBn": "হারপিক টয়লেট ক্লিনার (১ লিটার)",
			"sku": "HC-HARPIC-1L", "barcode": "890103001017", "categoryId": "cat-cleaning",
			"category": "Household & Cleaning", "categoryBn": "গৃহস্থালি ও পরিষ্কার",
			"unit": "piece", "priceMinor": int64(18500), "wacMinor": int64(15000),
			"stock": int64(22), "lowStock": int64(5),
		},
		{
			"id": "p-20", "name": "Parle-G Glucose Biscuits (80g)", "nameBn": "পার্লে-জি গ্লুকোজ বিস্কুট (৮০ গ্রাম)",
			"sku": "SNK-PARLE-80", "barcode": "890103001018", "categoryId": "cat-snacks",
			"category": "Packaged Snacks", "categoryBn": "প্যাকেটজাত খাবার",
			"unit": "piece", "priceMinor": int64(1000), "wacMinor": int64(800),
			"stock": int64(150), "lowStock": int64(25),
		},
	}
}

func initialCustomers() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id": "c-keshab", "name": "Keshab Gupta", "nameBn": "কেশব গুপ্ত",
			"phone": "01712-345678", "dueMinor": int64(0), "prepaidMinor": int64(0), "creditLimitMinor": int64(5000000),
		},
		{
			"id": "c-walkin", "name": "Walk-in Customer", "nameBn": "সাধারণ ক্রেতা (নগদ)",
			"phone": "N/A", "dueMinor": int64(0), "prepaidMinor": int64(0), "creditLimitMinor": int64(0),
		},
		{
			"id": "c-01", "name": "Rahim Stores (Md. Rahim)", "nameBn": "রহিম স্টোরস (মোঃ রহিম)",
			"phone": "01711-223344", "dueMinor": int64(125000), "prepaidMinor": int64(0), "creditLimitMinor": int64(2500000),
		},
		{
			"id": "c-02", "name": "Karim Bhai Tea Stall", "nameBn": "করিম ভাই চা দোকান",
			"phone": "01822-334455", "dueMinor": int64(0), "prepaidMinor": int64(50000), "creditLimitMinor": int64(1000000),
		},
		{
			"id": "c-03", "name": "Babul Mia Restaurant", "nameBn": "বাবুল মিয়া রেস্তোরাঁ",
			"phone": "01933-445566", "dueMinor": int64(340000), "prepaidMinor": int64(0), "creditLimitMinor": int64(5000000),
		},
		{
			"id": "c-04", "name": "Anil Ghosh", "nameBn": "অনিল ঘোষ",
			"phone": "01644-556677", "dueMinor": int64(85000), "prepaidMinor": int64(0), "creditLimitMinor": int64(1500000),
		},
	}
}

func initialSuppliers() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "sup-01", "name": "Rahim Grain Traders", "nameBn": "রহিম গ্রেইন ট্রেডার্স", "phone": "01711-100001", "address": "ঢাকা, বাংলাদেশ", "dueMinor": int64(450000), "creditLimitMinor": int64(5000000), "totalPurchaseMinor": int64(12500000)},
		{"id": "sup-02", "name": "Al-Amin Oil Co.", "nameBn": "আল-আমিন তেল কোং", "phone": "01812-200002", "address": "চট্টগ্রাম, বাংলাদেশ", "dueMinor": int64(120000), "creditLimitMinor": int64(3000000), "totalPurchaseMinor": int64(7800000)},
		{"id": "sup-03", "name": "Teer Flour Mills Ltd.", "nameBn": "তীর ফ্লাওয়ার মিলস লিমিটেড", "phone": "01933-300003", "address": "নারায়ণগঞ্জ, বাংলাদেশ", "dueMinor": int64(0), "creditLimitMinor": int64(2000000), "totalPurchaseMinor": int64(4200000)},
		{"id": "sup-04", "name": "Meghna Salt Industries", "nameBn": "মেঘনা সল্ট ইন্ডাস্ট্রিজ", "phone": "01611-400004", "address": "মুন্সীগঞ্জ, বাংলাদেশ", "dueMinor": int64(80000), "creditLimitMinor": int64(1500000), "totalPurchaseMinor": int64(2100000)},
	}
}

func initialExpenses() []map[string]interface{} {
	now := time.Now()
	return []map[string]interface{}{
		{"id": "exp-01", "description": "বিদ্যুৎ বিল (ফেব্রুয়ারি ২০২৬)", "category": "Utility", "categoryBn": "ইউটিলিটি", "amountMinor": int64(850000), "paidBy": "জয় সরকার", "timestamp": now.Add(-5 * 24 * time.Hour).Unix()},
		{"id": "exp-02", "description": "দোকান ভাড়া (মার্চ ২০২৬)", "category": "Rent", "categoryBn": "ভাড়া", "amountMinor": int64(2500000), "paidBy": "জয় সরকার", "timestamp": now.Add(-2 * 24 * time.Hour).Unix()},
		{"id": "exp-03", "description": "কর্মচারী বেতন (ফেব্রুয়ারি)", "category": "Salary", "categoryBn": "বেতন", "amountMinor": int64(8000000), "paidBy": "জয় সরকার", "timestamp": now.Add(-1 * 24 * time.Hour).Unix()},
		{"id": "exp-04", "description": "প্যাকেজিং সামগ্রী ক্রয়", "category": "Supplies", "categoryBn": "সরবরাহ", "amountMinor": int64(150000), "paidBy": "জয় সরকার", "timestamp": now.Unix()},
		{"id": "exp-05", "description": "পরিবহন ও ডেলিভারি খরচ", "category": "Transport", "categoryBn": "পরিবহন", "amountMinor": int64(320000), "paidBy": "জয় সরকার", "timestamp": now.Add(-3 * 24 * time.Hour).Unix()},
	}
}

func initialPurchaseOrders() []map[string]interface{} {
	now := time.Now()
	return []map[string]interface{}{
		{"id": "po-001", "supplierId": "sup-01", "supplierName": "রহিম গ্রেইন ট্রেডার্স", "status": "RECEIVED", "totalMinor": int64(2550000), "itemCount": int64(3), "notes": "মিনিকেট চাল (৫ বস্তা) + নাজিরশাইল চাল (৩ বস্তা)", "timestamp": now.Add(-3 * 24 * time.Hour).Unix()},
		{"id": "po-002", "supplierId": "sup-02", "supplierName": "আল-আমিন তেল কোং", "status": "PENDING", "totalMinor": int64(1780000), "itemCount": int64(2), "notes": "তীর সয়াবিন তেল + রূপচাঁদা সয়াবিন তেল", "timestamp": now.Add(-1 * 24 * time.Hour).Unix()},
		{"id": "po-003", "supplierId": "sup-03", "supplierName": "তীর ফ্লাওয়ার মিলস", "status": "IN_TRANSIT", "totalMinor": int64(920000), "itemCount": int64(2), "notes": "ফ্রেশ আটা (৫০ প্যাকেট) + তীর ময়দা (৩০ প্যাকেট)", "timestamp": now.Unix()},
		{"id": "po-004", "supplierId": "sup-04", "supplierName": "মেঘনা সল্ট ইন্ডাস্ট্রিজ", "status": "RECEIVED", "totalMinor": int64(420000), "itemCount": int64(1), "notes": "এসিআই লবণ (১০০ প্যাকেট)", "timestamp": now.Add(-7 * 24 * time.Hour).Unix()},
	}
}

func initialSale() map[string]interface{} {
	return map[string]interface{}{
		"id":               "SALE-INIT-001",
		"invoiceNo":        "INV-100001",
		"customerId":       "c-01",
		"customerName":     "Rahim Stores (Md. Rahim)",
		"cashierName":      "Joy Sarkar / জয় সরকার",
		"subtotalMinor":    int64(429000),
		"discountMinor":    int64(10000),
		"taxMinor":         int64(20950),
		"grandMinor":       int64(439950),
		"cogsMinor":        int64(386000),
		"cashPaidMinor":    int64(200000),
		"upiPaidMinor":     int64(100000),
		"prepaidPaidMinor": int64(0),
		"duePaidMinor":     int64(139950),
		"changeMinor":      int64(0),
		"timestamp":        time.Now().Add(-2 * time.Hour).Unix(),
	}
}

func initialSaleItem() map[string]interface{} {
	return map[string]interface{}{
		"id":             "si-01",
		"saleId":         "SALE-INIT-001",
		"productId":      "p-01",
		"productName":    "Anmol Marie 30 Taka",
		"sku":            "SNK-ANMOL-30",
		"unitPriceMinor": int64(3000),
		"costMinor":      int64(2400),
		"qty":            int64(1),
		"lineTotalMinor": int64(3000),
	}
}

// HealthInfo reports live Neon PostgreSQL connection status
func (r *NeonRepo) HealthInfo() map[string]interface{} {
	status := "connected"
	var version string
	if err := r.pool.QueryRow("SELECT version()").Scan(&version); err != nil {
		status = fmt.Sprintf("error: %v", err)
	}

	var productCount, salesCount int
	_ = r.pool.QueryRow("SELECT COUNT(*) FROM products").Scan(&productCount)
	_ = r.pool.QueryRow("SELECT COUNT(*) FROM sales").Scan(&salesCount)

	return map[string]interface{}{
		"status":          status,
		"engine":          "Neon Serverless PostgreSQL (AWS Southeast Asia)",
		"driver":          "jackc/pgx/v5 via NilLang Alap data",
		"pgVersion":       version,
		"totalProducts":   productCount,
		"totalSales":      salesCount,
		"syncMode":        "Live Cloud Persistence + Local L1 Memory Pool",
	}
}
