const avatarColors = ['#0F172A', '#334155', '#475569', '#1E293B'];

// Mirrors the share package — integer sen, round half-up per division.
function roundDiv(a, b) {
  if (b <= 0) return 0;
  return Math.floor((a + Math.floor(b / 2)) / b);
}

function formatRM(sen) {
  const sign = sen < 0 ? '-' : '';
  const abs = Math.abs(sen);
  return sign + 'RM ' + (abs / 100).toFixed(2);
}

function submitReceipt(input) {
  if (!input.files || !input.files.length) return;
  const other = input.id === 'receipt-camera' ? 'receipt-album' : 'receipt-camera';
  const el = document.getElementById(other);
  if (el) el.value = '';
  showScanOverlay();
  input.form.submit();
}

function markOnboardingSeen() {
  fetch('/onboarding/seen', { method: 'POST', credentials: 'same-origin' }).catch(() => {});
}

function openOnboardingModal(autoMarkSeenOnClose) {
  const modal = document.getElementById('onboarding-modal');
  if (!modal) return;
  modal.hidden = false;
  modal.setAttribute('aria-hidden', 'false');
  modal.dataset.autoMarkSeen = autoMarkSeenOnClose ? '1' : '0';
  document.body.style.overflow = 'hidden';
}

function closeOnboardingModal(explicitMarkSeen) {
  const modal = document.getElementById('onboarding-modal');
  if (!modal) return;
  modal.hidden = true;
  modal.setAttribute('aria-hidden', 'true');
  document.body.style.overflow = '';
  if (explicitMarkSeen || modal.dataset.autoMarkSeen === '1') {
    markOnboardingSeen();
  }
}

function initCaptureOnboarding() {
  const page = document.querySelector('.capture-page');
  if (!page || page.dataset.showOnboarding !== '1') return;
  openOnboardingModal(true);
}

function initCaptureScanError() {
  const banner = document.querySelector('.scan-error-banner');
  const overlay = document.getElementById('scan-overlay');
  if (banner && overlay) {
    overlay.hidden = true;
    overlay.setAttribute('aria-hidden', 'true');
  }
}

document.addEventListener('DOMContentLoaded', () => {
  initCaptureOnboarding();
  initCaptureScanError();
});
document.body.addEventListener('htmx:afterSettle', () => {
  initCaptureOnboarding();
  initCaptureScanError();
});

function showScanOverlay() {
  const overlay = document.getElementById('scan-overlay');
  if (!overlay) return;
  overlay.hidden = false;
  overlay.setAttribute('aria-hidden', 'false');
  const steps = overlay.querySelectorAll('.scan-step');
  let i = 0;
  const advance = () => {
    if (i > 0 && steps[i - 1]) {
      steps[i - 1].classList.remove('active');
      steps[i - 1].classList.add('done');
      steps[i - 1].textContent = '✓';
    }
    if (i < steps.length) {
      steps[i].classList.add('active');
      i++;
      setTimeout(advance, 700);
    }
  };
  advance();
}

function loadReceiptJSON() {
  const el = document.getElementById('receipt-data');
  if (!el || !el.textContent) return {};
  try {
    return JSON.parse(el.textContent);
  } catch (e) {
    console.error('Failed to parse receipt data', e);
    return {};
  }
}

function loadB64JSON(b64) {
  if (!b64) return {};
  try {
    return JSON.parse(atob(b64));
  } catch (e) {
    console.error('Failed to parse page data', e);
    return {};
  }
}

const warningMessages = {
  no_merchant: "We couldn't read the restaurant name — please fill it in.",
  no_items: 'No line items were found — add items manually.',
  no_total: "We couldn't read the total — please check the amount.",
  total_mismatch: "Items and tax don't add up to the total — please double-check SST, service charge, and rounding.",
  low_confidence_merchant: 'The merchant name may be wrong — please verify.',
  low_confidence_total: 'The total may be wrong — please verify against your receipt.',
  low_confidence_items: 'Some line items may be wrong — check highlighted rows.',
  low_confidence_item: 'Some line items may be wrong — check highlighted rows.',
  possible_duplicate: 'Some items look duplicated — remove any extras.',
};

function warningText(code) {
  return warningMessages[code] || 'Please review the details before sharing.';
}

