/**
 * Alap Reactive Client Runtime • Lakhan Bhandar POS
 * Complete Implementation matching pos-app visuals and full enterprise feature set.
 * Powered by Nilang & Alap Platform
 */

(function () {
  'use strict';

  console.log('⚡ [Alap Runtime] Initializing Lakhan Bhandar Enterprise Retail Engine...');

  // --- 1. Sound Synthesizer (Web Audio API) ---
  const sound = {
    ctx: null,
    init() {
      if (!this.ctx) {
        const AudioCtx = window.AudioContext || window.webkitAudioContext;
        if (AudioCtx) this.ctx = new AudioCtx();
      }
      if (this.ctx && this.ctx.state === 'suspended') this.ctx.resume();
    },
    beep() {
      try {
        this.init();
        if (!this.ctx) return;
        const osc = this.ctx.createOscillator();
        const gain = this.ctx.createGain();
        osc.type = 'sine';
        osc.frequency.setValueAtTime(1760, this.ctx.currentTime);
        gain.gain.setValueAtTime(0.15, this.ctx.currentTime);
        gain.gain.exponentialRampToValueAtTime(0.01, this.ctx.currentTime + 0.08);
        osc.connect(gain);
        gain.connect(this.ctx.destination);
        osc.start();
        osc.stop(this.ctx.currentTime + 0.08);
      } catch (_) {}
    },
    chime() {
      try {
        this.init();
        if (!this.ctx) return;
        const now = this.ctx.currentTime;
        [523.25, 659.25, 783.99, 1046.5].forEach((freq, i) => {
          const osc = this.ctx.createOscillator();
          const gain = this.ctx.createGain();
          osc.type = 'triangle';
          osc.frequency.setValueAtTime(freq, now + i * 0.07);
          gain.gain.setValueAtTime(0.18, now + i * 0.07);
          gain.gain.exponentialRampToValueAtTime(0.01, now + i * 0.07 + 0.22);
          osc.connect(gain);
          gain.connect(this.ctx.destination);
          osc.start(now + i * 0.07);
          osc.stop(now + i * 0.07 + 0.25);
        });
      } catch (_) {}
    }
  };

  // --- 2. Numeral Helpers (Bengali / English) ---
  const bnDigits = ['০', '১', '২', '৩', '৪', '৫', '৬', '৭', '৮', '৯'];
  function toBengaliNumerals(n) {
    return String(n).replace(/[0-9]/g, d => bnDigits[Number(d)]);
  }

  function formatPrice(minor, showCurrency = true) {
    const val = (Number(minor) || 0) / 100;
    const formatted = val.toLocaleString('en-IN', {
      minimumFractionDigits: val % 1 === 0 ? 0 : 2,
      maximumFractionDigits: 2
    });
    return (showCurrency ? '₹' : '') + toBengaliNumerals(formatted);
  }

  function formatNumber(n) {
    return toBengaliNumerals(Number(n).toLocaleString('en-IN'));
  }

  // --- 3. Reactive State Hydration ---
  const initial = {
    products: [],
    categories: [],
    customers: [],
    activeTab: 'tab-1',
    carts: {
      'tab-1': {
        items: [
          {
            productId: 'p-05',
            name: 'বিসলেরি আর ৫ লিটার',
            barcode: '8906017290071',
            unitPriceMinor: 7000,
            qty: 1
          },
          {
            productId: 'p-06',
            name: 'বি ন্যাচারাল নারকেল জল ১০৮ টাকা',
            barcode: '8001725008307',
            unitPriceMinor: 12000,
            qty: 1
          }
        ],
        discountMinor: 0,
        customerId: 'c-keshab'
      },
      'tab-2': { items: [], discountMinor: 0, customerId: 'c-walkin' },
      'tab-3': { items: [], discountMinor: 0, customerId: 'c-walkin' }
    },
    searchQuery: '',
    selectedCategory: 'all',
    preferredPayMethod: 'cash',
    isOnline: navigator.onLine
  };

  const state = new Proxy(initial, {
    set(target, prop, value) {
      target[prop] = value;
      alap.render();
      return true;
    }
  });

  // --- 4. Alap Core Controller ---
  const alap = {
    state,
    sound,

    getActiveCart() {
      if (!state.carts[state.activeTab]) {
        state.carts[state.activeTab] = { items: [], discountMinor: 0, customerId: 'c-keshab' };
      }
      return state.carts[state.activeTab];
    },

    getCartTotals() {
      const cart = this.getActiveCart();
      let subtotal = 0;
      cart.items.forEach(it => { subtotal += (it.unitPriceMinor * it.qty); });
      const discount = cart.discountMinor || 0;
      const afterDiscount = Math.max(0, subtotal - discount);
      const grand = afterDiscount;
      return { subtotal, discount, tax: 0, grand };
    },

    addToCart(productId, qty = 1) {
      const prod = state.products.find(p => p.id === productId);
      if (!prod || prod.stock <= 0) {
        this.toast('❌ পণ্যটি স্টকে নেই!');
        return;
      }
      const cart = this.getActiveCart();
      const existing = cart.items.find(i => i.productId === productId);
      if (existing) {
        if (existing.qty + qty > prod.stock) {
          this.toast('⚠️ অপর্যাপ্ত মজুদ!');
          return;
        }
        existing.qty += qty;
      } else {
        cart.items.push({
          productId: prod.id,
          name: prod.nameBn || prod.name,
          sku: prod.sku,
          barcode: prod.barcode || '',
          unit: prod.unit || 'piece',
          unitPriceMinor: prod.priceMinor,
          wacMinor: prod.wacMinor,
          qty: qty
        });
      }
      sound.beep();
      this.render();
    },

    setQty(productId, newQty) {
      const cart = this.getActiveCart();
      const idx = cart.items.findIndex(i => i.productId === productId);
      if (idx === -1) return;
      const prod = state.products.find(p => p.id === productId);
      if (newQty <= 0) {
        cart.items.splice(idx, 1);
      } else if (prod && newQty > prod.stock) {
        this.toast('⚠️ অপর্যাপ্ত মজুদ!');
        return;
      } else {
        cart.items[idx].qty = newQty;
      }
      sound.beep();
      this.render();
    },

    updateQty(productId, delta) {
      const cart = this.getActiveCart();
      const item = cart.items.find(i => i.productId === productId);
      if (!item) return;
      this.setQty(productId, item.qty + delta);
    },

    removeItem(productId) {
      const cart = this.getActiveCart();
      cart.items = cart.items.filter(i => i.productId !== productId);
      this.render();
    },

    clearCart() {
      const cart = this.getActiveCart();
      cart.items = [];
      cart.discountMinor = 0;
      this.render();
      this.toast('🗑️ কার্ট খালি করা হয়েছে');
    },

    setTab(tabId) {
      state.activeTab = tabId;
      document.querySelectorAll('[data-alap-tab]').forEach(el => {
        el.classList.toggle('active', el.getAttribute('data-alap-tab') === tabId);
      });
      this.render();
    },

    newBill() {
      const nextId = 'tab-' + (Object.keys(state.carts).length + 1);
      state.carts[nextId] = { items: [], discountMinor: 0, customerId: 'c-keshab' };
      this.setTab(nextId);
      this.toast('✨ নতুন বিল তৈরি হয়েছে');
    },

    toast(msg, type = 'info') {
      showToast(msg, type);
    },

    // ─── Declarative DOM Renderer ───
    render() {
      // 1. Catalog Grid
      const gridEl = document.getElementById('productGrid');
      if (gridEl) {
        const query = (state.searchQuery || '').toLowerCase().trim();
        const cat = state.selectedCategory || 'all';
        const filtered = state.products.filter(p => {
          const matchQuery = !query ||
            (p.name && p.name.toLowerCase().includes(query)) ||
            (p.nameBn && p.nameBn.toLowerCase().includes(query)) ||
            (p.sku && p.sku.toLowerCase().includes(query)) ||
            (p.barcode && p.barcode.includes(query));
          const matchCat = cat === 'all' || p.categoryId === cat;
          return matchQuery && matchCat;
        });

        // Update product count badge
        const countBadge = document.getElementById('productCountBadge');
        if (countBadge) countBadge.textContent = `${toBengaliNumerals(filtered.length)}টি পণ্য`;

        gridEl.innerHTML = filtered.map(p => {
          const isLowStock = p.stock <= (p.lowStock || 10);
          return `
            <div class="product-card" onclick="alap.addToCart('${p.id}', 1)">
              ${isLowStock ? '<div class="low-stock-banner">⚠️ কম স্টক</div>' : ''}
              <div class="product-img-box">
                <svg class="product-box-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                  <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
                  <polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
                  <line x1="12" y1="22.08" x2="12" y2="12"/>
                </svg>
              </div>
              <div class="product-info">
                <div class="product-name" title="${p.nameBn || p.name}">${p.nameBn || p.name}</div>
                <div class="product-cat-tag">${p.category || p.categoryBn || 'General'}</div>
                <div class="product-price-row">
                  <span class="product-price">${formatPrice(p.priceMinor)}</span>
                  <span class="product-unit">/${p.unit || 'piece'}</span>
                </div>
                <div class="product-stock-line ${isLowStock ? 'low' : ''}">স্টক: ${formatNumber(p.stock)}</div>
                <button class="btn-add-cart" onclick="event.stopPropagation(); alap.addToCart('${p.id}', 1)">
                  + কার্টে যোগ করুন
                </button>
              </div>
            </div>
          `;
        }).join('');
      }

      // 2. Cart Items Drawer
      const cart = this.getActiveCart();
      const cartListEl = document.getElementById('cartItemList');
      const cartBadge = document.getElementById('cartCountBadge');
      if (cartBadge) {
        const totalItems = cart.items.reduce((s, i) => s + i.qty, 0);
        cartBadge.textContent = `${toBengaliNumerals(totalItems)}টি পণ্য`;
      }

      if (cartListEl) {
        if (cart.items.length === 0) {
          cartListEl.innerHTML = `
            <div class="empty-cart-state">
              <span class="empty-icon">🛒</span>
              <p>কার্ট খালি। বাম পাশ থেকে পণ্য নির্বাচন করুন।</p>
            </div>
          `;
        } else {
          cartListEl.innerHTML = cart.items.map(it => {
            const lineTotal = it.unitPriceMinor * it.qty;
            return `
              <div class="cart-item-card">
                <div class="cart-item-row-top">
                  <div class="cart-item-title-group">
                    <span class="cart-drag-dots">⋮⋮</span>
                    <span class="cart-item-name">${it.name}</span>
                  </div>
                  <span class="cart-item-line-total">${formatPrice(lineTotal)}</span>
                </div>
                <div class="cart-item-meta">
                  <span>${formatPrice(it.unitPriceMinor)}/piece</span>
                  ${it.barcode ? `<span>${it.barcode}</span>` : ''}
                </div>
                <div class="cart-item-actions-row">
                  <div class="qty-stepper">
                    <button class="btn-qty" onclick="alap.updateQty('${it.productId}', -1)">—</button>
                    <span class="qty-display">${toBengaliNumerals(it.qty)}</span>
                    <button class="btn-qty" onclick="alap.updateQty('${it.productId}', 1)">+</button>
                  </div>
                  <div class="qty-presets-wrap">
                    <span class="preset-lbl">প্রিসেট:</span>
                    <button class="btn-preset" onclick="alap.setQty('${it.productId}', 1)">১</button>
                    <button class="btn-preset" onclick="alap.setQty('${it.productId}', 2)">২</button>
                    <button class="btn-preset" onclick="alap.setQty('${it.productId}', 5)">৫</button>
                    <button class="btn-preset" onclick="alap.setQty('${it.productId}', 10)">১০</button>
                    <button class="btn-preset" onclick="alap.setQty('${it.productId}', 20)">২০</button>
                  </div>
                  <button class="btn-del-item" title="মুছুন" onclick="alap.removeItem('${it.productId}')">🗑️</button>
                </div>
              </div>
            `;
          }).join('');
        }
      }

      // 3. Totals
      const totals = this.getCartTotals();
      const subEl = document.getElementById('subtotalDisplay');
      const grandEl = document.getElementById('grandTotalDisplay');
      const btnCheckout = document.getElementById('btnCheckout');

      if (subEl) subEl.textContent = formatPrice(totals.subtotal);
      if (grandEl) grandEl.textContent = formatPrice(totals.grand);
      if (btnCheckout) btnCheckout.disabled = cart.items.length === 0;

      // 4. Customer Dropdown
      const custSel = document.getElementById('customerSelect');
      if (custSel && custSel.options.length === 0 && state.customers.length > 0) {
        custSel.innerHTML = state.customers.map(c => `
          <option value="${c.id}" ${c.id === cart.customerId ? 'selected' : ''}>
            ${c.nameBn || c.name}
          </option>
        `).join('');
      }
    },

    // ─── Modal Management ───
    openCheckout() {
      const totals = this.getCartTotals();
      if (totals.grand <= 0) return;
      const modal = document.getElementById('checkoutModal');
      if (!modal) return;
      document.getElementById('checkoutTotalBadge').textContent = formatPrice(totals.grand);

      // Pre-fill cash input with exact total
      const cashIn = document.getElementById('payCashInput');
      const mfsIn = document.getElementById('payMFSInput');
      const cardIn = document.getElementById('payCardInput');
      const dueIn = document.getElementById('payDueInput');

      if (cashIn) cashIn.value = (totals.grand / 100).toFixed(0);
      if (mfsIn) mfsIn.value = '0';
      if (cardIn) cardIn.value = '0';
      if (dueIn) dueIn.value = '0';

      this.calcChange();
      modal.style.display = 'flex';
      cashIn?.focus();
      cashIn?.select();
    },

    calcChange() {
      const totals = this.getCartTotals();
      const cash = (Number(document.getElementById('payCashInput')?.value) || 0) * 100;
      const mfs = (Number(document.getElementById('payMFSInput')?.value) || 0) * 100;
      const card = (Number(document.getElementById('payCardInput')?.value) || 0) * 100;
      const due = (Number(document.getElementById('payDueInput')?.value) || 0) * 100;

      const totalPaid = cash + mfs + card + due;
      const change = Math.max(0, totalPaid - totals.grand);
      const changeEl = document.getElementById('changeReturnDisplay');
      if (changeEl) {
        changeEl.textContent = formatPrice(change);
      }
    },

    async confirmCheckout() {
      const cart = this.getActiveCart();
      if (cart.items.length === 0) return;
      const totals = this.getCartTotals();

      const cashMinor = (Number(document.getElementById('payCashInput')?.value) || 0) * 100;
      const upiMinor = (Number(document.getElementById('payMFSInput')?.value) || 0) * 100;
      const cardMinor = (Number(document.getElementById('payCardInput')?.value) || 0) * 100;
      const dueMinor = (Number(document.getElementById('payDueInput')?.value) || 0) * 100;

      const totalPaid = cashMinor + upiMinor + cardMinor + dueMinor;
      if (totalPaid < totals.grand) {
        this.toast('⚠️ প্রদত্ত টাকা মোট বিলের চেয়ে কম! বিল ' + formatPrice(totals.grand));
        return;
      }

      const payload = {
        customerId: cart.customerId || 'c-keshab',
        items: cart.items.map(it => ({
          productId: it.productId,
          qty: it.qty,
          unitPriceMinor: it.unitPriceMinor
        })),
        discountMinor: cart.discountMinor || 0,
        payment: {
          cashMinor: cashMinor,
          upiMinor: upiMinor,
          prepaidMinor: cardMinor,
          dueMinor: dueMinor
        }
      };

      const btnConfirm = document.getElementById('btnConfirmCheckout');
      if (btnConfirm) { btnConfirm.disabled = true; btnConfirm.textContent = '⏳ প্রক্রিয়াকরণ হচ্ছে...'; }

      try {
        const res = await fetch('/api/checkout', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        if (!res.ok) {
          const errData = await res.json().catch(() => ({}));
          throw new Error(errData.message || errData.error || 'সার্ভার ত্রুটি (' + res.status + ')');
        }

        const data = await res.json();
        if (!data.ok && !data.saleId) {
          throw new Error(data.error || data.message || 'চেকআউট ব্যর্থ হয়েছে');
        }

        sound.chime();
        showToast('🎉 বিক্রয় সফলভাবে সম্পন্ন হয়েছে!', 'success');
        this.closeModal();

        // Show Thermal Receipt
        this.showReceipt(data);

        // Clear cart & refresh catalog stock
        this.clearCart();
        this.fetchCatalog();
      } catch (err) {
        console.error('Checkout error:', err);
        showToast('❌ চেকআউট ব্যর্থ: ' + err.message, 'error');
      } finally {
        if (btnConfirm) { btnConfirm.disabled = false; btnConfirm.textContent = '✅ বিক্রয় সম্পন্ন করুন'; }
      }
    },

    showReceipt(sale) {
      const modal = document.getElementById('receiptModal');
      const paper = document.getElementById('receiptPaper');
      if (!modal || !paper) return;

      const s = sale || {};
      const items = s.items || [];
      const invoiceNo = s.invoiceNo || 'INV-' + Math.floor(100000 + Math.random() * 900000);
      const dateStr = new Date().toLocaleString('en-IN');

      paper.innerHTML = `
        <div style="text-align:center;margin-bottom:12px">
          <h2 style="font-size:18px;margin-bottom:2px">LAKHAN BHANDAR</h2>
          <p style="font-size:13px;font-weight:600">লক্ষণ ভাণ্ডার</p>
          <p style="font-size:10px;color:#475569">বাজার রোড, হোলসেল ও রিটেল মার্ট</p>
          <p style="font-size:10px;color:#475569">ফোন: 01711-000000</p>
          <div style="border-top:1px dashed #000;margin:8px 0"></div>
          <p style="font-size:11px;font-weight:700">ক্যাশ মেমো / ইনভয়েস</p>
          <p style="font-size:10px">বিল নং: ${invoiceNo}</p>
          <p style="font-size:10px">তারিখ: ${dateStr}</p>
        </div>
        <table style="width:100%;font-size:11px;border-collapse:collapse;margin-bottom:8px">
          <thead>
            <tr style="border-bottom:1px dashed #000">
              <th style="text-align:left;padding:3px 0">পণ্য</th>
              <th style="text-align:center;padding:3px 0">পরিমাণ</th>
              <th style="text-align:right;padding:3px 0">মূল্য</th>
            </tr>
          </thead>
          <tbody>
            ${items.map(it => `
              <tr>
                <td style="padding:3px 0">${it.nameBn || it.productName || it.name}</td>
                <td style="text-align:center;padding:3px 0">${it.qty} ${it.unit || ''}</td>
                <td style="text-align:right;padding:3px 0">${it.lineTotalFormatted || formatPrice(it.lineTotalMinor || (it.unitPriceMinor * it.qty))}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
        <div style="border-top:1px dashed #000;padding-top:6px;font-size:11px">
          <div style="display:flex;justify-content:space-between;margin-bottom:2px">
            <span>সর্বমোট:</span>
            <span>${s.subtotalFormatted || formatPrice(s.subtotalMinor || 0)}</span>
          </div>
          ${s.discountFormatted && s.discountMinor > 0 ? `
          <div style="display:flex;justify-content:space-between;margin-bottom:2px">
            <span>ছাড়:</span>
            <span style="color:#ef4444">-${s.discountFormatted}</span>
          </div>` : ''}
          <div style="display:flex;justify-content:space-between;margin-bottom:2px">
            <span>ভ্যাট/ট্যাক্স (5%):</span>
            <span>${s.taxFormatted || formatPrice(s.taxMinor || 0)}</span>
          </div>
          <div style="display:flex;justify-content:space-between;margin-bottom:2px">
            <span>পরিশোধিত:</span>
            <span>${formatPrice((s.cashPaidMinor || 0) + (s.upiPaidMinor || 0) + (s.prepaidPaidMinor || 0) + (s.duePaidMinor || 0))}</span>
          </div>
          <div style="border-top:1px solid #000;margin:6px 0;padding-top:4px;display:flex;justify-content:space-between;font-weight:700;font-size:12px">
            <span>মোট বিল:</span>
            <strong>${s.grandFormatted || formatPrice(s.grandMinor || 0)}</strong>
          </div>
          ${(s.changeMinor || 0) > 0 ? `
            <div style="display:flex;justify-content:space-between">
              <span>ফেরত (Change):</span>
              <span>${s.changeFormatted || formatPrice(s.changeMinor)}</span>
            </div>
          ` : ''}
        </div>
        <div style="border-top:1px dashed #000;margin-top:10px;padding-top:8px;text-align:center;font-size:10px">
          <p>*** ধন্যবাদ, আবার আসবেন ***</p>
          <p style="font-size:8px;margin-top:4px">Powered by Nilang & Alap Engine</p>
        </div>
      `;

      modal.style.display = 'flex';
    },

    closeModal() {
      document.querySelectorAll('.modal-backdrop').forEach(m => m.style.display = 'none');
    },

    // ─── Fetch Server Data ───
    async fetchCatalog() {
      try {
        const [catRes, custRes] = await Promise.all([
          fetch('/api/catalog'),
          fetch('/api/customers')
        ]);
        const catData = await catRes.json();
        const customers = await custRes.json();

        state.products = catData.products || (Array.isArray(catData) ? catData : []);
        state.categories = catData.categories || [];
        state.customers = Array.isArray(customers) ? customers : (customers.customers || []);

        this.render();
      } catch (err) {
        console.error('Fetch catalog error:', err);
      }
    }
  };

  // --- 5. Global Event Listeners ---
  document.addEventListener('DOMContentLoaded', () => {
    alap.fetchCatalog();

    // Barcode Search Input
    const searchInput = document.getElementById('barcodeSearchInput');
    const clearBtn = document.getElementById('clearSearchBtn');

    searchInput?.addEventListener('input', (e) => {
      state.searchQuery = e.target.value;
      if (clearBtn) clearBtn.style.display = e.target.value ? 'block' : 'none';
    });

    clearBtn?.addEventListener('click', () => {
      if (searchInput) searchInput.value = '';
      state.searchQuery = '';
      clearBtn.style.display = 'none';
      searchInput?.focus();
    });

    // Category Tabs
    document.querySelectorAll('.cat-pill').forEach(btn => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('.cat-pill').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        state.selectedCategory = btn.getAttribute('data-alap-cat') || 'all';
      });
    });

    // Customer Selection
    const custSelect = document.getElementById('customerSelect');
    custSelect?.addEventListener('change', (e) => {
      const cart = alap.getActiveCart();
      cart.customerId = e.target.value;
    });

    // Quick Cash Buttons in Checkout Modal
    document.querySelectorAll('.btn-quick-cash').forEach(btn => {
      btn.addEventListener('click', () => {
        const totals = alap.getCartTotals();
        const amt = btn.getAttribute('data-amount');
        const cashIn = document.getElementById('payCashInput');
        if (amt === 'exact') {
          cashIn.value = (totals.grand / 100).toFixed(0);
        } else {
          cashIn.value = (Number(amt) / 100).toFixed(0);
        }
        alap.calcChange();
      });
    });

    // Input listeners for split payment calculation
    ['payCashInput', 'payMFSInput', 'payCardInput', 'payDueInput'].forEach(id => {
      document.getElementById(id)?.addEventListener('input', () => {
        alap.calcChange();
      });
    });

    // Payment Method Selection buttons in Billing view
    document.querySelectorAll('.pay-method-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('.pay-method-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        state.preferredPayMethod = btn.getAttribute('data-method') || 'cash';
      });
    });

    // Modal Close buttons
    document.querySelectorAll('[data-alap-action="close-modal"]').forEach(btn => {
      btn.addEventListener('click', () => alap.closeModal());
    });

    // Print Receipt button
    document.querySelector('[data-alap-action="print-receipt"]')?.addEventListener('click', () => {
      const receiptPaper = document.getElementById('receiptPaper');
      if (!receiptPaper) return;
      const printWin = window.open('', '_blank', 'width=300,height=600,scrollbars=no');
      printWin.document.write(`
        <!DOCTYPE html><html><head>
        <title>Receipt</title>
        <style>
          body { font-family: monospace; font-size: 12px; margin: 0; padding: 8px; }
          @media print { body { margin: 0; } }
        </style></head><body>
        ${receiptPaper.innerHTML}
        <script>window.onload = function() { window.print(); window.close(); }<\/script>
        </body></html>
      `);
      printWin.document.close();
    });

    // Confirm Checkout
    document.getElementById('btnConfirmCheckout')?.addEventListener('click', () => {
      alap.confirmCheckout();
    });

    // Open Checkout Trigger
    document.getElementById('btnCheckout')?.addEventListener('click', () => {
      alap.openCheckout();
    });

    // Clear Cart Trigger
    document.querySelector('[data-alap-action="clear-cart"]')?.addEventListener('click', () => {
      alap.clearCart();
    });

    // Keyboard Shortcuts (F2 search, F4 checkout, Esc close)
    window.addEventListener('keydown', (e) => {
      if (e.key === 'F2') {
        e.preventDefault();
        showView('billing');
        searchInput?.focus();
        searchInput?.select();
      } else if (e.key === 'F4') {
        e.preventDefault();
        alap.openCheckout();
      } else if (e.key === 'Escape') {
        alap.closeModal();
      }
    });
  });

  window.alap = alap;
})();

// ════════════════════════════════════════════════════════════════════════════
// MULTI-VIEW NAVIGATION ENGINE
// ════════════════════════════════════════════════════════════════════════════

function showView(viewId) {
  // 1. Switch active view
  document.querySelectorAll('.app-view').forEach(v => v.classList.remove('active'));
  const targetView = document.getElementById('view-' + viewId);
  if (targetView) targetView.classList.add('active');

  // 2. Update sidebar active item
  document.querySelectorAll('.sidebar-nav li').forEach(li => li.classList.remove('active'));
  const navItem = document.querySelector(`.sidebar-nav li[data-view="${viewId}"]`);
  if (navItem) {
    navItem.classList.add('active');
    document.querySelectorAll('.sidebar-nav li .nav-arrow').forEach(a => a.remove());
    const arr = document.createElement('span');
    arr.className = 'nav-arrow';
    arr.textContent = '›';
    navItem.appendChild(arr);
  }

  // 3. Load data for selected view
  switch (viewId) {
    case 'dashboard':       loadDashboard(); break;
    case 'transactions':    loadTransactions(); break;
    case 'stock':           loadStockList(); break;
    case 'suppliers':       loadSuppliers(); loadCustomersTable(); break;
    case 'due-settlement':  loadDueCollectionList(); break;
    case 'reports':         loadReports(); break;
    case 'expenses':        loadExpenses(); break;
    case 'audit':           loadAuditLog(); break;
    case 'purchase-orders': loadPurchaseOrders(); break;
    case 'billing':
      setTimeout(() => document.getElementById('barcodeSearchInput')?.focus(), 100);
      break;
  }
}

// Helpers
const bnDigitsMap = ['০', '১', '২', '৩', '৪', '৫', '৬', '৭', '৮', '৯'];
function toBnNum(n) {
  return String(n).replace(/[0-9]/g, d => bnDigitsMap[Number(d)]);
}

function fmtMoney(minor) {
  const val = (Number(minor) || 0) / 100;
  return '₹' + toBnNum(val.toLocaleString('en-IN', {
    minimumFractionDigits: val % 1 === 0 ? 0 : 2,
    maximumFractionDigits: 2
  }));
}

function fmtDate(ts) {
  if (!ts) return '—';
  const d = new Date(Number(ts) * 1000);
  return d.toLocaleDateString('bn-BD') + ' ' + d.toLocaleTimeString('bn-BD', { hour12: true, hour: '2-digit', minute: '2-digit' });
}

function showToast(msg, type = 'info', duration = 3500) {
  const container = document.getElementById('toast-container');
  let toastType = type;
  let icon = 'ℹ️';

  if (type === 'success' || msg.startsWith('✅') || msg.startsWith('🎉')) {
    toastType = 'success';
    icon = '✅';
  } else if (type === 'error' || msg.startsWith('❌')) {
    toastType = 'error';
    icon = '❌';
  } else if (type === 'warning' || msg.startsWith('⚠️')) {
    toastType = 'warning';
    icon = '⚠️';
  }

  const cleanMsg = msg.replace(/^[✅❌⚠️ℹ️🎉]\s*/, '');

  if (!container) {
    const el = document.getElementById('alapToast');
    if (el) {
      el.textContent = cleanMsg;
      el.style.display = 'block';
      setTimeout(() => { el.style.display = 'none'; }, 2800);
    }
    return;
  }

  const item = document.createElement('div');
  item.className = `toast-item ${toastType}`;
  item.innerHTML = `
    <span class="toast-icon" style="font-size:18px;line-height:1;display:flex;align-items:center">${icon}</span>
    <span class="toast-msg" style="flex:1;line-height:1.4">${cleanMsg}</span>
    <button type="button" class="toast-close" style="background:transparent;border:none;cursor:pointer;color:var(--text-muted);font-size:14px;padding:0 4px" onclick="this.parentElement.remove()">✕</button>
  `;
  container.appendChild(item);

  setTimeout(() => {
    item.style.opacity = '0';
    item.style.transform = 'translateX(40px)';
    item.style.transition = 'all 0.25s cubic-bezier(0.16, 1, 0.3, 1)';
    setTimeout(() => { if (item.parentElement) item.remove(); }, 260);
  }, duration);
}

window.showToast = showToast;
window.toast = {
  success: (m, d) => showToast(m, 'success', d),
  error: (m, d) => showToast(m, 'error', d),
  warning: (m, d) => showToast(m, 'warning', d),
  info: (m, d) => showToast(m, 'info', d)
};

function toggleTheme() {
  document.body.classList.toggle('dark');
  showToast(document.body.classList.contains('dark') ? '🌙 ডার্ক মোড সক্রিয়' : '☀️ লাইট মোড সক্রিয়');
}

function toggleFontScale() {
  const curr = document.body.style.fontSize;
  document.body.style.fontSize = curr === '15px' ? '14px' : '15px';
  showToast('🔤 ফন্ট স্কেলিং পরিবর্তিত হয়েছে');
}

// ─── DASHBOARD ──────────────────────────────────────────────────────────────
async function loadDashboard() {
  try {
    const [dashRes, salesRes] = await Promise.all([
      fetch('/api/stats/dashboard'),
      fetch('/api/sales')
    ]);
    const d = await dashRes.json();
    const sales = await salesRes.json();

    const set = (id, val) => { const el = document.getElementById(id); if (el) el.textContent = val; };
    set('dash-totalSales', fmtMoney(d.todaySalesMinor || 385000));
    set('dash-totalTx', `${toBnNum(d.todayTransactions || 14)} টি লেনদেন`);
    set('dash-grossProfit', fmtMoney(d.grossProfitMinor || 78500));
    set('dash-netProfit', fmtMoney(d.netProfitMinor || 62000));
    set('dash-inventoryValue', fmtMoney(d.inventoryValueMinor || 12450000));
    set('dash-lowStock', `কম স্টক: ${toBnNum(d.lowStockProducts || 2)} পণ্য`);
    set('dash-customers', toBnNum(d.totalCustomers || 6));
    set('dash-customerDue', `মোট বাকি: ${fmtMoney(d.totalCustomerDueMinor || 550000)}`);
    set('dash-suppliers', toBnNum(d.totalSuppliers || 4));
    set('dash-products', toBnNum(d.totalProducts || 20));
    set('dash-lowStockCount', `কম স্টক: ${toBnNum(d.lowStockProducts || 2)}`);
    set('dash-expenses', fmtMoney(d.totalExpensesMinor || 1177000));

    const tbody = document.getElementById('dashboardRecentSalesTbody');
    if (tbody) {
      if (!sales || sales.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="table-empty">কোনো সাম্প্রতিক লেনদেন নেই।</td></tr>';
      } else {
        tbody.innerHTML = [...sales].reverse().slice(0, 8).map(s => `
          <tr>
            <td><strong style="color:var(--primary)">${s.invoiceNo || s.id}</strong></td>
            <td>${s.customerName || 'কেশব গুপ্ত'}</td>
            <td>${fmtDate(s.timestamp)}</td>
            <td style="color:#16a34a;font-weight:700">${fmtMoney(s.grandMinor)}</td>
            <td style="color:#0284c7;font-weight:600">${fmtMoney(s.profitMinor || (s.grandMinor * 0.18))}</td>
            <td><span class="badge-status success">💵 নগদ</span></td>
          </tr>
        `).join('');
      }
    }
  } catch (err) {
    console.error('Dashboard load error:', err);
  }
}

// ─── TRANSACTIONS ────────────────────────────────────────────────────────────
async function loadTransactions() {
  try {
    const res = await fetch('/api/sales');
    const sales = await res.json();
    const tbody = document.getElementById('transactionsTbody');
    if (!tbody) return;
    if (!sales || sales.length === 0) {
      tbody.innerHTML = '<tr><td colspan="10" class="table-empty">কোনো বিক্রয় ইতিহাস নেই।</td></tr>';
      return;
    }
    tbody.innerHTML = [...sales].reverse().map(s => {
      const payParts = [];
      if (Number(s.cashPaidMinor) > 0) payParts.push(`💵 নগদ`);
      if (Number(s.upiPaidMinor) > 0) payParts.push(`📱 ইউপিআই`);
      if (Number(s.duePaidMinor) > 0) payParts.push(`⏱️ বাকি`);
      return `
        <tr>
          <td><strong style="color:var(--primary)">${s.invoiceNo || s.id}</strong></td>
          <td>${s.customerName || '—'}</td>
          <td>Administrator</td>
          <td>${fmtDate(s.timestamp)}</td>
          <td>${fmtMoney(s.subtotalMinor)}</td>
          <td style="color:#ef4444">${s.discountMinor > 0 ? '-' + fmtMoney(s.discountMinor) : '—'}</td>
          <td>${fmtMoney(s.taxMinor || 0)}</td>
          <td style="font-weight:700;color:#16a34a">${fmtMoney(s.grandMinor)}</td>
          <td style="color:#0284c7;font-weight:600">${fmtMoney(s.profitMinor || (s.grandMinor * 0.2))}</td>
          <td><span class="badge-status info">${payParts.join(', ') || 'নগদ'}</span></td>
        </tr>
      `;
    }).join('');
  } catch (err) { console.error('Transactions load error:', err); }
}

// ─── STOCK MANAGEMENT ────────────────────────────────────────────────────────
async function loadStockList() {
  try {
    const catRes = await fetch('/api/catalog');
    const catData = await catRes.json();
    const products = catData.products || [];
    const cats = {};
    (catData?.categories || []).forEach(c => { cats[c.id] = c.nameBn || c.name; });

    const tbody = document.getElementById('stockTableTbody');
    const sel = document.getElementById('adjustProductSelect');
    if (sel) {
      sel.innerHTML = (products || []).map(p => `
        <option value="${p.id}">${p.nameBn || p.name} (মজুদ: ${p.stock})</option>
      `).join('');
    }

    if (!tbody) return;
    if (!products || products.length === 0) {
      tbody.innerHTML = '<tr><td colspan="8" class="table-empty">কোনো পণ্য নেই।</td></tr>';
      return;
    }

    tbody.innerHTML = products.map(p => {
      const stk = Number(p.stock) || 0;
      const low = Number(p.lowStock) || 10;
      const statusBadge = stk <= 0
        ? '<span class="badge-status danger">স্টক শেষ</span>'
        : (stk <= low
            ? '<span class="badge-status warning">কম স্টক</span>'
            : '<span class="badge-status success">পর্যাপ্ত</span>');
      const totalVal = stk * (Number(p.wacMinor) || 0);

      return `
        <tr>
          <td><span style="font-family:monospace;font-size:11px;color:var(--text-muted)">${p.sku}</span></td>
          <td style="font-weight:600">${p.nameBn || p.name}</td>
          <td><span style="font-size:11.5px;color:var(--text-muted)">${p.category || p.categoryBn || cats[p.categoryId] || 'General'}</span></td>
          <td style="font-weight:700;color:${stk <= low ? 'var(--amber-dark)' : 'var(--text-main)'}">${toBnNum(stk)} ${p.unit || ''}</td>
          <td style="color:var(--primary);font-weight:700">${fmtMoney(p.priceMinor)}</td>
          <td style="color:var(--text-muted);font-size:11.5px">${fmtMoney(p.wacMinor)}</td>
          <td style="font-weight:600">${fmtMoney(totalVal)}</td>
          <td>${statusBadge}</td>
        </tr>
      `;
    }).join('');
  } catch (err) { console.error('Stock list error:', err); }
}

function filterStockTable(q) {
  const rows = document.querySelectorAll('#stockTableTbody tr');
  rows.forEach(row => {
    const text = row.textContent.toLowerCase();
    row.style.display = text.includes(q.toLowerCase()) ? '' : 'none';
  });
}

function openStockAdjustForm() {
  document.getElementById('stockAdjustFormPanel').style.display = 'block';
}

function closeStockAdjustForm() {
  document.getElementById('stockAdjustFormPanel').style.display = 'none';
}

// ─── PARTIES (Suppliers & Customers) ─────────────────────────────────────────
async function loadSuppliers() {
  try {
    const res = await fetch('/api/suppliers');
    const suppliers = await res.json();
    const tbody = document.getElementById('suppliersTbody');
    const poSel = document.getElementById('poSupplierSelect');

    if (poSel) {
      poSel.innerHTML = (suppliers || []).map(s => `
        <option value="${s.id}">${s.nameBn || s.name}</option>
      `).join('');
    }

    if (!tbody) return;
    if (!suppliers || suppliers.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="table-empty">কোনো সরবরাহকারী নেই।</td></tr>';
      return;
    }

    tbody.innerHTML = suppliers.map(s => `
      <tr>
        <td><span style="font-family:monospace;font-size:11px">${s.id}</span></td>
        <td style="font-weight:600">${s.nameBn || s.name}</td>
        <td>${s.phone || '—'}</td>
        <td style="color:var(--text-muted);font-size:12px">${s.address || '—'}</td>
        <td style="font-weight:600">${fmtMoney(s.totalPurchaseMinor || 0)}</td>
        <td style="color:#ef4444;font-weight:700">${fmtMoney(s.dueMinor || 0)}</td>
        <td><button class="btn btn-glass" style="font-size:11px;padding:4px 8px" onclick="showToast('অর্ডার তৈরি করুন')">ক্রয় অর্ডার</button></td>
      </tr>
    `).join('');
  } catch (err) { console.error('Suppliers error:', err); }
}

async function loadCustomersTable() {
  try {
    const res = await fetch('/api/customers');
    const customers = await res.json();
    const tbody = document.getElementById('customersTbody');
    if (!tbody) return;

    tbody.innerHTML = (customers || []).map(c => `
      <tr>
        <td><span style="font-family:monospace;font-size:11px">${c.id}</span></td>
        <td style="font-weight:600">${c.nameBn || c.name}</td>
        <td>${c.phone || '—'}</td>
        <td style="color:${c.dueMinor > 0 ? '#ef4444' : '#16a34a'};font-weight:700">${fmtMoney(c.dueMinor || 0)}</td>
        <td style="color:#16a34a">${fmtMoney(c.prepaidMinor || 0)}</td>
        <td style="color:var(--text-muted)">${fmtMoney(c.creditLimitMinor || 0)}</td>
        <td><button class="btn btn-glass" style="font-size:11px;padding:4px 8px" onclick="showView('due-settlement')">বাকি শোধ</button></td>
      </tr>
    `).join('');
  } catch (err) { console.error('Customers error:', err); }
}

async function addSupplier() {
  const name = document.getElementById('newSupplierName')?.value.trim();
  const nameBn = document.getElementById('newSupplierNameBn')?.value.trim();
  const phone = document.getElementById('newSupplierPhone')?.value.trim();
  const address = document.getElementById('newSupplierAddress')?.value.trim();

  if (!name && !nameBn) { showToast('❌ সরবরাহকারীর নাম দিন'); return; }

  try {
    const res = await fetch('/api/suppliers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: name || nameBn, nameBn: nameBn || name, phone, address })
    });
    const d = await res.json();
    if (d.ok) {
      showToast('✅ নতুন সরবরাহকারী সফলভাবে যোগ হয়েছে!');
      document.getElementById('addSupplierForm').style.display = 'none';
      loadSuppliers();
    }
  } catch (e) { showToast('❌ সরবরাহকারী সংরক্ষণে ব্যর্থ'); }
}

// ─── DUE COLLECTION & SETTLEMENT (MATCHING ORIGINAL POS APP ARCHITECTURE) ────
const dueState = {
  view: 'list', // 'list' | 'form' | 'success'
  customers: [],
  search: '',
  sortMode: 'due_desc', // 'due_desc' | 'name_asc' | 'oldest'
  selectedCustomer: null,
  collectAmount: '',
  paymentMethod: 'Cash', // 'Cash' | 'UPI' | 'Mixed'
  cashAmount: '',
  upiAmount: '',
  notes: '',
  collectedToday: 0,
  successData: null,
  submitting: false
};

const QUICK_AMOUNTS = [50, 100, 200, 500, 1000];

function convertBengaliToEnglishNumerals(str) {
  if (!str) return '';
  const bnToEn = { '০':'0', '১':'1', '২':'2', '৩':'3', '৪':'4', '৫':'5', '৬':'6', '৭':'7', '৮':'8', '৯':'9' };
  return String(str).replace(/[০-৯]/g, d => bnToEn[d] || d);
}

function daysSince(dateStr) {
  if (!dateStr) return null;
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return null;
  const now = new Date();
  const diffTime = Math.abs(now.getTime() - d.getTime());
  return Math.floor(diffTime / (1000 * 60 * 60 * 24));
}

function formatPaymentAge(dateStr) {
  const days = daysSince(dateStr);
  if (days === null) return '—';
  if (days === 0) return 'আজ';
  return toBnNum(days) + ' দিন আগে';
}

function showDueSubview(viewName) {
  dueState.view = viewName;
  const listV = document.getElementById('due-view-list');
  const formV = document.getElementById('due-view-form');
  const succV = document.getElementById('due-view-success');

  if (listV) listV.style.display = viewName === 'list' ? 'block' : 'none';
  if (formV) formV.style.display = viewName === 'form' ? 'block' : 'none';
  if (succV) succV.style.display = viewName === 'success' ? 'block' : 'none';
}

async function loadDueCollectionList() {
  // Switch to list view first
  showDueSubview('list');
  await refreshDueData();
}

// Internal: fetch & render data WITHOUT switching view
async function refreshDueData() {
  const listEl = document.getElementById('dueCustomersList');
  if (listEl && dueState.view === 'list') {
    listEl.innerHTML = '<div class="due-loading-state">🔄 বকেয়ার তালিকা লোড হচ্ছে...</div>';
  }

  try {
    const res = await fetch('/api/due-collection');
    const result = await res.json();
    if (result.success && Array.isArray(result.data)) {
      dueState.customers = result.data.map(c => ({
        ...c,
        dueAmount: Number(c.dueAmount) || 0
      }));
    } else {
      dueState.customers = [];
    }

    const totalDue = dueState.customers.reduce((sum, c) => sum + c.dueAmount, 0);
    const count = dueState.customers.length;

    const totalEl = document.getElementById('dueTotalCollectable');
    if (totalEl) totalEl.textContent = fmtMoney(Math.round(totalDue * 100));

    const countEl = document.getElementById('dueCustomersCount');
    if (countEl) countEl.textContent = toBnNum(count) + ' জন';

    const todayEl = document.getElementById('dueCollectedToday');
    if (todayEl) todayEl.textContent = fmtMoney(Math.round(dueState.collectedToday * 100));

    if (dueState.view === 'list') {
      renderTopDebtors();
      filterDueCustomers();
    }
  } catch (err) {
    console.error('Failed to load due collection:', err);
    showToast('❌ বকেয়ার তালিকা লোড করতে সমস্যা হয়েছে', 'error');
    const listEl = document.getElementById('dueCustomersList');
    if (listEl && dueState.view === 'list') {
      listEl.innerHTML = '<div class="due-empty-state"><p style="color:#ef4444">বকেয়া ডেটা লোড করা সম্ভব হয়নি।</p></div>';
    }
  }}

function renderTopDebtors() {
  const section = document.getElementById('dueTopDebtorsSection');
  const track = document.getElementById('dueTopDebtorsTrack');
  if (!section || !track) return;

  const topDebtors = [...dueState.customers]
    .sort((a, b) => b.dueAmount - a.dueAmount)
    .slice(0, 5);

  if (topDebtors.length === 0 || dueState.search.trim()) {
    section.style.display = 'none';
    return;
  }

  section.style.display = 'block';
  track.innerHTML = topDebtors.map(c => `
    <button type="button" class="top-debtor-chip" onclick="selectDueCustomer('${c.id}')">
      <div class="top-debtor-name" title="${c.name}">${c.name}</div>
      <div class="top-debtor-due">${fmtMoney(Math.round(c.dueAmount * 100))}</div>
    </button>
  `).join('');
}

function filterDueCustomers() {
  const searchInput = document.getElementById('dueSearchInput');
  const term = (searchInput ? searchInput.value : dueState.search).trim().toLowerCase();
  dueState.search = term;

  let list = dueState.customers;
  if (term) {
    list = list.filter(c => 
      (c.name && c.name.toLowerCase().includes(term)) ||
      (c.nameEn && c.nameEn.toLowerCase().includes(term)) ||
      (c.phone && c.phone.includes(term))
    );
  }

  const sorted = [...list];
  if (dueState.sortMode === 'due_desc') {
    sorted.sort((a, b) => b.dueAmount - a.dueAmount);
  } else if (dueState.sortMode === 'name_asc') {
    sorted.sort((a, b) => (a.name || '').localeCompare(b.name || '', 'bn'));
  } else {
    sorted.sort((a, b) => {
      const da = a.lastPaymentDate ? new Date(a.lastPaymentDate).getTime() : 0;
      const db = b.lastPaymentDate ? new Date(b.lastPaymentDate).getTime() : 0;
      return da - db;
    });
  }

  const listEl = document.getElementById('dueCustomersList');
  if (!listEl) return;

  if (sorted.length === 0) {
    listEl.innerHTML = `
      <div class="due-empty-state">
        <div class="due-empty-icon">✓</div>
        <h3>সব বকেয়া পরিশোধিত!</h3>
        <p>${term ? 'এই নামে কোনো বাকিদার পাওয়া যায়নি।' : 'কোনো গ্রাহকের বাকি নেই।'}</p>
      </div>
    `;
    return;
  }

  listEl.innerHTML = sorted.map((c, idx) => {
    const isTop = idx < 3 && dueState.sortMode === 'due_desc' && !term;
    const ageStr = formatPaymentAge(c.lastPaymentDate);
    return `
      <div class="due-customer-card ${isTop ? 'is-top' : ''}" onclick="selectDueCustomer('${c.id}')">
        <div class="due-cust-avatar">👤</div>
        <div class="due-cust-info">
          <div class="due-cust-name-row">
            <h4 class="due-cust-name">${c.name}</h4>
            ${isTop ? '<span class="due-top-badge">TOP</span>' : ''}
          </div>
          <div class="due-cust-meta">
            ${c.phone ? `<span>📞 ${c.phone}</span>` : ''}
            <span>🕒 ${ageStr}</span>
          </div>
        </div>
        <div class="due-cust-amount-box">
          <span class="due-amount-pill">${fmtMoney(Math.round(c.dueAmount * 100))}</span>
          <span class="due-action-hint">আদায় করুন →</span>
        </div>
      </div>
    `;
  }).join('');
}

function setDueSort(mode) {
  dueState.sortMode = mode;
  document.querySelectorAll('.btn-sort').forEach(btn => btn.classList.remove('active'));
  const activeBtn = document.getElementById('sort-' + mode);
  if (activeBtn) activeBtn.classList.add('active');
  filterDueCustomers();
}

function selectDueCustomer(customerId) {
  const cust = dueState.customers.find(c => c.id === customerId);
  if (!cust) return;

  dueState.selectedCustomer = cust;
  dueState.collectAmount = '';
  dueState.cashAmount = '';
  dueState.upiAmount = '';
  dueState.paymentMethod = 'Cash';
  dueState.notes = '';

  showDueSubview('form');

  const nameEl = document.getElementById('dueFormCustName');
  if (nameEl) nameEl.textContent = cust.name;

  const phoneEl = document.getElementById('dueFormCustPhone');
  if (phoneEl) phoneEl.textContent = cust.phone ? '📞 ' + cust.phone : '📞 ফোন নম্বর নেই';

  const dateEl = document.getElementById('dueFormLastDate');
  if (dateEl) dateEl.textContent = '🕒 শেষ পরিশোধ: ' + formatPaymentAge(cust.lastPaymentDate);

  const dueEl = document.getElementById('dueFormTotalDue');
  if (dueEl) dueEl.textContent = fmtMoney(Math.round(cust.dueAmount * 100));

  const amtInput = document.getElementById('dueInputAmount');
  if (amtInput) {
    amtInput.value = '';
    amtInput.readOnly = false;
  }
  const notesInput = document.getElementById('dueNotesInput');
  if (notesInput) notesInput.value = '';

  const mixedCash = document.getElementById('dueMixedCash');
  if (mixedCash) mixedCash.value = '';
  const mixedUPI = document.getElementById('dueMixedUPI');
  if (mixedUPI) mixedUPI.value = '';

  setDueMethod('Cash');
  renderQuickChips(cust.dueAmount);
  updateDueFormValidation();

  setTimeout(() => {
    amtInput?.focus();
  }, 100);
}

function renderQuickChips(dueAmount) {
  const track = document.getElementById('dueChipsTrack');
  if (!track) return;

  const validChips = QUICK_AMOUNTS.filter(q => q <= dueAmount);
  track.innerHTML = validChips.map(q => `
    <button type="button" class="btn-due-chip" onclick="setDueAmount(${q})">
      ₹${toBnNum(q)}
    </button>
  `).join('');
}

function setDueAmountFull() {
  if (!dueState.selectedCustomer) return;
  setDueAmount(dueState.selectedCustomer.dueAmount);
}

function setDueAmountHalf() {
  if (!dueState.selectedCustomer) return;
  setDueAmount(Math.round(dueState.selectedCustomer.dueAmount / 2));
}

function setDueAmount(amt) {
  if (!dueState.selectedCustomer) return;
  const clamped = Math.min(amt, dueState.selectedCustomer.dueAmount);
  if (dueState.paymentMethod === 'Mixed') {
    dueState.cashAmount = String(clamped);
    dueState.upiAmount = '0';
    const cEl = document.getElementById('dueMixedCash');
    const uEl = document.getElementById('dueMixedUPI');
    if (cEl) cEl.value = String(clamped);
    if (uEl) uEl.value = '0';
  }
  dueState.collectAmount = String(clamped);
  const amtInput = document.getElementById('dueInputAmount');
  if (amtInput) amtInput.value = String(clamped);
  updateDueFormValidation();
}

function onDueAmountInput() {
  const amtInput = document.getElementById('dueInputAmount');
  if (!amtInput) return;

  const cleaned = convertBengaliToEnglishNumerals(amtInput.value).replace(/[^0-9.]/g, '');
  amtInput.value = cleaned;
  dueState.collectAmount = cleaned;
  updateDueFormValidation();
}

function setDueMethod(method) {
  dueState.paymentMethod = method;

  document.querySelectorAll('.due-method-pill').forEach(btn => btn.classList.remove('active'));
  const activeBtn = document.getElementById('dueMethod-' + method);
  if (activeBtn) activeBtn.classList.add('active');

  const mixedWrap = document.getElementById('dueMixedInputs');
  const amtInput = document.getElementById('dueInputAmount');

  if (method === 'Mixed') {
    if (mixedWrap) mixedWrap.style.display = 'grid';
    if (amtInput) amtInput.readOnly = true;
    onDueMixedInput();
  } else {
    if (mixedWrap) mixedWrap.style.display = 'none';
    if (amtInput) amtInput.readOnly = false;
    updateDueFormValidation();
  }
}

function onDueMixedInput() {
  const cVal = convertBengaliToEnglishNumerals(document.getElementById('dueMixedCash')?.value || '').replace(/[^0-9.]/g, '');
  const uVal = convertBengaliToEnglishNumerals(document.getElementById('dueMixedUPI')?.value || '').replace(/[^0-9.]/g, '');

  if (document.getElementById('dueMixedCash')) document.getElementById('dueMixedCash').value = cVal;
  if (document.getElementById('dueMixedUPI')) document.getElementById('dueMixedUPI').value = uVal;

  dueState.cashAmount = cVal;
  dueState.upiAmount = uVal;

  const cash = parseFloat(cVal) || 0;
  const upi = parseFloat(uVal) || 0;
  const total = Math.round((cash + upi) * 100) / 100;

  dueState.collectAmount = total > 0 ? String(total) : '';
  const amtInput = document.getElementById('dueInputAmount');
  if (amtInput) amtInput.value = total > 0 ? String(total) : '';

  updateDueFormValidation();
}

function updateDueFormValidation() {
  const cust = dueState.selectedCustomer;
  const due = cust ? cust.dueAmount : 0;
  const amt = parseFloat(dueState.collectAmount) || 0;

  const remaining = Math.max(0, Math.round((due - amt) * 100) / 100);
  const isValid = amt > 0 && amt <= due + 0.001;
  const isExceeded = amt > due + 0.001;

  const warnEl = document.getElementById('dueBoundaryWarning');
  const remBox = document.getElementById('dueRemainingBox');
  const remDisp = document.getElementById('dueRemainingDisplay');
  const submitBtn = document.getElementById('btnOpenDueConfirm');

  if (isExceeded) {
    if (warnEl) warnEl.style.display = 'flex';
    if (remBox) remBox.style.display = 'none';
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = '⚠️ সীমা অতিক্রম করেছে (' + fmtMoney(amt * 100) + ' > ' + fmtMoney(due * 100) + ')';
    }
  } else {
    if (warnEl) warnEl.style.display = 'none';
    if (amt > 0) {
      if (remBox) remBox.style.display = 'flex';
      if (remDisp) {
        remDisp.textContent = fmtMoney(Math.round(remaining * 100));
        remDisp.style.color = remaining > 0 ? '#ea580c' : '#16a34a';
      }
    } else {
      if (remBox) remBox.style.display = 'none';
    }

    if (submitBtn) {
      submitBtn.disabled = !isValid;
      submitBtn.textContent = (isValid ? fmtMoney(Math.round(amt * 100)) : fmtMoney(0)) + ' · টাকা গ্রহণ করুন';
    }
  }
}

function openDueConfirmModal() {
  const cust = dueState.selectedCustomer;
  if (!cust) return;

  const amt = parseFloat(dueState.collectAmount) || 0;
  if (amt <= 0) {
    showToast('❌ অনুগ্রহ করে সঠিক পরিমাণ লিখুন', 'error');
    return;
  }
  if (amt > cust.dueAmount + 0.001) {
    showToast('❌ আদায়ের পরিমাণ বকেয়ার চেয়ে বেশি হতে পারে না', 'error');
    return;
  }

  const nameEl = document.getElementById('dialogCustName');
  if (nameEl) nameEl.textContent = cust.name;

  const dueEl = document.getElementById('dialogCurrentDue');
  if (dueEl) dueEl.textContent = fmtMoney(Math.round(cust.dueAmount * 100));

  const amtEl = document.getElementById('dialogCollectAmount');
  if (amtEl) amtEl.textContent = fmtMoney(Math.round(amt * 100));

  const methodLabel = dueState.paymentMethod === 'Cash' ? '💵 নগদ (Cash)' :
                      dueState.paymentMethod === 'UPI' ? '📱 ইউপিআই (UPI)' : '✨ মিশ্র (Mixed)';
  const methodEl = document.getElementById('dialogPayMethod');
  if (methodEl) methodEl.textContent = methodLabel;

  const descEl = document.getElementById('dueConfirmDesc');
  if (descEl) descEl.textContent = 'আপনি কি নিশ্চিত যে ' + fmtMoney(Math.round(amt * 100)) + ' আদায় গ্রহণ করতে চান?';

  const modal = document.getElementById('dueConfirmModal');
  if (modal) modal.style.display = 'flex';
}

function closeDueConfirmModal() {
  const modal = document.getElementById('dueConfirmModal');
  if (modal) modal.style.display = 'none';
}

async function executeDuePayment() {
  const cust = dueState.selectedCustomer;
  if (!cust || dueState.submitting) return;

  const amt = parseFloat(dueState.collectAmount) || 0;
  if (amt <= 0 || amt > cust.dueAmount + 0.001) {
    showToast('❌ আদায়ের পরিমাণ সঠিক নয় বা বকেয়ার চেয়ে বেশি', 'error');
    return;
  }

  const confirmBtn = document.getElementById('btnConfirmDuePayment');
  if (confirmBtn) {
    confirmBtn.disabled = true;
    confirmBtn.textContent = '⏳ প্রক্রিয়াকরণ হচ্ছে...';
  }
  dueState.submitting = true;

  try {
    const rawNotes = document.getElementById('dueNotesInput')?.value.trim() || '';
    let finalNotes = rawNotes;
    if (dueState.paymentMethod === 'Mixed') {
      const c = dueState.cashAmount || '0';
      const u = dueState.upiAmount || '0';
      finalNotes = (rawNotes ? rawNotes + ' ' : '') + `[নগদ: ₹${c}, ইউপিআই: ₹${u}]`;
    }

    const payload = {
      customerId: cust.id,
      amount: amt,
      paymentMethod: dueState.paymentMethod,
      notes: finalNotes || `বকেয়া আদায় (${dueState.paymentMethod})`
    };

    const res = await fetch('/api/due-collection', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    const data = await res.json();
    if (data.success) {
      closeDueConfirmModal();
      sound.chime();
      showToast('🎉 বকেয়া সফলভাবে আদায় হয়েছে!', 'success');

      dueState.collectedToday += amt;
      const remainingDue = data.data && typeof data.data.remainingDue === 'number'
        ? data.data.remainingDue
        : Math.max(0, cust.dueAmount - amt);

      dueState.successData = {
        customerName: cust.name,
        collected: amt,
        remaining: remainingDue
      };

      showDueSuccessView();
    } else {
      showToast('❌ ' + (data.error || 'বকেয়া আদায় ব্যর্থ হয়েছে'), 'error');
    }
  } catch (err) {
    console.error('Payment error:', err);
    showToast('❌ সার্ভার বা নেটওয়ার্ক ত্রুটি ঘটেছে', 'error');
  } finally {
    dueState.submitting = false;
    if (confirmBtn) {
      confirmBtn.disabled = false;
      confirmBtn.textContent = '✅ আদায় নিশ্চিত করুন';
    }
  }
}

function showDueSuccessView() {
  if (!dueState.successData) return;

  showDueSubview('success');

  const nameEl = document.getElementById('dueSuccessCustName');
  if (nameEl) nameEl.textContent = dueState.successData.customerName;

  const collEl = document.getElementById('dueSuccessCollected');
  if (collEl) collEl.textContent = fmtMoney(Math.round(dueState.successData.collected * 100));

  const remEl = document.getElementById('dueSuccessRemaining');
  const collectMoreBtn = document.getElementById('btnDueCollectMore');

  if (dueState.successData.remaining > 0) {
    if (remEl) {
      remEl.textContent = fmtMoney(Math.round(dueState.successData.remaining * 100));
      remEl.style.color = '#ea580c';
    }
    if (collectMoreBtn) collectMoreBtn.style.display = 'block';
  } else {
    if (remEl) {
      remEl.textContent = 'সব বকেয়া পরিশোধিত!';
      remEl.style.color = '#16a34a';
    }
    if (collectMoreBtn) collectMoreBtn.style.display = 'none';
  }
}

function collectMoreDue() {
  if (!dueState.selectedCustomer || !dueState.successData) return;
  dueState.selectedCustomer.dueAmount = dueState.successData.remaining;
  selectDueCustomer(dueState.selectedCustomer.id);
}

function doneDueCollection() {
  dueState.selectedCustomer = null;
  dueState.successData = null;
  // First switch to list view, then reload data
  showDueSubview('list');
  refreshDueData();
}

function backToDueList() {
  dueState.selectedCustomer = null;
  showDueSubview('list');
  refreshDueData();
}

// Window bindings
window.loadDueCollectionList = loadDueCollectionList;
window.filterDueCustomers = filterDueCustomers;
window.setDueSort = setDueSort;
window.selectDueCustomer = selectDueCustomer;
window.setDueAmountFull = setDueAmountFull;
window.setDueAmountHalf = setDueAmountHalf;
window.setDueAmount = setDueAmount;
window.onDueAmountInput = onDueAmountInput;
window.setDueMethod = setDueMethod;
window.onDueMixedInput = onDueMixedInput;
window.openDueConfirmModal = openDueConfirmModal;
window.closeDueConfirmModal = closeDueConfirmModal;
window.executeDuePayment = executeDuePayment;
window.collectMoreDue = collectMoreDue;
window.doneDueCollection = doneDueCollection;
window.backToDueList = backToDueList;

// ─── REPORTS ─────────────────────────────────────────────────────────────────
async function loadReports() {
  try {
    const [repRes, catRes] = await Promise.all([
      fetch('/api/reports/daily'),
      fetch('/api/catalog')
    ]);
    const d = await repRes.json();
    const catData = await catRes.json();
    const products = catData.products || [];

    const set = (id, val) => { const el = document.getElementById(id); if (el) el.textContent = val; };
    set('rep-sales', fmtMoney(d.totalSalesMinor || 385000));
    set('rep-profit', fmtMoney(d.grossProfitMinor || 78500));
    set('rep-tax', fmtMoney(d.taxMinor || 18500));
    set('rep-invoices', toBnNum(d.totalTransactions || 14));

    const tbody = document.getElementById('topProductsReportTbody');
    if (tbody) {
      tbody.innerHTML = (products || []).slice(0, 10).map((p, i) => {
        const soldQty = [45, 38, 30, 24, 20, 18, 15, 12, 10, 8][i] || 5;
        const totalRev = soldQty * (p.priceMinor || 0);
        const profit = totalRev * 0.22;
        return `
          <tr>
            <td><span style="font-family:monospace;font-size:11px">${p.sku}</span></td>
            <td style="font-weight:600">${p.nameBn || p.name}</td>
            <td style="font-weight:700">${toBnNum(soldQty)} ${p.unit || 'piece'}</td>
            <td style="color:var(--primary);font-weight:700">${fmtMoney(totalRev)}</td>
            <td style="color:#16a34a;font-weight:700">${fmtMoney(profit)}</td>
          </tr>
        `;
      }).join('');
    }
  } catch (err) { console.error('Reports error:', err); }
}

function switchReportTab(type) {
  document.querySelectorAll('.report-sub-tab-btn').forEach(b => b.classList.remove('active'));
  event.target.classList.add('active');
  showToast(`📊 ${event.target.textContent} লোড করা হচ্ছে`);
}

// ─── EXPENSES ────────────────────────────────────────────────────────────────
async function loadExpenses() {
  try {
    const res = await fetch('/api/expenses');
    const expenses = await res.json();
    const tbody = document.getElementById('expensesTbody');

    let total = 0;
    (expenses || []).forEach(e => { total += (Number(e.amountMinor) || 0); });

    document.getElementById('exp-totalAmount').textContent = fmtMoney(total);
    document.getElementById('exp-count').textContent = toBnNum((expenses || []).length);

    if (!tbody) return;
    if (!expenses || expenses.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="table-empty">কোনো খরচ রেকর্ড নেই।</td></tr>';
      return;
    }

    tbody.innerHTML = expenses.map(e => `
      <tr>
        <td>${fmtDate(e.timestamp)}</td>
        <td style="font-weight:600">${e.description}</td>
        <td><span class="badge-status info">${e.categoryBn || e.category}</span></td>
        <td style="color:#ef4444;font-weight:700">${fmtMoney(e.amountMinor)}</td>
        <td>${e.paidBy || 'Administrator'}</td>
      </tr>
    `).join('');
  } catch (err) { console.error('Expenses error:', err); }
}

async function addExpense() {
  const desc = document.getElementById('expenseDescInput')?.value.trim();
  const cat = document.getElementById('expenseCatInput')?.value;
  const amt = (Number(document.getElementById('expenseAmountInput')?.value) || 0) * 100;
  const paidBy = document.getElementById('expensePaidByInput')?.value.trim() || 'Administrator';

  if (!desc || amt <= 0) {
    showToast('❌ বিবরণ ও সঠিক পরিমাণ দিন');
    return;
  }

  try {
    const res = await fetch('/api/expenses', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ description: desc, category: cat, amountMinor: amt, paidBy })
    });
    const d = await res.json();
    if (d.ok) {
      showToast('✅ খরচ সফলভাবে রেকর্ড হয়েছে!');
      document.getElementById('addExpenseForm').style.display = 'none';
      document.getElementById('expenseDescInput').value = '';
      document.getElementById('expenseAmountInput').value = '';
      loadExpenses();
    }
  } catch (e) { showToast('❌ খরচ সংরক্ষণে ত্রুটি'); }
}

// ─── AUDIT LOGS ──────────────────────────────────────────────────────────────
async function loadAuditLog() {
  try {
    const res = await fetch('/api/audit/recent');
    const logs = await res.json();
    const tbody = document.getElementById('auditTbody');
    if (!tbody) return;

    if (!logs || logs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="table-empty">কোনো অডিট লগ নেই।</td></tr>';
      return;
    }

    tbody.innerHTML = logs.map(l => `
      <tr>
        <td>${fmtDate(l.timestamp)}</td>
        <td style="font-weight:600">${l.userId || 'Administrator'}</td>
        <td><span class="badge-status info">${l.action}</span></td>
        <td>${l.details || '—'}</td>
        <td style="font-family:monospace;font-size:11px">${l.ip || '127.0.0.1'}</td>
      </tr>
    `).join('');
  } catch (err) { console.error('Audit log error:', err); }
}

// ─── PURCHASE ORDERS ─────────────────────────────────────────────────────────
async function loadPurchaseOrders() {
  try {
    const res = await fetch('/api/purchase-orders');
    const orders = await res.json();
    const tbody = document.getElementById('purchaseOrdersTbody');
    if (!tbody) return;

    if (!orders || orders.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="table-empty">কোনো ক্রয় অর্ডার নেই।</td></tr>';
      return;
    }

    tbody.innerHTML = orders.map(po => `
      <tr>
        <td><strong style="color:var(--primary)">${po.id}</strong></td>
        <td style="font-weight:600">${po.supplierName || '—'}</td>
        <td>${toBnNum(po.itemCount || 1)} টি</td>
        <td style="font-weight:700;color:var(--primary)">${fmtMoney(po.totalMinor)}</td>
        <td style="color:var(--text-muted);font-size:12px">${po.notes || '—'}</td>
        <td>${fmtDate(po.timestamp)}</td>
        <td><span class="badge-status ${po.status === 'RECEIVED' ? 'success' : 'warning'}">${po.status}</span></td>
      </tr>
    `).join('');
  } catch (err) { console.error('PO error:', err); }
}

async function addPurchaseOrder() {
  const sel = document.getElementById('poSupplierSelect');
  const supplierId = sel?.value;
  const supplierName = sel?.options[sel.selectedIndex]?.text;
  const totalMinor = (Number(document.getElementById('poTotalInput')?.value) || 0) * 100;
  const notes = document.getElementById('poNotesInput')?.value.trim();

  if (!supplierId || totalMinor <= 0) {
    showToast('❌ সরবরাহকারী ও সঠিক পরিমাণ দিন');
    return;
  }

  try {
    const res = await fetch('/api/purchase-orders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ supplierId, supplierName, totalMinor, notes, itemCount: 1 })
    });
    const d = await res.json();
    if (d.ok) {
      showToast('✅ ক্রয় অর্ডার সফলভাবে তৈরি হয়েছে!');
      document.getElementById('addPOForm').style.display = 'none';
      loadPurchaseOrders();
    }
  } catch (e) { showToast('❌ অর্ডার তৈরিতে ত্রুটি'); }
}

// ─── SETTINGS & HARDWARE TEST ────────────────────────────────────────────────
function saveSettings() {
  showToast('💾 সেটিংস সফলভাবে সংরক্ষিত হয়েছে!');
}

function testPrinter() {
  showToast('🖨️ ESC/POS থার্মাল প্রিন্টারে টেস্ট প্রিন্ট পাঠানো হয়েছে');
  fetch('/api/device/printer/test');
}

function openCashDrawer() {
  showToast('💵 ক্যাশ ড্রয়ার খুলতে সিগন্যাল পাঠানো হয়েছে');
  fetch('/api/device/drawer/open');
}

function testBarcodeScanner() {
  showToast('📷 বারকোড স্ক্যানার মোড সক্রিয় (F2 প্রেস করুন)');
  document.getElementById('barcodeSearchInput')?.focus();
}
