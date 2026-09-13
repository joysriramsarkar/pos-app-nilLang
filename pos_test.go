package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
	"github.com/joysriramsarkar/nilLang/pkg/alap/server"
	"github.com/joysriramsarkar/nilLang/pkg/alap/sync"
)

func setupTestApp() *server.Service {
	pool := data.NewDBPool(data.DBPoolConfig{
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		Driver:       data.DriverPostgres,
	})

	engine := pos.NewPOSEngine()
	engine.SeedDefaultEnterpriseData()

	state = &AppState{
		pool:      pool,
		posEngine: engine,
		startTime: time.Now(),
	}

	seedDatabase(pool)

	srv := server.NewService("test-pos", "")
	srv.GET("/api/health", handleHealth)
	srv.GET("/api/catalog", handleCatalog)
	srv.GET("/api/products", handleCatalog)
	srv.GET("/api/customers", handleCustomers)
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
	srv.POST("/api/device/drawer/open", handleDrawerOpen)
	srv.GET("/api/audit/recent", handleAuditRecent)
	srv.GET("/", handleIndexHTML)
	srv.GET("/index.html", handleIndexHTML)
	srv.GET("/style.css", handleStyleCSS)
	srv.GET("/alap-runtime.js", handleAlapRuntimeJS)
	return srv
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestApp()

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp["status"])
	}
}