function registerAlpineComponents(Alpine) {
  Alpine.data('qrCropper', (opts = {}) => ({
    hasExisting: !!opts.hasExisting,
    previewURL: '',
    fileName: '',
    naturalW: 0,
    naturalH: 0,
    baseScale: 1,
    scale: 1,
    panX: 0,
    panY: 0,
    dragging: false,
    lastX: 0,
    lastY: 0,
    submitting: false,
    get imgStyle() {
      const s = this.baseScale * this.scale;
      return {
        width: this.naturalW ? (this.naturalW + 'px') : 'auto',
        height: this.naturalH ? (this.naturalH + 'px') : 'auto',
        transform: `translate(calc(-50% + ${this.panX}px), calc(-50% + ${this.panY}px)) scale(${s})`,
      };
    },
    onFileChange(e) {
      const file = e.target.files?.[0];
      if (this.previewURL) URL.revokeObjectURL(this.previewURL);
      this.previewURL = '';
      this.fileName = '';
      this.scale = 1;
      this.panX = 0;
      this.panY = 0;
      if (!file) return;
      this.fileName = file.name;
      this.previewURL = URL.createObjectURL(file);
    },
    onImageLoad(e) {
      this.naturalW = e.target.naturalWidth;
      this.naturalH = e.target.naturalHeight;
      const stage = e.target.parentElement;
      const sw = stage.clientWidth;
      const sh = stage.clientHeight;
      // Cover the full stage so crop frame (inner 76%) can still be filled via zoom.
      this.baseScale = Math.max(sw / this.naturalW, sh / this.naturalH);
      this.scale = 1;
      this.panX = 0;
      this.panY = 0;
      this.clampPan();
    },
    stageSize() {
      const stage = this.$el.querySelector('.qr-crop-stage');
      if (!stage) return { w: 280, h: 280 };
      return { w: stage.clientWidth, h: stage.clientHeight };
    },
    clampPan() {
      if (!this.naturalW || !this.naturalH) return;
      const { w, h } = this.stageSize();
      const s = this.baseScale * this.scale;
      const dispW = this.naturalW * s;
      const dispH = this.naturalH * s;
      // Keep crop frame (inset 12%) covered with image.
      const frameW = w * 0.76;
      const frameH = h * 0.76;
      const maxX = Math.max(0, (dispW - frameW) / 2);
      const maxY = Math.max(0, (dispH - frameH) / 2);
      this.panX = Math.min(maxX, Math.max(-maxX, this.panX));
      this.panY = Math.min(maxY, Math.max(-maxY, this.panY));
    },
    startDrag(e) {
      if (!this.previewURL) return;
      this.dragging = true;
      this.lastX = e.clientX;
      this.lastY = e.clientY;
      e.currentTarget.setPointerCapture?.(e.pointerId);
    },
    onDrag(e) {
      if (!this.dragging) return;
      this.panX += e.clientX - this.lastX;
      this.panY += e.clientY - this.lastY;
      this.lastX = e.clientX;
      this.lastY = e.clientY;
      this.clampPan();
    },
    endDrag() { this.dragging = false; },
    onWheel(e) {
      if (!this.previewURL) return;
      const next = Math.min(4, Math.max(1, this.scale + (e.deltaY < 0 ? 0.08 : -0.08)));
      this.scale = Math.round(next * 100) / 100;
      this.clampPan();
    },
    async exportCroppedBlob() {
      const { w, h } = this.stageSize();
      const frame = 0.76;
      const out = 768;
      const canvas = document.createElement('canvas');
      canvas.width = out;
      canvas.height = out;
      const ctx = canvas.getContext('2d');
      ctx.fillStyle = '#fff';
      ctx.fillRect(0, 0, out, out);

      const s = this.baseScale * this.scale;
      // Image center in stage coords
      const imgCx = w / 2 + this.panX;
      const imgCy = h / 2 + this.panY;
      // Crop square in stage coords
      const cropX = w * (1 - frame) / 2;
      const cropY = h * (1 - frame) / 2;
      const cropS = w * frame;

      // Convert crop rect to image natural pixels
      const srcX = (cropX - (imgCx - (this.naturalW * s) / 2)) / s;
      const srcY = (cropY - (imgCy - (this.naturalH * s) / 2)) / s;
      const srcS = cropS / s;

      const img = this.$el.querySelector('.qr-crop-img');
      ctx.drawImage(img, srcX, srcY, srcS, srcS, 0, 0, out, out);

      return new Promise((resolve, reject) => {
        canvas.toBlob((blob) => {
          if (!blob) reject(new Error('crop failed'));
          else resolve(blob);
        }, 'image/jpeg', 0.92);
      });
    },
    async onSubmit(e) {
      if (!this.previewURL) return; // keep existing QR file / optional when hasExisting
      e.preventDefault();
      if (this.submitting) return;
      this.submitting = true;
      try {
        const blob = await this.exportCroppedBlob();
        const dt = new DataTransfer();
        dt.items.add(new File([blob], 'duitnow-qr.jpg', { type: 'image/jpeg' }));
        this.$refs.fileInput.files = dt.files;
        e.target.submit();
      } catch (err) {
        console.error(err);
        this.submitting = false;
        alert('Could not crop QR — try another photo');
      }
    },
  }));

  Alpine.data('receiptReview', () => ({
    merchant: '',
    subtotalSen: 0,
    sstSen: 0,
    serviceSen: 0,
    roundingSen: 0,
    totalSen: 0,
    items: [],
    warnings: [],
    init() {
      const data = loadReceiptJSON();
      this.merchant = data.merchant || '';
      this.subtotalSen = data.subtotalSen || 0;
      this.sstSen = data.sstSen || 0;
      this.serviceSen = data.serviceSen || 0;
      this.roundingSen = data.roundingSen || 0;
      this.totalSen = data.totalSen || 0;
      this.warnings = Array.isArray(data.warnings) ? [...data.warnings] : [];
      this.items = Array.isArray(data.items) ? data.items.map(it => ({
        ...it,
        includedInSplit: it.includedInSplit !== false,
      })) : [];
      if (this.items.length === 0) {
        this.items = [{ name: '', qty: 1, unitPriceSen: 0, lineTotalSen: 0, includedInSplit: true }];
      }
    },
    warningText,
    get splittableSubtotalSen() {
      return this.items.reduce((s, it) => s + (it.includedInSplit ? (parseInt(it.lineTotalSen, 10) || 0) : 0), 0);
    },
    get taxTotal() { return this.sstSen + this.serviceSen + this.roundingSen; },
    get computedTotal() {
      const itemsSum = this.items.reduce((s, it) => s + (parseInt(it.lineTotalSen, 10) || 0), 0);
      const base = this.subtotalSen > 0 ? this.subtotalSen : itemsSum;
      return base + this.taxTotal;
    },
    get hasTotalMismatch() {
      if (this.totalSen <= 0) return false;
      return Math.abs(this.computedTotal - this.totalSen) > 2;
    },
    reconcileText() {
      const computed = formatRM(this.computedTotal);
      const printed = formatRM(this.totalSen);
      return `Items + tax = ${computed} (receipt says ${printed})`;
    },
    addItem() {
      this.items.push({ name: '', qty: 1, unitPriceSen: 0, lineTotalSen: 0, includedInSplit: true });
    },
    removeItem(i) { this.items.splice(i, 1); },
    syncLine(i) {
      const it = this.items[i];
      const qty = Math.max(1, parseInt(it.qty, 10) || 1);
      const unit = parseInt(it.unitPriceSen, 10) || 0;
      it.qty = qty;
      it.lineTotalSen = unit * qty;
    },
  }));

  Alpine.data('friendPick', () => ({
    slug: '',
    me: '',
    split: null,
    items: [],
    taxTotalSen: 0,
    selected: new Set(),
    showBreakdown: false,
    owedSen: 0,
    init() {
      const cfg = loadB64JSON(this.$el.dataset.pickB64);
      this.slug = cfg.slug || '';
      this.me = cfg.me || '';
      this.split = cfg.split || null;
      this.items = cfg.items || [];
      this.taxTotalSen = cfg.taxTotalSen || 0;
      this.selected = new Set(cfg.selected || []);
      this.owedSen = cfg.owedSen || 0;
    },
    toggle(id) {
      if (this.selected.has(id)) this.selected.delete(id);
      else this.selected.add(id);
      if (!this.selected.size) this.showBreakdown = false;
      this.saveClaims();
    },
    isSelected(id) { return this.selected.has(id); },
    others(it) { return (it.claimedBy || []).filter(n => n !== this.me); },
    splitWays(it) {
      const n = this.others(it).length + (this.isSelected(it.id) ? 1 : 0);
      return 'split ' + n + ' way' + (n === 1 ? '' : 's');
    },
    initial(name) {
      const n = (name || '').trim();
      return n ? n[0].toUpperCase() : '?';
    },
    avatarColor(name) {
      let h = 0;
      for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) | 0;
      return avatarColors[((h % avatarColors.length) + avatarColors.length) % avatarColors.length];
    },
    estimate() {
      const lines = [];
      let claimed = 0;
      for (const it of this.items) {
        if (!this.selected.has(it.id)) continue;
        const others = (it.claimedBy || []).filter(n => n !== this.me);
        const claimants = Math.max(others.length + 1, 1);
        const portion = roundDiv(it.lineTotalSen, claimants);
        let name = it.name;
        if (claimants > 1) name = it.name + ' · shared ÷' + claimants;
        lines.push({ name, amtSen: portion });
        claimed += portion;
      }
      const sub = this.split?.splittableSubtotalSen ?? this.split?.subtotalSen ?? 0;
      const tax = this.taxTotalSen || 0;
      const owed = sub <= 0 ? claimed : claimed + roundDiv(claimed * tax, sub);
      return { lines, taxSen: owed - claimed, owedSen: owed, claimedSen: claimed };
    },
    get displayOwed() { return formatRM(this.estimate().owedSen); },
    async saveClaims() {
      const ids = [...this.selected];
      try {
        const r = await fetch('/r/' + this.slug + '/claims', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ itemIds: ids }),
        });
        if (!r.ok) throw new Error(await r.text());
        const d = await fetch('/r/' + this.slug + '/pick-data').then(x => x.json());
        this.items = d.items;
        this.owedSen = d.owedSen;
        if (d.splittableSubtotalSen != null && this.split) {
          this.split.splittableSubtotalSen = d.splittableSubtotalSen;
        }
      } catch (e) {
        console.error(e);
      }
    },
  }));
}

