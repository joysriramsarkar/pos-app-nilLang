package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// RenderAlapPOSPage renders the complete enterprise Alap declarative component hierarchy for SSR HTML
func RenderAlapPOSPage(products []map[string]interface{}, customers []map[string]interface{}) string {
	// Prepare enriched products
	enrichedProds := make([]map[string]interface{}, len(products))
	for i, p := range products {
		row := make(map[string]interface{})
		for k, v := range p {
			row[k] = v
		}
		pMinor, _ := toInt64(p["priceMinor"])
		wMinor, _ := toInt64(p["wacMinor"])
		row["priceFormatted"] = data.NewMoney(pMinor, "BDT").Format()
		row["wacFormatted"] = data.NewMoney(wMinor, "BDT").Format()
		enrichedProds[i] = row
	}

	stateMap := map[string]interface{}{
		"products":  enrichedProds,
		"customers": customers,
		"activeTab": "tab-1",
		"carts": map[string]interface{}{
			"tab-1": map[string]interface{}{"items": []interface{}{}, "discountMinor": 0, "customerId": "c-01"},
			"tab-2": map[string]interface{}{"items": []interface{}{}, "discountMinor": 0, "customerId": "c-01"},
			"tab-3": map[string]interface{}{"items": []interface{}{}, "discountMinor": 0, "customerId": "c-01"},
		},
		"searchQuery":      "",
		"selectedCategory": "all",
		"isOnline":         true,
	}

	stateJSON, _ := json.Marshal(stateMap)

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="bn">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>লাখান ভাণ্ডার POS • NilLang Alap Production Platform</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Hind+Siliguri:wght@400;500;600;700&family=Outfit:wght@400;500;600;700&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="/style.css">
</head>
<body class="pos-app-body">
  <div id="alap-root" class="app-container" data-alap-component="POSApp">

    <!-- 1. Header Component -->
    <header class="pos-header" data-alap-component="Header">
      <div class="brand-zone">
        <div class="brand-logo">🏪</div>
        <div class="brand-titles">
          <div class="store-name">লাখান ভাণ্ডার <span class="badge-engine">NilLang + Alap Engine</span></div>
          <div class="store-meta">পাইকারি ও খুচরা বিক্রয় কেন্দ্র • শিফট #০১ • টার্মিনাল-১</div>
        </div>
      </div>

      <div class="header-center-info">
        <div class="clock-display" id="clockDisplay">--:--:--</div>
        <div class="cashier-badge">
          <span class="dot-online" id="statusDot"></span>
          <span id="onlineStatusLabel">অনলাইন</span> • ক্যাশিয়ার: <strong id="cashierNameDisplay">জয় সরকার</strong>
        </div>
      </div>

      <div class="header-actions">
        <button class="btn btn-glass" data-alap-action="open-report" title="দৈনিক বিক্রয় ও লাভ রিপোর্ট (Alt+R / F8)">
          <span class="btn-icon">📊</span> রিপোর্ট
        </button>
        <button class="btn btn-glass" data-alap-action="open-stock" title="মজুদ পণ্য ও স্টক সমন্বয়">
          <span class="btn-icon">📦</span> স্টক
        </button>
        <button class="btn btn-glass" data-alap-action="open-customers" title="গ্রাহক বাকি ও খতিয়ান">
          <span class="btn-icon">👥</span> বাকি
        </button>
        <button class="btn btn-glass" data-alap-action="open-shift" title="শিফট ও ক্যাশ ড্রয়ার">
          <span class="btn-icon">🔄</span> শিফট
        </button>
        <button class="btn btn-glass" data-alap-action="open-sync" title="অফলাইন সিঙ্ক কিউ">
          <span class="btn-icon">⚡</span> সিঙ্ক <span class="badge-sync-count" id="syncPendingBadge" style="display:none">০</span>
        </button>
      </div>
    </header>

    <!-- 2. Workspace Layout -->
    <main class="pos-workspace">

      <!-- LEFT: Catalog & Search Component -->
      <section class="pane-catalog" data-alap-component="CatalogPane">
        <div class="scanner-search-bar">
          <div class="search-input-wrapper">
            <span class="search-icon">🔍</span>
            <input type="text" id="barcodeSearchInput" class="input-search"
                   placeholder="বারকোড স্ক্যান করুন বা পণ্যের নাম / SKU লিখুন (F2)..." autocomplete="off" autofocus>
            <button class="search-clear-btn" id="clearSearchBtn" style="display:none">✕</button>
          </div>
          <button class="btn btn-secondary btn-f2" data-alap-action="focus-search">F2 স্ক্যান</button>
        </div>

        <div class="category-tabs">
          <button class="cat-pill active" data-alap-cat="all">সব পণ্য</button>
          <button class="cat-pill" data-alap-cat="cat-1">চাল ও ডাল</button>
          <button class="cat-pill" data-alap-cat="cat-2">তেল ও ঘি</button>
          <button class="cat-pill" data-alap-cat="cat-3">আটা ও চিনি</button>
          <button class="cat-pill" data-alap-cat="cat-4">মসলা ও লবণ</button>
          <button class="cat-pill" data-alap-cat="cat-5">চা ও পানীয়</button>
        </div>

        <div class="product-grid" id="productGrid">
`)

	// SSR Initial Product Cards
	for _, p := range enrichedProds {
		id, _ := p["id"].(string)
		nameBn, _ := p["nameBn"].(string)
		if nameBn == "" {
			nameBn, _ = p["name"].(string)
		}
		sku, _ := p["sku"].(string)
		priceFormatted, _ := p["priceFormatted"].(string)
		stock, _ := toInt64(p["stock"])
		unit, _ := p["unit"].(string)

		stockBadge := fmt.Sprintf("মজুদ: %d %s", stock, unit)
		stockClass := ""
		if stock <= 0 {
			stockBadge = "স্টক শেষ"
			stockClass = "out-of-stock"
		} else if stock <= 15 {
			stockClass = "low"
		}

		disabledAttr := ""
		if stock <= 0 {
			disabledAttr = "disabled"
		}

		sb.WriteString(fmt.Sprintf(`          <div class="product-card %s" data-alap-action="add" data-id="%s">
            <div class="card-head">
              <span class="sku-tag">%s</span>
              <span class="stock-badge %s">%s</span>
            </div>
            <div class="product-name">%s</div>
            <div class="card-footer">
              <div>
                <span class="price-lbl">মূল্য</span>
                <span class="price-val">%s</span>
              </div>
              <button class="btn btn-primary btn-sm" %s data-alap-action="add" data-id="%s">
                ＋ যোগ
              </button>
            </div>
          </div>
`, stockClass, id, sku, stockClass, stockBadge, nameBn, priceFormatted, disabledAttr, id))
	}

	sb.WriteString(`        </div>
      </section>

      <!-- RIGHT: Multi-Tab Cart Component -->
      <section class="pane-terminal" data-alap-component="CartPane">
        <div class="cart-tabs-bar">
          <div class="cart-tabs-group">
            <button class="tab-btn active" data-alap-action="tab" data-tab="tab-1" data-alap-tab="tab-1">
              হিসাব ১ <span class="tab-shortcut">(F6)</span>
            </button>
            <button class="tab-btn" data-alap-action="tab" data-tab="tab-2" data-alap-tab="tab-2">
              হিসাব ২ <span class="tab-shortcut">(F7)</span>
            </button>
            <button class="tab-btn" data-alap-action="tab" data-tab="tab-3" data-alap-tab="tab-3">
              হিসাব ৩ <span class="tab-shortcut">(F8)</span>
            </button>
          </div>
          <button class="btn-clear-cart" data-alap-action="clear-cart" title="কার্ট খালি করুন">🗑️ খালি</button>
        </div>

        <div class="customer-bar">
          <div class="customer-select-wrap">
            <span class="cust-icon">👤</span>
            <select id="customerSelect" class="customer-dropdown">
`)

	for _, c := range customers {
		cID, _ := c["id"].(string)
		cName, _ := c["nameBn"].(string)
		if cName == "" {
			cName, _ = c["name"].(string)
		}
		cPhone, _ := c["phone"].(string)
		due, _ := toInt64(c["dueMinor"])
		prepaid, _ := toInt64(c["prepaidMinor"])
		bal := prepaid - due
		sb.WriteString(fmt.Sprintf(`              <option value="%s">%s (%s) • ব্যালেন্স: %s</option>
`, cID, cName, cPhone, data.NewMoney(bal, "BDT").Format()))
	}

	sb.WriteString(`            </select>
          </div>
        </div>

        <div class="cart-table-wrapper">
          <div class="cart-table-header">
            <div class="col-item">পণ্য</div>
            <div class="col-qty">পরিমাণ</div>
            <div class="col-price">মোট টাকা</div>
            <div class="col-act"></div>
          </div>
          <div class="cart-items-body" id="cartItemList">
            <div class="empty-cart-state">
              <span class="empty-icon">🛒</span>
              <p>কার্ট খালি। বাম পাশ থেকে পণ্য নির্বাচন করুন অথবা বারকোড স্ক্যান করুন।</p>
            </div>
          </div>
        </div>

        <div class="cart-summary-panel">
          <div class="summary-line">
            <span class="summary-lbl">সাবটোটাল</span>
            <span class="summary-val" id="subtotalDisplay">৳০.০০</span>
          </div>
          <div class="summary-line">
            <span class="summary-lbl">ছাড় / ডিসকাউন্ট (৳)</span>
            <input type="number" id="discountInput" class="input-discount" value="0" min="0" step="1">
          </div>
          <div class="summary-line">
            <span class="summary-lbl">ভ্যাট / কর (৫%)</span>
            <span class="summary-val" id="taxDisplay">৳০.০০</span>
          </div>
          <div class="summary-line grand-total-line">
            <span class="grand-lbl">সর্বমোট প্রদেয়</span>
            <span class="grand-val" id="grandTotalDisplay">৳০.০০</span>
          </div>
          <button id="btnCheckout" class="btn btn-checkout-action" data-alap-action="open-checkout" disabled>
            💳 F4 চেকআউট ও পেমেন্ট
          </button>
        </div>
      </section>
    </main>

    <!-- 3. Enterprise Modals Hierarchy -->

    <!-- Checkout Modal -->
    <div id="checkoutModal" class="modal-backdrop">
      <div class="modal-card modal-checkout">
        <div class="modal-header">
          <h3>💳 বিক্রয় সম্পন্ন ও পেমেন্ট গ্রহণ</h3>
          <button class="btn-close" data-alap-action="close-modal">✕</button>
        </div>
        <div class="modal-body">
          <div class="checkout-grand-badge">
            <span>মোট প্রদেয় বিল:</span>
            <strong id="checkoutTotalBadge">৳০.০০</strong>
          </div>

          <div class="quick-cash-row">
            <span class="quick-lbl">দ্রুত নগদ বাটন:</span>
            <button class="btn-quick-cash" data-amount="exact">যথাযথ বিল</button>
            <button class="btn-quick-cash" data-amount="50000">৳৫০০</button>
            <button class="btn-quick-cash" data-amount="100000">৳১,০০০</button>
            <button class="btn-quick-cash" data-amount="200000">৳২,০০০</button>
            <button class="btn-quick-cash" data-amount="500000">৳৫,০০০</button>
          </div>

          <div class="split-pay-grid">
            <div class="pay-row">
              <label>💵 নগদ গ্রহণ (Cash):</label>
              <input type="number" id="payCashInput" class="input-pay" value="0" min="0">
            </div>
            <div class="pay-row">
              <label>📱 বিকাশ / নগদ (MFS):</label>
              <input type="number" id="payMFSInput" class="input-pay" value="0" min="0">
            </div>
            <div class="pay-row">
              <label>💳 ব্যাংক কার্ড (POS):</label>
              <input type="number" id="payCardInput" class="input-pay" value="0" min="0">
            </div>
            <div class="pay-row">
              <label style="color:#f59e0b">📝 বাকি হিসাব (Due):</label>
              <input type="number" id="payDueInput" class="input-pay" value="0" min="0">
            </div>
          </div>

          <div class="change-return-box">
            <span>ফেরত দিন (Change):</span>
            <strong id="changeReturnDisplay" class="change-val positive">৳০.০০</strong>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" data-alap-action="close-modal">বাতিল</button>
          <button class="btn btn-primary" id="btnConfirmCheckout" data-alap-action="confirm-checkout">
            ✅ নিশ্চিত করুন ও রসিদ দিন
          </button>
        </div>
      </div>
    </div>

    <!-- Thermal Receipt Modal -->
    <div id="receiptModal" class="modal-backdrop">
      <div class="modal-card modal-receipt">
        <div class="modal-header">
          <h3>🧾 বিক্রয় রসিদ (Thermal 58mm Receipt)</h3>
          <button class="btn-close" data-alap-action="close-modal">✕</button>
        </div>
        <div class="modal-body">
          <div class="thermal-receipt-paper" id="receiptPaper">
            <!-- Receipt Text Content injected here -->
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" data-alap-action="close-modal">বন্ধ করুন</button>
          <button class="btn btn-primary" data-alap-action="print-receipt">🖨️ প্রিন্ট করুন</button>
        </div>
      </div>
    </div>

    <!-- Daily Sales Report Modal -->
    <div id="dailyReportModal" class="modal-backdrop">
      <div class="modal-card modal-report">
        <div class="modal-header">
          <h3>📊 দৈনিক বিক্রয় ও লাভ-লোকসান রিপোর্ট</h3>
          <button class="btn-close" data-alap-action="close-modal">✕</button>
        </div>
        <div class="modal-body">
          <div class="report-stats-grid">
            <div class="stat-card">
              <span class="stat-lbl">মোট বিক্রয়</span>
              <strong class="stat-val" id="reportTotalSales" style="color:#10b981">৳০.০০</strong>
            </div>
            <div class="stat-card">
              <span class="stat-lbl">মোট লাভ (WAC Profit)</span>
              <strong class="stat-val" id="reportTotalProfit" style="color:#00d4ff">৳০.০০</strong>
            </div>
            <div class="stat-card">
              <span class="stat-lbl">মোট অর্ডার</span>
              <strong class="stat-val" id="reportOrderCount">০ টি</strong>
            </div>
            <div class="stat-card">
              <span class="stat-lbl">মজুদ পণ্যের মূল্য</span>
              <strong class="stat-val" id="reportInventoryValuation" style="color:#a855f7">৳০.০০</strong>
            </div>
          </div>

          <div class="report-payments-summary" id="reportPaymentsSummary">
            <!-- Injected by alap-runtime.js -->
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" data-alap-action="export-csv">📥 CSV রিপোর্ট ডাউনলোড</button>
          <button class="btn btn-primary" data-alap-action="close-modal">বন্ধ করুন</button>
        </div>
      </div>
    </div>

    <!-- Stock Management Modal -->
    <div id="stockModal" class="modal-backdrop">
      <div class="modal-card modal-stock">
        <div class="modal-header">
          <h3>📦 স্টক ব্যবস্থাপনা ও পণ্য ক্রয়</h3>
          <button class="btn-close" data-alap-action="close-modal">✕</button>
        </div>
        <div class="modal-body">
          <div class="stock-adjust-form">
            <h4>নতুন স্টক যোগ বা সমন্বয় করুন</h4>
            <div class="form-grid">
              <div class="form-group">
                <label>পণ্য নির্বাচন করুন:</label>
                <select id="adjustProductSelect" class="form-input">
`)

	for _, p := range enrichedProds {
		pID, _ := p["id"].(string)
		pName, _ := p["nameBn"].(string)
		stk, _ := toInt64(p["stock"])
		sb.WriteString(fmt.Sprintf(`                  <option value="%s">%s (বর্তমান মজুদ: %d)</option>
`, pID, pName, stk))
	}

	sb.WriteString(`                </select>
              </div>
              <div class="form-group">
                <label>যোগ করার পরিমাণ (+):</label>
                <input type="number" id="adjustQtyInput" class="form-input" value="10" min="1">
              </div>
              <div class="form-group">
                <label>ক্রয় মূল্য প্রতি একক (৳):</label>
                <input type="number" id="adjustCostInput" class="form-input" placeholder="ঐচ্ছিক (WAC গণনায়)">
              </div>
              <div class="form-group">
                <label>সমন্বয়ের কারণ:</label>
                <input type="text" id="adjustReasonInput" class="form-input" value="নতুন চালান ক্রয়">
              </div>
            </div>
            <button class="btn btn-primary" style="margin-top:12px" data-alap-action="submit-adjust">
              📥 স্টক আপডেট করুন
            </button>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" data-alap-action="close-modal">বন্ধ করুন</button>
        </div>
      </div>
    </div>

    <!-- Customer Due & Ledger Modal -->
    <div id="customerModal" class="modal-backdrop">
      <div class="modal-card modal-customer">
        <div class="modal-header">
          <h3>👥 গ্রাহক খতিয়ান ও বাকি পরিষদ</h3>
          <button class="btn-close" data-alap-action="close-modal">✕</button>
        </div>
        <div class="modal-body">
          <div class="due-pay-form">
            <h4>বাকি টাকা গ্রহণ করুন</h4>
            <div class="form-grid">
              <div class="form-group">
                <label>গ্রাহক:</label>
                <select id="duePayCustomerSelect" class="form-input">
`)

	for _, c := range customers {
		cID, _ := c["id"].(string)
		cName, _ := c["nameBn"].(string)
		due, _ := toInt64(c["dueMinor"])
		sb.WriteString(fmt.Sprintf(`                  <option value="%s">%s (বাকি: %s)</option>
`, cID, cName, data.NewMoney(due, "BDT").Format()))
	}

	sb.WriteString(`                </select>
              </div>
              <div class="form-group">
                <label>পরিশোধের পরিমাণ (৳):</label>
                <input type="number" id="duePayAmountInput" class="form-input" placeholder="৳ পরিমাণ">
              </div>
              <div class="form-group">
                <label>পেমেন্ট মাধ্যম:</label>
                <select id="duePayMethodSelect" class="form-input">
                  <option value="CASH">নগদ (Cash)</option>
                  <option value="BKASH">বিকাশ (bKash)</option>
                  <option value="BANK">ব্যাংক স্থানান্তর</option>
                </select>
              </div>
              <div class="form-group">
                <label>রেফারেন্স / নোট:</label>
                <input type="text" id="duePayRefInput" class="form-input" placeholder="রসিদ নম্বর বা নোট">
              </div>
            </div>
            <button class="btn btn-primary" style="margin-top:12px" data-alap-action="submit-due-pay">
              ✅ বাকি টাকা জমা নিন
            </button>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" data-alap-action="close-modal">বন্ধ করুন</button>
        </div>
      </div>
    </div>

    <!-- Shift & Register Modal -->
    <div id="shiftModal" class="modal-backdrop">
      <div class="modal-card modal-shift">
        <div class="modal-header">
          <h3>🔄 শিফট ও রেজিস্টার ক্যাশ ব্যবস্থাপনা</h3>
          <button class="btn-close" data-alap-action="close-modal">✕</button>
        </div>
        <div class="modal-body">
          <div class="shift-info-card" id="shiftInfoBody">
            <!-- Populated dynamically -->
          </div>
          <div class="close-shift-box" style="margin-top:18px; padding-top:14px; border-top:1px solid rgba(255,255,255,0.1)">
            <h4>শিফট সমাপ্ত ও ড্রয়ার ক্যাশ গণনা</h4>
            <div class="form-group">
              <label>ড্রয়ারে গণনাকৃত নগদ টাকা (৳):</label>
              <input type="number" id="closeShiftActualCash" class="form-input" placeholder="গণনাকৃত নগদ টাকা">
            </div>
            <div class="form-group" style="margin-top:8px">
              <label>সমাপ্তি নোট:</label>
              <input type="text" id="closeShiftNotes" class="form-input" placeholder="শিফট নোট বা মন্তব্য">
            </div>
            <button class="btn btn-danger" style="margin-top:12px" data-alap-action="confirm-close-shift">
              🔒 শিফট বন্ধ করুন
            </button>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" data-alap-action="close-modal">বন্ধ করুন</button>
        </div>
      </div>
    </div>

    <!-- Offline Sync Modal -->
    <div id="syncModal" class="modal-backdrop">
      <div class="modal-card modal-sync">
        <div class="modal-header">
          <h3>⚡ অফলাইন ট্রানজ্যাকশন সিঙ্ক কিউ</h3>
          <button class="btn-close" data-alap-action="close-modal">✕</button>
        </div>
        <div class="modal-body">
          <p id="syncStatusSummary">ইন্টারনেট না থাকলেও সমস্ত বিক্রয় অফলাইনে নিরাপদে সংরক্ষিত রয়েছে।</p>
          <div class="sync-queue-list" id="syncQueueList" style="margin-top:12px">
            <p style="color:#94a3b8">বর্তমানে কোনো পেন্ডিং অফলাইন ট্রানজ্যাকশন নেই।</p>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary" data-alap-action="force-sync">🔄 সার্ভারে সিঙ্ক করুন</button>
          <button class="btn btn-secondary" data-alap-action="close-modal">বন্ধ করুন</button>
        </div>
      </div>
    </div>

    <!-- Toast Notification -->
    <div id="alapToast" class="alap-toast"></div>
  </div>

  <!-- Alap SSR State Hydration -->
  <script id="__NILANG_STATE__" type="application/json">`)
	sb.Write(stateJSON)
	sb.WriteString(`</script>
  <script>window.__NILANG_INITIAL_STATE__ = JSON.parse(document.getElementById('__NILANG_STATE__').textContent);</script>

  <!-- Official Alap Reactive Client Runtime -->
  <script src="/alap-runtime.js"></script>
</body>
</html>`)

	return sb.String()
}