func TestCatalogEndpoint(t *testing.T) {
	srv := setupTestApp()

	req := httptest.NewRequest("GET", "/api/catalog", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	products, ok := resp["products"].([]interface{})
	if !ok || len(products) < 10 {
		t.Fatalf("expected >= 10 products, got %d", len(products))
	}
}

func TestCheckoutAtomicTransaction(t *testing.T) {
	srv := setupTestApp()

	// Cart with:
	// 1x Miniket Rice (p-01): ৳3,400.00 (340000)
	// 2x Fresh Atta (p-10): 2 x ৳125.00 = ৳250.00 (25000)
	// Subtotal = ৳3,650.00 (365000)
	// Discount = ৳50.00 (5000)
	// After Discount = ৳3,600.00 (360000)
	// 5% Tax = ৳180.00 (18000)
	// Grand Total = ৳3,780.00 (378000)
	checkoutPayload := CheckoutRequest{
		CustomerID:  "c-02", // Karim Bhai with ৳500.00 prepaid
		CashierName: "Joy Sarkar",
		Items: []CheckoutItem{
			{ProductID: "p-01", Qty: 1},
			{ProductID: "p-10", Qty: 2},
		},
		DiscountMinor: 5000,
		Payment: PaymentBreakdown{
			CashMinor:    350000, // ৳3,500.00 cash
			UpiMinor:     0,
			PrepaidMinor: 30000,  // ৳300.00 from prepaid
			DueMinor:     0,
		},
	}

	payloadBytes, _ := json.Marshal(checkoutPayload)
	req := httptest.NewRequest("POST", "/api/checkout", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected checkout 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["ok"] != true {
		t.Fatalf("expected ok=true, got %v", res["ok"])
	}

	// Total tendered = 350000 + 30000 = 380000
	// Grand = 378000
	// Change = 2000 (৳20.00)
	changeMinor := int64(res["changeMinor"].(float64))
	if changeMinor != 2000 {
		t.Fatalf("expected change minor 2000, got %d", changeMinor)
	}

	// Verify customer's prepaid balance decreased by 30000 (50000 - 30000 = 20000)
	prepaidAfter := int64(res["prepaidAfter"].(float64))
	if prepaidAfter != 20000 {
		t.Fatalf("expected prepaidAfter 20000, got %d", prepaidAfter)
	}

	// Verify stock of p-01 decreased by 1 (45 - 1 = 44)
	prod, _ := data.Table("products").Where("id", "=", "p-01").First(state.pool)
	if prod["stock"].(int64) != 44 {
		t.Fatalf("expected stock 44, got %v", prod["stock"])
	}

	// Verify Receipt text was generated
	receiptText, ok := res["receiptText"].(string)
	if !ok || !strings.Contains(receiptText, "LAKHAN BHANDAR") && !strings.Contains(receiptText, "লাখান ভাণ্ডার") {
		t.Fatalf("expected receiptText to be non-empty and branded")
	}
}

func TestDailyReport(t *testing.T) {
	srv := setupTestApp()

	req := httptest.NewRequest("GET", "/api/reports/daily", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if resp["totalTransactions"].(float64) < 1 {
		t.Fatalf("expected at least 1 transaction, got %v", resp["totalTransactions"])
	}
}

func TestRefundEndpoint(t *testing.T) {
	srv := setupTestApp()

	// 1. Initial Sale p-01 has 45 stock
	refundPayload := RefundAPIRequest{
		SaleID:    "SALE-INIT-001",
		ProductID: "p-01",
		Qty:       1,
		Reason:    "Customer returned bag",
		CashierID: "Joy Sarkar",
	}

	payloadBytes, _ := json.Marshal(refundPayload)
	req := httptest.NewRequest("POST", "/api/refund", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected refund 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if res["ok"] != true {
		t.Fatalf("expected ok=true")
	}

	// Verify stock of p-01 increased back (+1)
	prod, _ := data.Table("products").Where("id", "=", "p-01").First(state.pool)
	if prod["stock"].(int64) != 46 { // 45 + 1 = 46
		t.Fatalf("expected stock 46, got %v", prod["stock"])
	}
}

func TestShiftEndpoints(t *testing.T) {
	srv := setupTestApp()

	// 1. Check current shift
	req := httptest.NewRequest("GET", "/api/shift/current", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// 2. Close shift
	closePayload := ShiftCloseRequest{
		ActualCashMinor: 1000000,
		Notes:           "End of day reconciliation",
	}
	pBytes, _ := json.Marshal(closePayload)
	req2 := httptest.NewRequest("POST", "/api/shift/close", bytes.NewBuffer(pBytes))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec2.Code)
	}
}

func TestCustomerDuePaymentEndpoint(t *testing.T) {
	srv := setupTestApp()

	// Customer c-01 has dueMinor = 125000 (৳1,250.00)
	payPayload := CustomerDuePaymentRequest{
		CustomerID:  "c-01",
		AmountMinor: 25000, // ৳250.00
		Method:      "CASH",
		Reference:   "REC-001",
		Notes:       "Part payment",
	}

	pBytes, _ := json.Marshal(payPayload)
	req := httptest.NewRequest("POST", "/api/customers/due-payment", bytes.NewBuffer(pBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if res["newDueMinor"].(float64) != 100000 { // 125000 - 25000 = 100000
		t.Fatalf("expected new due 100000, got %v", res["newDueMinor"])
	}
}

func TestOfflineSyncEndpoints(t *testing.T) {
	srv := setupTestApp()

	op := sync.MutationOperation{
		OperationID: "op-sync-01",
		DeviceID:    "terminal-1",
		EntityType:  "sale",
		Operation:   "checkout",
		Payload: map[string]interface{}{
			"invoice": "INV-OFFLINE-001",
		},
	}

	pBytes, _ := json.Marshal(op)
	req := httptest.NewRequest("POST", "/api/sync/push", bytes.NewBuffer(pBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPrinterAndDrawerEndpoints(t *testing.T) {
	srv := setupTestApp()

	// Printer Test
	req := httptest.NewRequest("POST", "/api/device/printer/test", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Drawer Open
	req2 := httptest.NewRequest("POST", "/api/device/drawer/open", nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec2.Code)
	}
}

func TestAlapSSRRenderer(t *testing.T) {
	srv := setupTestApp()

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// 1. Verify Alap declarative component markers
	expectedComponents := []string{
		`data-alap-component="POSApp"`,
		`data-alap-component="Header"`,
		`data-alap-component="CatalogPane"`,
		`data-alap-component="CartPane"`,
	}
	for _, comp := range expectedComponents {
		if !strings.Contains(body, comp) {
			t.Fatalf("expected SSR output to contain component marker: %s", comp)
		}
	}

	// 2. Verify SSR State Hydration block
	if !strings.Contains(body, `id="__NILANG_STATE__"`) {
		t.Fatalf("expected SSR output to contain __NILANG_STATE__ hydration script")
	}

	// 3. Verify Alap Runtime inclusion
	if !strings.Contains(body, `<script src="/alap-runtime.js"></script>`) {
		t.Fatalf("expected SSR output to include /alap-runtime.js")
	}
}

func TestAlapRuntimeEndpoint(t *testing.T) {
	srv := setupTestApp()

	req := httptest.NewRequest("GET", "/alap-runtime.js", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Alap Reactive Client Runtime") {
		t.Fatalf("expected alap-runtime.js to contain runtime signature")
	}
}
