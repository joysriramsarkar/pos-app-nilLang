package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/device"
	"github.com/joysriramsarkar/nilLang/pkg/alap/i18n"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
	"github.com/joysriramsarkar/nilLang/pkg/alap/server"
	alapsync "github.com/joysriramsarkar/nilLang/pkg/alap/sync"
)

// AppState holds the database pool, enterprise POS domain engine, and runtime state
type AppState struct {
	pool      *data.DBPool
	neonRepo  *NeonRepo
	posEngine *pos.POSEngine
	startTime time.Time
	mu        sync.RWMutex
}

var state *AppState

func main() {
	port := 8090
	if p := os.Getenv("PORT"); p != "" {
		if val, err := strconv.Atoi(p); err == nil {
			port = val
		}
	}

	// 1. Initialize Memory Database Pool (for sub-millisecond local reads)
	pool := data.NewDBPool(data.DBPoolConfig{
		MaxOpenConns: 50,
		MaxIdleConns: 20,
		Driver:       data.DriverPostgres,
	})

	// 2. Initialize Enterprise POS Engine
	engine := pos.NewPOSEngine()
	engine.SeedDefaultEnterpriseData()

	// 3. Connect to Neon PostgreSQL, Migrate Schema & Synchronize
	neonRepo, err := InitNeonDB()
	if err != nil {
		log.Printf("⚠️ Warning: Neon PostgreSQL connection failed (%v). Seeding in-memory database.\n", err)
		seedDatabase(pool)
	} else {
		// Populate memory pool directly from Neon PostgreSQL
		if err := neonRepo.PopulatePool(pool); err != nil {
			log.Printf("⚠️ Warning: PopulatePool failed (%v). Seeding in-memory database.\n", err)
			seedDatabase(pool)
		}
	}

	state = &AppState{
		pool:      pool,
		neonRepo:  neonRepo,
		posEngine: engine,
		startTime: time.Now(),
	}

	// 4. Initialize Alap Service
	srv := server.NewService("lakhan-bhandar-pos", "")

	// 5. Register All API Endpoints
	srv.GET("/api/health", handleHealth)
	srv.GET("/api/catalog", handleCatalog)
	srv.GET("/api/products", handleCatalog)
	srv.GET("/api/customers", handleCustomers)
	srv.GET("/api/due-collection", handleDueCollectionList)
	srv.POST("/api/due-collection", handleCustomerDuePayment)
	srv.GET("/api/ledger/:id", handleCustomerLedger)
	srv.POST("/api/customers/due-payment", handleCustomerDuePayment)
	srv.GET("/api/cart", handleGetCart)
	srv.POST("/api/checkout", handleCheckout)
	srv.POST("/api/refund", handleRefund)
	srv.GET("/api/reports/daily", handleDailyReport)
	srv.GET("/api/reports/export", handleExportReport)
	srv.POST("/api/inventory/adjust", handleInventoryAdjust)
	srv.GET("/api/inventory/movements", handleInventoryMovements)
	srv.GET("/api/shift/current", handleShiftCurrent)
	srv.POST("/api/shift/open", handleShiftOpen)
	srv.POST("/api/shift/close", handleShiftClose)
	srv.POST("/api/sync/push", handleSyncPush)
	srv.GET("/api/sync/pending", handleSyncPending)
	srv.POST("/api/device/printer/test", handlePrinterTest)
	srv.GET("/api/device/printer/test", handlePrinterTest)
	srv.POST("/api/device/drawer/open", handleDrawerOpen)
	srv.GET("/api/device/drawer/open", handleDrawerOpen)
	srv.GET("/api/audit/recent", handleAuditRecent)
	srv.GET("/api/sales", handleSales)
	srv.GET("/api/stats/dashboard", handleDashboardStats)
	srv.GET("/api/expenses", handleExpenses)
	srv.POST("/api/expenses", handleAddExpense)
	srv.GET("/api/suppliers", handleSuppliers)
	srv.POST("/api/suppliers", handleAddSupplier)
	srv.GET("/api/purchase-orders", handlePurchaseOrders)
	srv.POST("/api/purchase-orders", handleAddPurchaseOrder)

	// 6. Static Asset Handlers
	srv.GET("/", handleIndexHTML)
	srv.GET("/index.html", handleIndexHTML)
	srv.GET("/style.css", handleStyleCSS)
	srv.GET("/alap-runtime.js", handleAlapRuntimeJS)
	srv.GET("/app.js", handleAlapRuntimeJS)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("🚀 [Lakhan Bhandar POS] Server starting on http://localhost:%d\n", port)
	fmt.Printf("📦 Enterprise Retail Engine powered by Nilang & Alap Framework\n")
	if err := srv.Listen(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// ─── SEED RELATIONAL DATA ───────────────────────────────────────────────────

func seedDatabase(pool *data.DBPool) {
	// Standard Product Categories matching pos-app
	categories := []map[string]interface{}{
		{"id": "cat-groceries", "name": "Groceries", "nameBn": "মুদি ও চাল-ডাল", "icon": "🌾"},
		{"id": "cat-snacks", "name": "Packaged Snacks", "nameBn": "প্যাকেটজাত খাবার", "icon": "🍪"},
		{"id": "cat-beverages", "name": "Beverages", "nameBn": "পানীয়", "icon": "🥤"},
		{"id": "cat-dairy", "name": "Dairy & Frozen", "nameBn": "দুগ্ধজাত ও হিমায়িত", "icon": "🥛"},
		{"id": "cat-personal", "name": "Personal Care", "nameBn": "ব্যক্তিগত যত্ন", "icon": "🧼"},
		{"id": "cat-cleaning", "name": "Household & Cleaning", "nameBn": "গৃহস্থালি ও পরিষ্কার", "icon": "🧹"},
		{"id": "cat-confectionery", "name": "Confectionery", "nameBn": "মিষ্টান্ন ও চকোলেট", "icon": "🍫"},
		{"id": "cat-general", "name": "General", "nameBn": "সাধারণ", "icon": "📦"},
	}
	for _, c := range categories {
		_, _ = data.Table("categories").Insert(pool, c)
	}

	// Products matching pos-app original catalog and screenshot
	products := []map[string]interface{}{
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
			"stock": int64(7), "lowStock": int64(10), // Low stock alert!
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
			"id": "p-05", "name": "Bisleri Water 5 Litre", "nameBn": "বিসলেরি আর ৫ লিটার",
			"sku": "BEV-BISLERI-5L", "barcode": "8906017290071", "categoryId": "cat-beverages",
			"category": "Beverages", "categoryBn": "পানীয়",
			"unit": "piece", "priceMinor": int64(7000), "wacMinor": int64(5500),
			"stock": int64(40), "lowStock": int64(10),
		},
		{
			"id": "p-06", "name": "B Natural Coconut Water 108 Taka", "nameBn": "বি ন্যাচারাল নারকেল জল ১০৮ টাকা",
			"sku": "BEV-BNATURAL-COCO", "barcode": "8001725008307", "categoryId": "cat-beverages",
			"category": "Beverages", "categoryBn": "পানীয়",
			"unit": "piece", "priceMinor": int64(12000), "wacMinor": int64(9800),
			"stock": int64(25), "lowStock": int64(5),
		},
		{
			"id": "p-07", "name": "Amul Taaza Milk (500ml)", "nameBn": "আমুল তাজা দুধ (৫০০ মি.লি.)",
			"sku": "DAIRY-AMUL-500", "barcode": "890126201005", "categoryId": "cat-dairy",
			"category": "Dairy & Frozen", "categoryBn": "দুগ্ধজাত ও হিমায়িত",
			"unit": "piece", "priceMinor": int64(2700), "wacMinor": int64(2300),
			"stock": int64(35), "lowStock": int64(10),
		},
		{
			"id": "p-08", "name": "Lifebuoy Total Soap (100g)", "nameBn": "লাইফবয় সাবান (১০০ গ্রাম)",
			"sku": "PC-LIFEBUOY-100", "barcode": "890103001006", "categoryId": "cat-personal",
			"category": "Personal Care", "categoryBn": "ব্যক্তিগত যত্ন",
			"unit": "piece", "priceMinor": int64(3500), "wacMinor": int64(2800),
			"stock": int64(80), "lowStock": int64(15),
		},
		{
			"id": "p-09", "name": "Surf Excel Detergent (1kg)", "nameBn": "সার্ফ এক্সেল ডিটারজেন্ট (১ কেজি)",
			"sku": "HC-SURF-1K", "barcode": "890103001007", "categoryId": "cat-cleaning",
			"category": "Household & Cleaning", "categoryBn": "গৃহস্থালি ও পরিষ্কার",
			"unit": "piece", "priceMinor": int64(14500), "wacMinor": int64(12000),
			"stock": int64(45), "lowStock": int64(10),
		},
		{
			"id": "p-10", "name": "Cadbury Dairy Milk (50 Taka)", "nameBn": "ক্যাডবেরি ডেইরি মিল্ক (৫০ টাকা)",
			"sku": "CONF-CADBURY-50", "barcode": "890103001008", "categoryId": "cat-confectionery",
			"category": "Confectionery", "categoryBn": "মিষ্টান্ন ও চকোলেট",
			"unit": "piece", "priceMinor": int64(5000), "wacMinor": int64(4200),
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
			"stock": int64(6), "lowStock": int64(10), // Low stock alert!
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
	for _, p := range products {
		_, _ = data.Table("products").Insert(pool, p)
	}

	// Customers matching pos-app
	customers := []map[string]interface{}{
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
	for _, c := range customers {
		_, _ = data.Table("customers").Insert(pool, c)
	}

	// Suppliers
	suppliers := []map[string]interface{}{
		{"id": "sup-01", "name": "Rahim Grain Traders", "nameBn": "রহিম গ্রেইন ট্রেডার্স", "phone": "01711-100001", "address": "ঢাকা, বাংলাদেশ", "dueMinor": int64(450000), "creditLimitMinor": int64(5000000), "totalPurchaseMinor": int64(12500000)},
		{"id": "sup-02", "name": "Al-Amin Oil Co.", "nameBn": "আল-আমিন তেল কোং", "phone": "01812-200002", "address": "চট্টগ্রাম, বাংলাদেশ", "dueMinor": int64(120000), "creditLimitMinor": int64(3000000), "totalPurchaseMinor": int64(7800000)},
		{"id": "sup-03", "name": "Teer Flour Mills Ltd.", "nameBn": "তীর ফ্লাওয়ার মিলস লিমিটেড", "phone": "01933-300003", "address": "নারায়ণগঞ্জ, বাংলাদেশ", "dueMinor": int64(0), "creditLimitMinor": int64(2000000), "totalPurchaseMinor": int64(4200000)},
		{"id": "sup-04", "name": "Meghna Salt Industries", "nameBn": "মেঘনা সল্ট ইন্ডাস্ট্রিজ", "phone": "01611-400004", "address": "মুন্সীগঞ্জ, বাংলাদেশ", "dueMinor": int64(80000), "creditLimitMinor": int64(1500000), "totalPurchaseMinor": int64(2100000)},
	}
	for _, s := range suppliers {
		_, _ = data.Table("suppliers").Insert(pool, s)
	}

	// Expenses
	expenses := []map[string]interface{}{
		{"id": "exp-01", "description": "বিদ্যুৎ বিল (ফেব্রুয়ারি ২০২৬)", "category": "Utility", "categoryBn": "ইউটিলিটি", "amountMinor": int64(850000), "paidBy": "জয় সরকার", "timestamp": time.Now().Add(-5 * 24 * time.Hour).Unix()},
		{"id": "exp-02", "description": "দোকান ভাড়া (মার্চ ২০২৬)", "category": "Rent", "categoryBn": "ভাড়া", "amountMinor": int64(2500000), "paidBy": "জয় সরকার", "timestamp": time.Now().Add(-2 * 24 * time.Hour).Unix()},
		{"id": "exp-03", "description": "কর্মচারী বেতন (ফেব্রুয়ারি)", "category": "Salary", "categoryBn": "বেতন", "amountMinor": int64(8000000), "paidBy": "জয় সরকার", "timestamp": time.Now().Add(-1 * 24 * time.Hour).Unix()},
		{"id": "exp-04", "description": "প্যাকেজিং সামগ্রী ক্রয়", "category": "Supplies", "categoryBn": "সরবরাহ", "amountMinor": int64(150000), "paidBy": "জয় সরকার", "timestamp": time.Now().Unix()},
		{"id": "exp-05", "description": "পরিবহন ও ডেলিভারি খরচ", "category": "Transport", "categoryBn": "পরিবহন", "amountMinor": int64(320000), "paidBy": "জয় সরকার", "timestamp": time.Now().Add(-3 * 24 * time.Hour).Unix()},
	}
	for _, e := range expenses {
		_, _ = data.Table("expenses").Insert(pool, e)
	}

	// Purchase Orders
	purchaseOrders := []map[string]interface{}{
		{"id": "po-001", "supplierId": "sup-01", "supplierName": "রহিম গ্রেইন ট্রেডার্স", "status": "RECEIVED", "totalMinor": int64(2550000), "itemCount": int64(3), "notes": "মিনিকেট চাল (৫ বস্তা) + নাজিরশাইল চাল (৩ বস্তা)", "timestamp": time.Now().Add(-3 * 24 * time.Hour).Unix()},
		{"id": "po-002", "supplierId": "sup-02", "supplierName": "আল-আমিন তেল কোং", "status": "PENDING", "totalMinor": int64(1780000), "itemCount": int64(2), "notes": "তীর সয়াবিন তেল + রূপচাঁদা সয়াবিন তেল", "timestamp": time.Now().Add(-1 * 24 * time.Hour).Unix()},
		{"id": "po-003", "supplierId": "sup-03", "supplierName": "তীর ফ্লাওয়ার মিলস", "status": "IN_TRANSIT", "totalMinor": int64(920000), "itemCount": int64(2), "notes": "ফ্রেশ আটা (৫০ প্যাকেট) + তীর ময়দা (৩০ প্যাকেট)", "timestamp": time.Now().Unix()},
		{"id": "po-004", "supplierId": "sup-04", "supplierName": "মেঘনা সল্ট ইন্ডাস্ট্রিজ", "status": "RECEIVED", "totalMinor": int64(420000), "itemCount": int64(1), "notes": "এসিআই লবণ (১০০ প্যাকেট)", "timestamp": time.Now().Add(-7 * 24 * time.Hour).Unix()},
	}
	for _, po := range purchaseOrders {
		_, _ = data.Table("purchase_orders").Insert(pool, po)
	}

	// Seed one baseline sale to populate daily reports
	seedInitialSale(pool)
}

func seedInitialSale(pool *data.DBPool) {
	saleID := "SALE-INIT-001"
	invoiceNo := "INV-100001"
	sale := map[string]interface{}{
		"id":               saleID,
		"invoiceNo":        invoiceNo,
		"customerId":       "c-01",
		"customerName":     "Rahim Stores (Md. Rahim)",
		"cashierName":      "Joy Sarkar / জয় সরকার",
		"subtotalMinor":    int64(429000),
		"discountMinor":    int64(10000),
		"taxMinor":         int64(20950), // 5% of (4290 - 100)
		"grandMinor":       int64(439950),
		"cogsMinor":        int64(386000),
		"cashPaidMinor":    int64(200000),
		"upiPaidMinor":     int64(100000),
		"prepaidPaidMinor": int64(0),
		"duePaidMinor":     int64(139950),
		"changeMinor":      int64(0),
		"timestamp":        time.Now().Add(-2 * time.Hour).Unix(),
	}
	_, _ = data.Table("sales").Insert(pool, sale)

	saleItem := map[string]interface{}{
		"id":             "si-01",
		"saleId":         saleID,
		"productId":      "p-01",
		"productName":    "Miniket Rice (50kg Bag)",
		"sku":            "RICE-MIN-50",
		"unitPriceMinor": int64(340000),
		"costMinor":      int64(305000),
		"qty":            int64(1),
		"lineTotalMinor": int64(340000),
	}
	_, _ = data.Table("sale_items").Insert(pool, saleItem)
}

// ─── HTTP HANDLERS ───────────────────────────────────────────────────────────

func handleHealth(ctx *routing.Context) (interface{}, error) {
	uptime := time.Since(state.startTime).Round(time.Second).String()
	total, pending := 0, 0
	if state.posEngine != nil && state.posEngine.SyncQueue != nil {
		total, pending = state.posEngine.SyncQueue.Count()
	}
	res := map[string]interface{}{
		"status":      "ok",
		"app":         "লাখান ভাণ্ডার POS (Lakhan Bhandar)",
		"engine":      "Nilang Core + Alap Framework",
		"version":     "1.0.0 Enterprise",
		"uptime":      uptime,
		"dbStatus":    "connected",
		"memoryUnits": "minor integer paisa (exact)",
		"offlineSync": map[string]int{
			"total":   total,
			"pending": pending,
		},
	}
	if state.neonRepo != nil {
		res["database"] = state.neonRepo.HealthInfo()
	}
	return res, nil
}

func handleCatalog(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	categories, err := data.Table("categories").Get(state.pool)
	if err != nil {
		return nil, err
	}

	products, err := data.Table("products").Get(state.pool)
	if err != nil {
		return nil, err
	}

	// Enrich products with formatted money strings
	enriched := make([]map[string]interface{}, len(products))
	for i, p := range products {
		row := make(map[string]interface{})
		for k, v := range p {
			row[k] = v
		}
		pMinor, _ := toInt64(p["priceMinor"])
		wMinor, _ := toInt64(p["wacMinor"])
		mPrice := data.NewMoney(pMinor, "BDT")
		mWac := data.NewMoney(wMinor, "BDT")
		row["priceFormatted"] = mPrice.Format()
		row["wacFormatted"] = mWac.Format()
		enriched[i] = row
	}

	return map[string]interface{}{
		"categories": categories,
		"products":   enriched,
	}, nil
}

func handleCustomers(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	customers, err := data.Table("customers").Get(state.pool)
	if err != nil {
		return nil, err
	}

	enriched := make([]map[string]interface{}, len(customers))
	for i, c := range customers {
		row := make(map[string]interface{})
		for k, v := range c {
			row[k] = v
		}
		dueMinor, _ := toInt64(c["dueMinor"])
		prepaidMinor, _ := toInt64(c["prepaidMinor"])
		limitMinor, _ := toInt64(c["creditLimitMinor"])

		row["dueFormatted"] = data.NewMoney(dueMinor, "BDT").Format()
		row["prepaidFormatted"] = data.NewMoney(prepaidMinor, "BDT").Format()
		row["creditLimitFormatted"] = data.NewMoney(limitMinor, "BDT").Format()
		enriched[i] = row
	}

	return enriched, nil
}

func handleCustomerLedger(ctx *routing.Context) (interface{}, error) {
	custID := ctx.Param("id")
	if custID == "" {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("customer id required")
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	cust, err := data.Table("customers").Where("id", "=", custID).First(state.pool)
	if err != nil || cust == nil {
		ctx.StatusCode = 404
		return nil, fmt.Errorf("customer not found")
	}

	entries, _ := data.Table("ledger_entries").Where("customerId", "=", custID).Get(state.pool)

	return map[string]interface{}{
		"customer": cust,
		"entries":  entries,
	}, nil
}

type CustomerDuePaymentRequest struct {
	CustomerID    string      `json:"customerId"`
	Amount        interface{} `json:"amount,omitempty"`
	AmountMinor   int64       `json:"amountMinor,omitempty"`
	PaymentMethod string      `json:"paymentMethod,omitempty"`
	Method        string      `json:"method,omitempty"`
	Reference     string      `json:"reference,omitempty"`
	Notes         string      `json:"notes,omitempty"`
}

func handleDueCollectionList(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	customers, err := data.Table("customers").Get(state.pool)
	if err != nil {
		return nil, err
	}

	sales, _ := data.Table("sales").Get(state.pool)
	lastSales := make(map[string]int64)
	for _, s := range sales {
		cID := fmt.Sprintf("%v", s["customerId"])
		ts, _ := toInt64(s["timestamp"])
		if ts > lastSales[cID] {
			lastSales[cID] = ts
		}
	}

	dueList := make([]map[string]interface{}, 0)
	for _, c := range customers {
		cID := fmt.Sprintf("%v", c["id"])
		if cID == "c-walkin" {
			continue
		}
		dueMinor, _ := toInt64(c["dueMinor"])
		if dueMinor <= 0 {
			continue
		}
		row := make(map[string]interface{})
		for k, v := range c {
			row[k] = v
		}
		row["dueAmount"] = float64(dueMinor) / 100.0
		row["dueFormatted"] = data.NewMoney(dueMinor, "BDT").Format()
		if ts, ok := lastSales[cID]; ok && ts > 0 {
			row["lastPaymentDate"] = time.Unix(ts, 0).Format(time.RFC3339)
		} else {
			row["lastPaymentDate"] = nil
		}
		dueList = append(dueList, row)
	}

	return map[string]interface{}{
		"success": true,
		"data":    dueList,
	}, nil
}

func handleCustomerDuePayment(ctx *routing.Context) (interface{}, error) {
	var req CustomerDuePaymentRequest
	bodyBytes, err := json.Marshal(ctx.Body)
	if err != nil {
		ctx.StatusCode = 400
		return map[string]interface{}{"success": false, "error": "ভুল পে-লোড"}, fmt.Errorf("invalid payload: %v", err)
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		ctx.StatusCode = 400
		return map[string]interface{}{"success": false, "error": "তথ্য পার্স করা যায়নি"}, fmt.Errorf("cannot parse payload: %v", err)
	}

	var amountMinor int64 = req.AmountMinor
	if amountMinor <= 0 && req.Amount != nil {
		switch v := req.Amount.(type) {
		case float64:
			amountMinor = int64(v * 100)
		case int:
			amountMinor = int64(v * 100)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				amountMinor = int64(f * 100)
			}
		}
	}

	if req.CustomerID == "" || amountMinor <= 0 {
		ctx.StatusCode = 400
		return map[string]interface{}{"success": false, "error": "সঠিক গ্রাহক আইডি এবং পরিশোধের পরিমাণ দিন"}, fmt.Errorf("customerId and positive amountMinor are required")
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	cust, err := data.Table("customers").Where("id", "=", req.CustomerID).First(state.pool)
	if err != nil || cust == nil {
		ctx.StatusCode = 404
		return map[string]interface{}{"success": false, "error": "গ্রাহক খুঁজে পাওয়া যায়নি"}, fmt.Errorf("customer not found")
	}

	currDue, _ := toInt64(cust["dueMinor"])
	if currDue <= 0 {
		ctx.StatusCode = 400
		return map[string]interface{}{"success": false, "error": "এই গ্রাহকের কোনো বকেয়া নেই"}, fmt.Errorf("no due balance")
	}

	// ─── STRICT BOUNDARY VALIDATION ───
	// Never allow paying more than current outstanding due!
	if amountMinor > currDue {
		ctx.StatusCode = 400
		errMsg := fmt.Sprintf("আদায়ের পরিমাণ বকেয়া থেকে বেশি হতে পারে না (বর্তমান বকেয়া: %s)", data.NewMoney(currDue, "BDT").Format())
		return map[string]interface{}{"success": false, "error": errMsg}, fmt.Errorf("%s", errMsg)
	}

	newDue := currDue - amountMinor
	cust["dueMinor"] = newDue
	_, _ = data.Table("customers").Where("id", "=", req.CustomerID).Update(state.pool, cust)

	method := req.PaymentMethod
	if method == "" {
		method = req.Method
	}
	if method == "" {
		method = "Cash"
	}

	entryID := fmt.Sprintf("led-%d", time.Now().UnixNano())
	nowUnix := time.Now().Unix()
	entryMap := map[string]interface{}{
		"id":         entryID,
		"customerId": req.CustomerID,
		"dueChange":  -amountMinor,
		"dueAfter":   newDue,
		"note":       fmt.Sprintf("Due collection via %s (%s)", method, req.Notes),
		"timestamp":  nowUnix,
	}
	_, _ = data.Table("ledger_entries").Insert(state.pool, entryMap)

	if state.neonRepo != nil {
		if err := state.neonRepo.PersistCustomerPayment(req.CustomerID, newDue, amountMinor, entryMap); err != nil {
			log.Printf("⚠️ Error persisting to Neon PostgreSQL: %v", err)
		}
	}

	if state.posEngine != nil {
		_, _ = state.posEngine.Customers.RecordDuePayment(req.CustomerID, amountMinor, req.Reference, req.Notes)
	}

	return map[string]interface{}{
		"ok":      true,
		"success": true,
		"data": map[string]interface{}{
			"customer":              cust,
			"collectedAmount":       float64(amountMinor) / 100.0,
			"remainingDue":          float64(newDue) / 100.0,
			"remainingDueMinor":     newDue,
			"remainingDueFormatted": data.NewMoney(newDue, "BDT").Format(),
		},
		"customerId":      req.CustomerID,
		"paidMinor":       amountMinor,
		"newDueMinor":     newDue,
		"newDueFormatted": data.NewMoney(newDue, "BDT").Format(),
	}, nil
}

func handleGetCart(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.posEngine == nil {
		return nil, fmt.Errorf("pos engine not initialized")
	}

	activeCart := state.posEngine.Carts.GetActiveCart()
	return activeCart.Snapshot(), nil
}

// Checkout payload definitions
type CheckoutItem struct {
	ProductID string `json:"productId"`
	Qty       int64  `json:"qty"`
}

type PaymentBreakdown struct {
	CashMinor    int64 `json:"cashMinor"`
	UpiMinor     int64 `json:"upiMinor"`
	PrepaidMinor int64 `json:"prepaidMinor"`
	DueMinor     int64 `json:"dueMinor"`
}

type CheckoutRequest struct {
	CustomerID    string           `json:"customerId"`
	CashierName   string           `json:"cashierName"`
	Items         []CheckoutItem   `json:"items"`
	DiscountMinor int64            `json:"discountMinor"`
	Payment       PaymentBreakdown `json:"payment"`
}

func handleCheckout(ctx *routing.Context) (interface{}, error) {
	var req CheckoutRequest
	bodyBytes, err := json.Marshal(ctx.Body)
	if err != nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("invalid payload: %v", err)
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("cannot parse checkout payload: %v", err)
	}

	if len(req.Items) == 0 {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("cart is empty")
	}

	if req.CustomerID == "" {
		req.CustomerID = "c-walkin"
	}
	if req.CashierName == "" {
		req.CashierName = "Joy Sarkar / জয় সরকার"
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// 1. Fetch and validate customer
	cust, err := data.Table("customers").Where("id", "=", req.CustomerID).First(state.pool)
	if err != nil || cust == nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("customer not found")
	}

	custName := fmt.Sprintf("%v", cust["name"])
	currDue, _ := toInt64(cust["dueMinor"])
	currPrepaid, _ := toInt64(cust["prepaidMinor"])
	creditLimit, _ := toInt64(cust["creditLimitMinor"])

	// Check prepaid availability
	if req.Payment.PrepaidMinor > 0 {
		if currPrepaid < req.Payment.PrepaidMinor {
			ctx.StatusCode = 400
			return nil, fmt.Errorf("insufficient prepaid balance (available: %s)", data.NewMoney(currPrepaid, "BDT").Format())
		}
	}

	// Check due credit limit
	if req.Payment.DueMinor > 0 {
		if req.CustomerID == "c-walkin" {
			ctx.StatusCode = 400
			return nil, fmt.Errorf("walk-in customers cannot purchase on due/credit")
		}
		if (currDue + req.Payment.DueMinor) > creditLimit {
			ctx.StatusCode = 400
			return nil, fmt.Errorf("credit limit exceeded! Current Due: %s, Limit: %s",
				data.NewMoney(currDue, "BDT").Format(), data.NewMoney(creditLimit, "BDT").Format())
		}
	}

	// 2. Fetch and validate items and stock
	type processedItem struct {
		Product   map[string]interface{}
		Qty       int64
		UnitPrice int64
		CostPrice int64
		LineTotal int64
	}

	var processed []processedItem
	var subtotalMinor int64 = 0
	var totalCogsMinor int64 = 0

	for _, it := range req.Items {
		if it.Qty <= 0 {
			ctx.StatusCode = 400
			return nil, fmt.Errorf("quantity must be greater than zero")
		}
		prod, err := data.Table("products").Where("id", "=", it.ProductID).First(state.pool)
		if err != nil || prod == nil {
			ctx.StatusCode = 400
			return nil, fmt.Errorf("product %s not found", it.ProductID)
		}

		currentStock, _ := toInt64(prod["stock"])
		if currentStock < it.Qty {
			ctx.StatusCode = 400
			return nil, fmt.Errorf("insufficient stock for '%s' (available: %d, requested: %d)",
				prod["name"], currentStock, it.Qty)
		}

		uPrice, _ := toInt64(prod["priceMinor"])
		wCost, _ := toInt64(prod["wacMinor"])
		lTotal := uPrice * it.Qty

		subtotalMinor += lTotal
		totalCogsMinor += (wCost * it.Qty)

		processed = append(processed, processedItem{
			Product:   prod,
			Qty:       it.Qty,
			UnitPrice: uPrice,
			CostPrice: wCost,
			LineTotal: lTotal,
		})
	}

	// 3. Exact calculations
	disc := req.DiscountMinor
	if disc > subtotalMinor {
		disc = subtotalMinor
	}
	afterDisc := subtotalMinor - disc
	taxMinor := (afterDisc * 5) / 100
	grandMinor := afterDisc + taxMinor

	totalTendered := req.Payment.CashMinor + req.Payment.UpiMinor + req.Payment.PrepaidMinor + req.Payment.DueMinor
	if totalTendered < grandMinor {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("underpaid! required: %s, received: %s",
			data.NewMoney(grandMinor, "BDT").Format(), data.NewMoney(totalTendered, "BDT").Format())
	}

	changeMinor := totalTendered - grandMinor

	// 4. Atomic Execution inside Transaction
	saleID := fmt.Sprintf("SALE-%d", time.Now().UnixNano()/1000000)
	invoiceNo := fmt.Sprintf("INV-%06d", time.Now().Unix()%1000000)
	nowUnix := time.Now().Unix()
	var saleRecord map[string]interface{}

	txErr := state.pool.Transaction(func(tx *data.Tx) error {
		// A. Deduct stock for all items and record stock_history
		for _, pi := range processed {
			newStock := pi.Product["stock"].(int64) - pi.Qty
			pi.Product["stock"] = newStock
			_, err := data.Table("products").Where("id", "=", pi.Product["id"]).Update(state.pool, pi.Product)
			if err != nil {
				return err
			}

			_, _ = data.Table("stock_history").Insert(state.pool, map[string]interface{}{
				"id":         fmt.Sprintf("sh-%d", time.Now().UnixNano()),
				"productId":  pi.Product["id"],
				"type":       "SALE",
				"qtyChange":  -pi.Qty,
				"stockAfter": newStock,
				"reason":     "Invoice: " + invoiceNo,
				"timestamp":  nowUnix,
			})
		}

		// B. Update Customer Balances and Ledger
		if req.Payment.DueMinor > 0 || req.Payment.PrepaidMinor > 0 {
			newDue := currDue + req.Payment.DueMinor
			newPrepaid := currPrepaid - req.Payment.PrepaidMinor

			cust["dueMinor"] = newDue
			cust["prepaidMinor"] = newPrepaid
			_, err := data.Table("customers").Where("id", "=", req.CustomerID).Update(state.pool, cust)
			if err != nil {
				return err
			}

			_, _ = data.Table("ledger_entries").Insert(state.pool, map[string]interface{}{
				"id":            fmt.Sprintf("led-%d", time.Now().UnixNano()),
				"customerId":    req.CustomerID,
				"saleId":        saleID,
				"dueChange":     req.Payment.DueMinor,
				"prepaidChange": -req.Payment.PrepaidMinor,
				"dueAfter":      newDue,
				"prepaidAfter":  newPrepaid,
				"note":          "Invoice: " + invoiceNo,
				"timestamp":     nowUnix,
			})
		}

		// C. Insert Sale
		saleRecord = map[string]interface{}{
			"id":               saleID,
			"invoiceNo":        invoiceNo,
			"customerId":       req.CustomerID,
			"customerName":     custName,
			"cashierName":      req.CashierName,
			"subtotalMinor":    subtotalMinor,
			"discountMinor":    disc,
			"taxMinor":         taxMinor,
			"grandMinor":       grandMinor,
			"cogsMinor":        totalCogsMinor,
			"cashPaidMinor":    req.Payment.CashMinor,
			"upiPaidMinor":     req.Payment.UpiMinor,
			"prepaidPaidMinor": req.Payment.PrepaidMinor,
			"duePaidMinor":     req.Payment.DueMinor,
			"changeMinor":      changeMinor,
			"timestamp":        nowUnix,
		}
		_, err = data.Table("sales").Insert(state.pool, saleRecord)
		if err != nil {
			return err
		}

		// D. Insert Sale Items
		for _, pi := range processed {
			_, err = data.Table("sale_items").Insert(state.pool, map[string]interface{}{
				"id":             fmt.Sprintf("si-%d", time.Now().UnixNano()),
				"saleId":         saleID,
				"productId":      pi.Product["id"],
				"productName":    pi.Product["name"],
				"sku":            pi.Product["sku"],
				"unitPriceMinor": pi.UnitPrice,
				"costMinor":      pi.CostPrice,
				"qty":            pi.Qty,
				"lineTotalMinor": pi.LineTotal,
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if txErr != nil {
		ctx.StatusCode = 500
		return nil, fmt.Errorf("transaction aborted: %v", txErr)
	}

	// Persist atomically to Neon PostgreSQL Cloud Database
	if state.neonRepo != nil {
		var stockUpdates []map[string]interface{}
		var stockHistories []map[string]interface{}
		var saleItems []map[string]interface{}

		for _, pi := range processed {
			newStk := pi.Product["stock"].(int64)
			stockUpdates = append(stockUpdates, map[string]interface{}{
				"id":    pi.Product["id"],
				"stock": newStk,
			})
			stockHistories = append(stockHistories, map[string]interface{}{
				"id":         fmt.Sprintf("sh-%d", time.Now().UnixNano()),
				"productId":  pi.Product["id"],
				"type":       "SALE",
				"qtyChange":  -pi.Qty,
				"stockAfter": newStk,
				"reason":     "Invoice: " + invoiceNo,
				"timestamp":  nowUnix,
			})
			saleItems = append(saleItems, map[string]interface{}{
				"id":             fmt.Sprintf("si-%d", time.Now().UnixNano()),
				"saleId":         saleID,
				"productId":      pi.Product["id"],
				"productName":    pi.Product["name"],
				"sku":            pi.Product["sku"],
				"unitPriceMinor": pi.UnitPrice,
				"costMinor":      pi.CostPrice,
				"qty":            pi.Qty,
				"lineTotalMinor": pi.LineTotal,
			})
		}

		var custUp map[string]interface{}
		var ledEntry map[string]interface{}
		if req.Payment.DueMinor > 0 || req.Payment.PrepaidMinor > 0 {
			custUp = cust
			ledEntry = map[string]interface{}{
				"id":            fmt.Sprintf("led-%d", time.Now().UnixNano()),
				"customerId":    req.CustomerID,
				"saleId":        saleID,
				"dueChange":     req.Payment.DueMinor,
				"prepaidChange": -req.Payment.PrepaidMinor,
				"dueAfter":      cust["dueMinor"],
				"prepaidAfter":  cust["prepaidMinor"],
				"note":          "Invoice: " + invoiceNo,
				"timestamp":     nowUnix,
			}
		}

		if err := state.neonRepo.PersistCheckout(saleRecord, saleItems, stockUpdates, stockHistories, custUp, ledEntry); err != nil {
			log.Printf("⚠️ Error persisting checkout to Neon PostgreSQL: %v", err)
		}
	}

	// 5. Build Receipt via Device Formatter
	var receiptItems []map[string]interface{}
	receiptLines := make([]device.ReceiptLineItem, len(processed))
	for i, pi := range processed {
		receiptItems = append(receiptItems, map[string]interface{}{
			"productId":          pi.Product["id"],
			"name":               pi.Product["name"],
			"nameBn":             pi.Product["nameBn"],
			"qty":                pi.Qty,
			"unit":               pi.Product["unit"],
			"unitPriceFormatted": data.NewMoney(pi.UnitPrice, "BDT").Format(),
			"lineTotalFormatted": data.NewMoney(pi.LineTotal, "BDT").Format(),
		})

		receiptLines[i] = device.ReceiptLineItem{
			Name:     fmt.Sprintf("%v", pi.Product["name"]),
			Quantity: fmt.Sprintf("%d %v", pi.Qty, pi.Product["unit"]),
			Price:    data.NewMoney(pi.UnitPrice, "BDT").Format(),
			Total:    data.NewMoney(pi.LineTotal, "BDT").Format(),
		}
	}

	receiptData := device.ReceiptPayload{
		StoreName:     "লাখান ভাণ্ডার (Lakhan Bhandar)",
		StoreSubtitle: "Wholesale & Retail Groceries",
		InvoiceNo:     invoiceNo,
		DateStr:       time.Unix(nowUnix, 0).Format("02/01/2006 03:04 PM"),
		Cashier:       req.CashierName,
		Customer:      custName,
		Items:         receiptLines,
		Subtotal:      data.NewMoney(subtotalMinor, "BDT").Format(),
		Discount:      data.NewMoney(disc, "BDT").Format(),
		Tax:           data.NewMoney(taxMinor, "BDT").Format(),
		GrandTotal:    data.NewMoney(grandMinor, "BDT").Format(),
		PaymentMethod: "Split Payment",
		PaidAmount:    data.NewMoney(totalTendered, "BDT").Format(),
		ChangeDue:     data.NewMoney(changeMinor, "BDT").Format(),
		FooterNote:    i18n.T(i18n.LocaleBnBD, "receipt.thank_you"),
	}

	formatter := device.NewESCPOSFormatter(device.Width58mm)
	receiptText := formatter.FormatPlainText(receiptData)

	hasCash := req.Payment.CashMinor > 0

	// Also record sale in Shift & Audit if engine is active
	if state.posEngine != nil {
		pRecs := make([]pos.PaymentRecord, 0)
		if req.Payment.CashMinor > 0 {
			pRecs = append(pRecs, pos.PaymentRecord{Method: pos.MethodCash, AmountMinor: req.Payment.CashMinor})
		}
		if req.Payment.UpiMinor > 0 {
			pRecs = append(pRecs, pos.PaymentRecord{Method: pos.MethodBKash, AmountMinor: req.Payment.UpiMinor})
		}
		if req.Payment.DueMinor > 0 {
			pRecs = append(pRecs, pos.PaymentRecord{Method: pos.MethodCredit, AmountMinor: req.Payment.DueMinor})
		}

		sObj := &pos.Sale{
			ID:            saleID,
			InvoiceNumber: invoiceNo,
			RegisterID:    "reg-01",
			CashierID:     req.CashierName,
			CustomerID:    req.CustomerID,
			SubtotalMinor: subtotalMinor,
			DiscountMinor: disc,
			TaxMinor:      taxMinor,
			TotalMinor:    grandMinor,
			PaidMinor:     totalTendered,
			ChangeMinor:   changeMinor,
			Status:        pos.StatusCompleted,
			Payments:      pRecs,
			CreatedAt:     time.Unix(nowUnix, 0),
		}
		_ = state.posEngine.Shifts.RecordSale(sObj)
		state.posEngine.Audit.Record(
			pos.ActionSaleCompleted,
			saleID,
			req.CashierName,
			nil,
			map[string]interface{}{"invoice": invoiceNo, "total": grandMinor},
			"POS Checkout",
		)
	}

	return map[string]interface{}{
		"ok":                true,
		"saleId":            saleID,
		"invoiceNo":         invoiceNo,
		"date":              time.Unix(nowUnix, 0).Format("02 Jan 2006, 03:04 PM"),
		"customerId":        req.CustomerID,
		"customerName":      custName,
		"cashierName":       req.CashierName,
		"items":             receiptItems,
		"subtotalFormatted": data.NewMoney(subtotalMinor, "BDT").Format(),
		"discountFormatted": data.NewMoney(disc, "BDT").Format(),
		"taxFormatted":      data.NewMoney(taxMinor, "BDT").Format(),
		"grandMinor":        grandMinor,
		"grandFormatted":    data.NewMoney(grandMinor, "BDT").Format(),
		"payment": map[string]interface{}{
			"cash":    data.NewMoney(req.Payment.CashMinor, "BDT").Format(),
			"upi":     data.NewMoney(req.Payment.UpiMinor, "BDT").Format(),
			"prepaid": data.NewMoney(req.Payment.PrepaidMinor, "BDT").Format(),
			"due":     data.NewMoney(req.Payment.DueMinor, "BDT").Format(),
		},
		"changeMinor":     changeMinor,
		"changeFormatted": data.NewMoney(changeMinor, "BDT").Format(),
		"dueAfter":        cust["dueMinor"],
		"prepaidAfter":    cust["prepaidMinor"],
		"receiptText":     receiptText,
		"triggerDrawer":   hasCash,
	}, nil
}

type RefundAPIRequest struct {
	SaleID    string `json:"saleId"`
	ProductID string `json:"productId"`
	Qty       int64  `json:"qty"`
	Reason    string `json:"reason"`
	CashierID string `json:"cashierId"`
}

func handleRefund(ctx *routing.Context) (interface{}, error) {
	var req RefundAPIRequest
	bodyBytes, err := json.Marshal(ctx.Body)
	if err != nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("invalid payload: %v", err)
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("cannot parse payload: %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	sale, err := data.Table("sales").Where("id", "=", req.SaleID).First(state.pool)
	if err != nil || sale == nil {
		ctx.StatusCode = 404
		return nil, fmt.Errorf("sale not found: %s", req.SaleID)
	}

	prod, err := data.Table("products").Where("id", "=", req.ProductID).First(state.pool)
	if err != nil || prod == nil {
		ctx.StatusCode = 404
		return nil, fmt.Errorf("product not found: %s", req.ProductID)
	}

	// Restore stock
	currStock, _ := toInt64(prod["stock"])
	newStock := currStock + req.Qty
	prod["stock"] = newStock
	_, _ = data.Table("products").Where("id", "=", req.ProductID).Update(state.pool, prod)

	uPrice, _ := toInt64(prod["priceMinor"])
	refundMinor := uPrice * req.Qty

	shMap := map[string]interface{}{
		"id":         fmt.Sprintf("sh-%d", time.Now().UnixNano()),
		"productId":  req.ProductID,
		"type":       "RETURN",
		"qtyChange":  req.Qty,
		"stockAfter": newStock,
		"reason":     fmt.Sprintf("Refund for %s (Reason: %s)", sale["invoiceNo"], req.Reason),
		"timestamp":  time.Now().Unix(),
	}
	_, _ = data.Table("stock_history").Insert(state.pool, shMap)

	if state.neonRepo != nil {
		_ = state.neonRepo.PersistRefund(req.ProductID, newStock, shMap)
	}

	return map[string]interface{}{
		"ok":                true,
		"saleId":            req.SaleID,
		"refundMinor":       refundMinor,
		"refundFormatted":   data.NewMoney(refundMinor, "BDT").Format(),
		"restoredStock":     newStock,
		"message":           "Refund processed and inventory restored.",
	}, nil
}

func handleDailyReport(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	sales, _ := data.Table("sales").Get(state.pool)
	products, _ := data.Table("products").Get(state.pool)

	var totalSalesMinor int64 = 0
	var totalCogsMinor int64 = 0
	var totalCashMinor int64 = 0
	var totalUpiMinor int64 = 0
	var totalPrepaidMinor int64 = 0
	var totalDueMinor int64 = 0

	for _, s := range sales {
		g, _ := toInt64(s["grandMinor"])
		c, _ := toInt64(s["cogsMinor"])
		cp, _ := toInt64(s["cashPaidMinor"])
		up, _ := toInt64(s["upiPaidMinor"])
		pp, _ := toInt64(s["prepaidPaidMinor"])
		dp, _ := toInt64(s["duePaidMinor"])

		totalSalesMinor += g
		totalCogsMinor += c
		totalCashMinor += cp
		totalUpiMinor += up
		totalPrepaidMinor += pp
		totalDueMinor += dp
	}

	grossProfitMinor := totalSalesMinor - totalCogsMinor

	// Inventory valuation: sum of (stock * WAC)
	var inventoryValueMinor int64 = 0
	for _, p := range products {
		stk, _ := toInt64(p["stock"])
		wac, _ := toInt64(p["wacMinor"])
		inventoryValueMinor += (stk * wac)
	}

	return map[string]interface{}{
		"totalTransactions":  len(sales),
		"totalSales":         data.NewMoney(totalSalesMinor, "BDT").Format(),
		"totalSalesMinor":    totalSalesMinor,
		"grossProfit":        data.NewMoney(grossProfitMinor, "BDT").Format(),
		"grossProfitMinor":   grossProfitMinor,
		"cogs":               data.NewMoney(totalCogsMinor, "BDT").Format(),
		"cashCollected":      data.NewMoney(totalCashMinor, "BDT").Format(),
		"upiCollected":       data.NewMoney(totalUpiMinor, "BDT").Format(),
		"prepaidRedeemed":    data.NewMoney(totalPrepaidMinor, "BDT").Format(),
		"dueGiven":           data.NewMoney(totalDueMinor, "BDT").Format(),
		"inventoryValuation": data.NewMoney(inventoryValueMinor, "BDT").Format(),
		"recentSales":        sales,
	}, nil
}

func handleExportReport(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	sales, _ := data.Table("sales").Get(state.pool)
	var csvBuf string
	csvBuf += "Invoice Number,Date,Customer ID,Total (BDT),Change (BDT)\n"
	for _, s := range sales {
		g, _ := toInt64(s["grandMinor"])
		c, _ := toInt64(s["changeMinor"])
		csvBuf += fmt.Sprintf("%v,%v,%v,%s,%s\n",
			s["invoiceNo"], s["timestamp"], s["customerId"],
			data.NewMoney(g, "BDT").Format(), data.NewMoney(c, "BDT").Format())
	}

	return server.TextResponse{Text: csvBuf}, nil
}

type StockAdjustRequest struct {
	ProductID         string `json:"productId"`
	AddedQty          int64  `json:"addedQty"`
	PurchaseUnitMinor int64  `json:"purchaseUnitMinor"`
	Reason            string `json:"reason"`
}

func handleInventoryAdjust(ctx *routing.Context) (interface{}, error) {
	var req StockAdjustRequest
	bodyBytes, err := json.Marshal(ctx.Body)
	if err != nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("invalid payload: %v", err)
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("cannot parse adjust payload: %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	prod, err := data.Table("products").Where("id", "=", req.ProductID).First(state.pool)
	if err != nil || prod == nil {
		ctx.StatusCode = 404
		return nil, fmt.Errorf("product not found")
	}

	currStock, _ := toInt64(prod["stock"])
	currWac, _ := toInt64(prod["wacMinor"])

	newStock := currStock + req.AddedQty
	if newStock < 0 {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("stock cannot be negative")
	}

	newWac := currWac
	if req.AddedQty > 0 && req.PurchaseUnitMinor > 0 {
		totalCost := (currStock * currWac) + (req.AddedQty * req.PurchaseUnitMinor)
		newWac = totalCost / newStock
	}

	prod["stock"] = newStock
	prod["wacMinor"] = newWac
	_, _ = data.Table("products").Where("id", "=", req.ProductID).Update(state.pool, prod)

	shMap := map[string]interface{}{
		"id":         fmt.Sprintf("sh-%d", time.Now().UnixNano()),
		"productId":  req.ProductID,
		"type":       "ADJUSTMENT",
		"qtyChange":  req.AddedQty,
		"stockAfter": newStock,
		"reason":     req.Reason,
		"timestamp":  time.Now().Unix(),
	}
	_, _ = data.Table("stock_history").Insert(state.pool, shMap)

	if state.neonRepo != nil {
		_ = state.neonRepo.PersistInventoryAdjust(req.ProductID, newStock, newWac, shMap)
	}

	return map[string]interface{}{
		"ok":                true,
		"product":           prod,
		"newWacFormatted":   data.NewMoney(newWac, "BDT").Format(),
		"newStockFormatted": fmt.Sprintf("%d %v", newStock, prod["unit"]),
	}, nil
}

func handleInventoryMovements(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	history, err := data.Table("stock_history").Get(state.pool)
	if err != nil {
		return []map[string]interface{}{}, nil
	}
	return history, nil
}

func handleShiftCurrent(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.posEngine != nil {
		shift, err := state.posEngine.Shifts.CurrentShift()
		if err == nil {
			return shift, nil
		}
	}
	return map[string]interface{}{
		"status":      "OPEN",
		"register_id": "reg-01",
		"cashier_id":  "cashier-01",
		"cashier_name": "জয় সরকার (Joy Sarkar)",
		"start_time":  state.startTime,
	}, nil
}

type ShiftOpenRequest struct {
	CashierName       string `json:"cashierName"`
	StartingCashMinor int64  `json:"startingCashMinor"`
}

func handleShiftOpen(ctx *routing.Context) (interface{}, error) {
	var req ShiftOpenRequest
	bodyBytes, _ := json.Marshal(ctx.Body)
	_ = json.Unmarshal(bodyBytes, &req)

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.posEngine != nil {
		s, err := state.posEngine.Shifts.OpenShift("reg-01", "cashier-01", req.CashierName, req.StartingCashMinor)
		if err != nil {
			ctx.StatusCode = 400
			return nil, err
		}
		return s, nil
	}
	return map[string]interface{}{"ok": true}, nil
}

type ShiftCloseRequest struct {
	ActualCashMinor int64  `json:"actualCashMinor"`
	Role            string `json:"role,omitempty"`
	Notes           string `json:"notes"`
}

func handleShiftClose(ctx *routing.Context) (interface{}, error) {
	var req ShiftCloseRequest
	bodyBytes, _ := json.Marshal(ctx.Body)
	_ = json.Unmarshal(bodyBytes, &req)

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.posEngine != nil {
		role := req.Role
		if role == "" {
			role = pos.RoleManager
		}
		s, err := state.posEngine.Shifts.CloseShift(req.ActualCashMinor, req.Notes, role)
		if err != nil {
			ctx.StatusCode = 400
			return nil, err
		}
		return s, nil
	}
	return map[string]interface{}{"ok": true}, nil
}

func handleSyncPush(ctx *routing.Context) (interface{}, error) {
	var ops []*alapsync.MutationOperation
	bodyBytes, err := json.Marshal(ctx.Body)
	if err != nil {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("invalid payload: %v", err)
	}

	// Support single op or list
	if err := json.Unmarshal(bodyBytes, &ops); err != nil {
		var single alapsync.MutationOperation
		if err2 := json.Unmarshal(bodyBytes, &single); err2 == nil {
			ops = []*alapsync.MutationOperation{&single}
		} else {
			ctx.StatusCode = 400
			return nil, fmt.Errorf("cannot parse sync mutations: %v", err)
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.posEngine != nil {
		successes, failures, err := state.posEngine.Sync.Process(ops)
		return map[string]interface{}{
			"successCount": len(successes),
			"failedCount":  len(failures),
			"successes":    successes,
			"failures":     failures,
			"error":        err,
		}, nil
	}

	return map[string]interface{}{"ok": true}, nil
}

func handleSyncPending(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.posEngine != nil {
		return state.posEngine.SyncQueue.Pending(), nil
	}
	return []interface{}{}, nil
}

func handlePrinterTest(ctx *routing.Context) (interface{}, error) {
	formatter := device.NewESCPOSFormatter(device.Width58mm)
	payload := device.ReceiptPayload{
		StoreName:     "লাখান ভাণ্ডার (Lakhan Bhandar)",
		StoreSubtitle: "*** HARDWARE TEST PRINT ***",
		InvoiceNo:     "TEST-000",
		DateStr:       time.Now().Format("02/01/2006 03:04 PM"),
		Cashier:       "Joy Sarkar",
		GrandTotal:    "৳0.00",
		FooterNote:    "Thermal printer connection verified!",
	}
	return map[string]interface{}{
		"ok":          true,
		"preview":     formatter.FormatPlainText(payload),
		"width":       "58mm",
		"status":      "ready",
	}, nil
}

func handleDrawerOpen(ctx *routing.Context) (interface{}, error) {
	drawer := device.NewCashDrawer()
	pulse := drawer.Open()
	return map[string]interface{}{
		"ok":        true,
		"pulseBytes": fmt.Sprintf("%x", pulse),
		"message":   "Cash drawer kickout pulse sent",
	}, nil
}

func handleAuditRecent(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.posEngine != nil {
		return state.posEngine.Audit.RecentEntries(50), nil
	}
	return []interface{}{}, nil
}

// ─── NEW FEATURE HANDLERS ───────────────────────────────────────────────────

func handleSales(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	sales, err := data.Table("sales").Get(state.pool)
	if err != nil {
		return []map[string]interface{}{}, nil
	}

	enriched := make([]map[string]interface{}, len(sales))
	for i, s := range sales {
		row := make(map[string]interface{})
		for k, v := range s {
			row[k] = v
		}
		grand, _ := toInt64(s["grandMinor"])
		cogs, _ := toInt64(s["cogsMinor"])
		row["grandFormatted"] = data.NewMoney(grand, "BDT").Format()
		row["profitMinor"] = grand - cogs
		row["profitFormatted"] = data.NewMoney(grand-cogs, "BDT").Format()
		enriched[i] = row
	}
	return enriched, nil
}

func handleDashboardStats(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	sales, _ := data.Table("sales").Get(state.pool)
	products, _ := data.Table("products").Get(state.pool)
	customers, _ := data.Table("customers").Get(state.pool)
	suppliers, _ := data.Table("suppliers").Get(state.pool)
	expenses, _ := data.Table("expenses").Get(state.pool)

	var totalSalesMinor, totalCogsMinor, totalExpensesMinor, totalCustomerDueMinor int64
	for _, s := range sales {
		g, _ := toInt64(s["grandMinor"])
		c, _ := toInt64(s["cogsMinor"])
		totalSalesMinor += g
		totalCogsMinor += c
	}
	for _, c := range customers {
		d, _ := toInt64(c["dueMinor"])
		totalCustomerDueMinor += d
	}
	for _, e := range expenses {
		a, _ := toInt64(e["amountMinor"])
		totalExpensesMinor += a
	}
	var inventoryValueMinor int64
	var lowStockCount int64
	for _, p := range products {
		stk, _ := toInt64(p["stock"])
		wac, _ := toInt64(p["wacMinor"])
		low, _ := toInt64(p["lowStock"])
		inventoryValueMinor += stk * wac
		if stk <= low {
			lowStockCount++
		}
	}

	grossProfit := totalSalesMinor - totalCogsMinor
	netProfit := grossProfit - totalExpensesMinor

	return map[string]interface{}{
		"todaySales":           data.NewMoney(totalSalesMinor, "BDT").Format(),
		"todaySalesMinor":      totalSalesMinor,
		"todayTransactions":    len(sales),
		"grossProfit":          data.NewMoney(grossProfit, "BDT").Format(),
		"grossProfitMinor":     grossProfit,
		"netProfit":            data.NewMoney(netProfit, "BDT").Format(),
		"netProfitMinor":       netProfit,
		"totalCustomers":       len(customers),
		"totalSuppliers":       len(suppliers),
		"totalProducts":        len(products),
		"lowStockProducts":     lowStockCount,
		"totalCustomerDue":     data.NewMoney(totalCustomerDueMinor, "BDT").Format(),
		"totalCustomerDueMinor": totalCustomerDueMinor,
		"inventoryValue":       data.NewMoney(inventoryValueMinor, "BDT").Format(),
		"inventoryValueMinor":  inventoryValueMinor,
		"totalExpenses":        data.NewMoney(totalExpensesMinor, "BDT").Format(),
		"totalExpensesMinor":   totalExpensesMinor,
	}, nil
}

type AddExpenseRequest struct {
	Description string `json:"description"`
	Category    string `json:"category"`
	AmountMinor int64  `json:"amountMinor"`
	PaidBy      string `json:"paidBy"`
}

func handleExpenses(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	expenses, err := data.Table("expenses").Get(state.pool)
	if err != nil {
		return []map[string]interface{}{}, nil
	}
	enriched := make([]map[string]interface{}, len(expenses))
	for i, e := range expenses {
		row := make(map[string]interface{})
		for k, v := range e {
			row[k] = v
		}
		amt, _ := toInt64(e["amountMinor"])
		row["amountFormatted"] = data.NewMoney(amt, "BDT").Format()
		enriched[i] = row
	}
	return enriched, nil
}

func handleAddExpense(ctx *routing.Context) (interface{}, error) {
	var req AddExpenseRequest
	if bodyBytes, err := json.Marshal(ctx.Body); err == nil {
		_ = json.Unmarshal(bodyBytes, &req)
	}
	if req.Description == "" || req.AmountMinor <= 0 {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("description and positive amount required")
	}
	if req.Category == "" {
		req.Category = "General"
	}
	if req.PaidBy == "" {
		req.PaidBy = "জয় সরকার"
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	expID := fmt.Sprintf("exp-%d", time.Now().UnixNano())
	expense := map[string]interface{}{
		"id": expID, "description": req.Description, "category": req.Category,
		"amountMinor": req.AmountMinor, "paidBy": req.PaidBy, "timestamp": time.Now().Unix(),
	}
	_, err := data.Table("expenses").Insert(state.pool, expense)
	if err != nil {
		ctx.StatusCode = 500
		return nil, fmt.Errorf("failed to add expense")
	}
	if state.neonRepo != nil {
		_ = state.neonRepo.PersistExpense(expense)
	}
	expense["amountFormatted"] = data.NewMoney(req.AmountMinor, "BDT").Format()
	return map[string]interface{}{"ok": true, "expense": expense}, nil
}

func handleSuppliers(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	suppliers, err := data.Table("suppliers").Get(state.pool)
	if err != nil {
		return []map[string]interface{}{}, nil
	}
	enriched := make([]map[string]interface{}, len(suppliers))
	for i, s := range suppliers {
		row := make(map[string]interface{})
		for k, v := range s {
			row[k] = v
		}
		due, _ := toInt64(s["dueMinor"])
		limit, _ := toInt64(s["creditLimitMinor"])
		totalPurchase, _ := toInt64(s["totalPurchaseMinor"])
		row["dueFormatted"] = data.NewMoney(due, "BDT").Format()
		row["creditLimitFormatted"] = data.NewMoney(limit, "BDT").Format()
		row["totalPurchaseFormatted"] = data.NewMoney(totalPurchase, "BDT").Format()
		enriched[i] = row
	}
	return enriched, nil
}

func handleAddSupplier(ctx *routing.Context) (interface{}, error) {
	type req struct {
		Name    string `json:"name"`
		NameBn  string `json:"nameBn"`
		Phone   string `json:"phone"`
		Address string `json:"address"`
	}
	var r req
	if bodyBytes, err := json.Marshal(ctx.Body); err == nil {
		_ = json.Unmarshal(bodyBytes, &r)
	}
	if r.Name == "" {
		ctx.StatusCode = 400
		return nil, fmt.Errorf("supplier name required")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	supID := fmt.Sprintf("sup-%d", time.Now().UnixNano())
	supplier := map[string]interface{}{
		"id": supID, "name": r.Name, "nameBn": r.NameBn, "phone": r.Phone,
		"address": r.Address, "dueMinor": int64(0), "creditLimitMinor": int64(1000000), "totalPurchaseMinor": int64(0),
	}
	_, _ = data.Table("suppliers").Insert(state.pool, supplier)
	if state.neonRepo != nil {
		_ = state.neonRepo.PersistSupplier(supplier)
	}
	return map[string]interface{}{"ok": true, "supplier": supplier}, nil
}

func handlePurchaseOrders(ctx *routing.Context) (interface{}, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	orders, err := data.Table("purchase_orders").Get(state.pool)
	if err != nil {
		return []map[string]interface{}{}, nil
	}
	enriched := make([]map[string]interface{}, len(orders))
	for i, o := range orders {
		row := make(map[string]interface{})
		for k, v := range o {
			row[k] = v
		}
		total, _ := toInt64(o["totalMinor"])
		row["totalFormatted"] = data.NewMoney(total, "BDT").Format()
		enriched[i] = row
	}
	return enriched, nil
}

func handleAddPurchaseOrder(ctx *routing.Context) (interface{}, error) {
	type req struct {
		SupplierID   string `json:"supplierId"`
		SupplierName string `json:"supplierName"`
		Notes        string `json:"notes"`
		TotalMinor   int64  `json:"totalMinor"`
	}
	var r req
	if bodyBytes, err := json.Marshal(ctx.Body); err == nil {
		_ = json.Unmarshal(bodyBytes, &r)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	poID := fmt.Sprintf("po-%d", time.Now().UnixNano())
	order := map[string]interface{}{
		"id": poID, "supplierId": r.SupplierID, "supplierName": r.SupplierName,
		"status": "PENDING", "totalMinor": r.TotalMinor, "itemCount": int64(1),
		"notes": r.Notes, "timestamp": time.Now().Unix(),
	}
	_, _ = data.Table("purchase_orders").Insert(state.pool, order)
	if state.neonRepo != nil {
		_ = state.neonRepo.PersistPurchaseOrder(order)
	}
	order["totalFormatted"] = data.NewMoney(r.TotalMinor, "BDT").Format()
	return map[string]interface{}{"ok": true, "order": order}, nil
}

// ─── STATIC ASSET SERVING ───────────────────────────────────────────────────

func handleIndexHTML(ctx *routing.Context) (interface{}, error) {
	// Serve the static public/index.html directly
	// The Alap runtime + API calls handle dynamic data via AJAX
	htmlPath := filepath.Join("public", "index.html")
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		// Fallback to SSR renderer if file missing
		state.mu.RLock()
		defer state.mu.RUnlock()
		products, _ := data.Table("products").Get(state.pool)
		customers, _ := data.Table("customers").Get(state.pool)
		htmlContent := RenderAlapPOSPage(products, customers)
		return server.HTMLResponse{HTML: htmlContent}, nil
	}
	return server.HTMLResponse{HTML: string(content)}, nil
}

func handleStyleCSS(ctx *routing.Context) (interface{}, error) {
	cssPath := filepath.Join("public", "style.css")
	content, err := os.ReadFile(cssPath)
	if err != nil {
		ctx.StatusCode = 404
		return server.CSSResponse{CSS: "/* style.css not found */"}, nil
	}
	return server.CSSResponse{CSS: string(content)}, nil
}

func handleAlapRuntimeJS(ctx *routing.Context) (interface{}, error) {
	jsPath := filepath.Join("public", "alap-runtime.js")
	content, err := os.ReadFile(jsPath)
	if err != nil {
		ctx.StatusCode = 404
		return server.JSResponse{JS: "console.error('alap-runtime.js not found');"}, nil
	}
	return server.JSResponse{JS: string(content)}, nil
}

func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}