// Register whether or not Alpine has started yet. If app.js loads before Alpine
// (the intended order), the alpine:init listener fires. If Alpine is already
// present, register immediately so components are never missing.
if (window.Alpine) {
  registerAlpineComponents(window.Alpine);
} else {
  document.addEventListener('alpine:init', () => registerAlpineComponents(window.Alpine));
}

function copyShareLink(url, btn) {
  navigator.clipboard.writeText(url).then(() => {
    const el = document.getElementById('copy-msg');
    if (el) { el.textContent = 'Copied!'; setTimeout(() => { el.textContent = 'Copy link'; }, 2000); }
    if (btn) {
      btn.classList.add('is-copied');
      const label = btn.getAttribute('aria-label') || 'Share link';
      btn.setAttribute('aria-label', 'Copied!');
      setTimeout(() => {
        btn.classList.remove('is-copied');
        btn.setAttribute('aria-label', label);
      }, 2000);
    }
  });
}

function shareNative(url, text, btn) {
  if (navigator.share) {
    navigator.share({ title: 'Pisah', text, url }).catch(() => copyShareLink(url, btn));
  } else {
    copyShareLink(url, btn);
  }
}

function initAlpineOnHtmx(e) {
  if (window.Alpine && e.detail.target) {
    Alpine.initTree(e.detail.target);
  }
}

