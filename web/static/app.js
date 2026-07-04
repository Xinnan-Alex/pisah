// Mirrors web.go avatarColor palette so co-claimant avatars are stable.
const avatarColors = ['#18A07A', '#7C5CFF', '#E8A02D', '#F25C3B'];

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
    fetch('/onboarding/seen', { method: 'POST', credentials: 'same-origin' }).catch(() => {});
  }
}

function initCaptureOnboarding() {
  const page = document.querySelector('.capture-page');
  if (!page || page.dataset.showOnboarding !== '1') return;
  openOnboardingModal(true);
}

document.addEventListener('DOMContentLoaded', initCaptureOnboarding);
document.body.addEventListener('htmx:afterSettle', initCaptureOnboarding);

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

function registerAlpineComponents(Alpine) {
  Alpine.data('receiptReview', () => ({
    merchant: '',
    subtotalSen: 0,
    sstSen: 0,
    serviceSen: 0,
    roundingSen: 0,
    totalSen: 0,
    items: [],
    init() {
      const data = loadReceiptJSON();
      this.merchant = data.merchant || '';
      this.subtotalSen = data.subtotalSen || 0;
      this.sstSen = data.sstSen || 0;
      this.serviceSen = data.serviceSen || 0;
      this.roundingSen = data.roundingSen || 0;
      this.totalSen = data.totalSen || 0;
      this.items = Array.isArray(data.items) ? data.items.map(it => ({
        ...it,
        includedInSplit: it.includedInSplit !== false,
      })) : [];
    },
    get splittableSubtotalSen() {
      return this.items.reduce((s, it) => s + (it.includedInSplit ? (parseInt(it.lineTotalSen, 10) || 0) : 0), 0);
    },
    get taxTotal() { return this.sstSen + this.serviceSen + this.roundingSen; },
    get computedTotal() {
      const itemsSum = this.items.reduce((s, it) => s + (parseInt(it.lineTotalSen, 10) || 0), 0);
      if (this.subtotalSen > 0) return this.subtotalSen + this.taxTotal;
      return itemsSum + this.taxTotal;
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

function copyShareLink(url) {
  navigator.clipboard.writeText(url).then(() => {
    const el = document.getElementById('copy-msg');
    if (el) { el.textContent = 'Copied!'; setTimeout(() => { el.textContent = 'Copy link'; }, 2000); }
  });
}

function shareNative(url, text) {
  if (navigator.share) {
    navigator.share({ title: 'Pisah', text, url }).catch(() => copyShareLink(url));
  } else {
    copyShareLink(url);
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
  document.querySelector('.viewfinder')?.classList.remove('viewfinder-compact');
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