document.body.addEventListener('htmx:afterSwap', initAlpineOnHtmx);
document.body.addEventListener('htmx:afterSettle', initAlpineOnHtmx);

function readSummaryTotals() {
  const strip = document.getElementById('summary-strip');
  if (!strip) return null;
  return {
    outstandingSen: parseInt(strip.dataset.outstandingSen, 10) || 0,
    collectedSen: parseInt(strip.dataset.collectedSen, 10) || 0,
    activeCount: parseInt(strip.dataset.activeCount, 10) || 0,
  };
}

function writeSummaryTotals(outstandingSen, collectedSen, activeCount) {
  const strip = document.getElementById('summary-strip');
  if (!strip) return;
  strip.dataset.outstandingSen = outstandingSen;
  strip.dataset.collectedSen = collectedSen;
  strip.dataset.activeCount = activeCount;
  const outEl = document.getElementById('summary-outstanding');
  const colEl = document.getElementById('summary-collected');
  const actEl = document.getElementById('summary-active');
  if (outEl) outEl.textContent = formatRM(outstandingSen);
  if (colEl) colEl.textContent = formatRM(collectedSen);
  if (actEl) actEl.textContent = activeCount;
}

function hideCaptureSplitsSection() {
  const section = document.getElementById('capture-splits');
  if (section) section.hidden = true;
}

function applySummaryAfterSplitDelete(row) {
  const totalSen = parseInt(row.dataset.totalSen, 10) || 0;
  const collectedSen = parseInt(row.dataset.collectedSen, 10) || 0;
  const remainingSen = totalSen - collectedSen;
  const totals = readSummaryTotals();
  if (!totals) return false;

  let { outstandingSen, collectedSen: collectedTotal, activeCount } = totals;
  collectedTotal -= collectedSen;
  if (remainingSen > 0) {
    outstandingSen -= remainingSen;
    activeCount -= 1;
  }
  writeSummaryTotals(outstandingSen, collectedTotal, activeCount);
  return row.closest('#splits-list')?.querySelectorAll('.split-row').length === 1;
}

document.body.addEventListener('htmx:beforeSwap', (e) => {
  const row = e.detail.target;
  if (!row?.classList?.contains('split-row')) return;
  const req = e.detail.requestConfig;
  if (!req || req.verb !== 'delete') return;
  if (e.detail.xhr.status !== 200) return;

  const lastSplit = applySummaryAfterSplitDelete(row);
  if (lastSplit) {
    document.body.addEventListener('htmx:afterSwap', hideCaptureSplitsSection, { once: true });
  }
});
