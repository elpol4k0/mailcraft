import './styles/main.css';

import { state, showToast, toasts, prependEmail, updateEmailInList, removeEmailFromList } from './state';
import type { ToastType } from './state';
import { listTags, listFolders, listMailboxes, getStats, listEmails } from './api';
import type { Email } from './api';
import { debounce, parseHash, setHash, el } from './utils';
import { icon } from './icons';
import { requestNotificationPermission, notifyNewEmail } from './notifications';
import { applyHashState, getActiveFilters } from './state';
import { createSidebar } from './components/Sidebar';
import { createEmailList } from './components/EmailList';
import { createEmailDetail } from './components/EmailDetail';
import { createTagFilter } from './components/TagFilter';
import { createRulesPage } from './components/RuleBuilder';
import { createSettingsPage } from './components/SettingsPage';

function boot() {
  const appEl = document.getElementById('app');
  if (!appEl) return;

  const topbar = el('header', 'topbar');
  const brand = document.createElement('a');
  brand.className = 'topbar-brand';
  brand.href = '#';
  brand.innerHTML = `<div class="topbar-brand-icon">${icon('mail', 15)}</div><span>MailCraft</span>`;
  brand.addEventListener('click', (e) => {
    e.preventDefault();
    state.view.set('inbox');
    state.filterTag.set(null);
    state.filterRead.set(null);
    state.filterStarred.set(null);
    state.search.set('');
  });

  const searchWrapper = el('div', 'topbar-search');
  searchWrapper.innerHTML = `<svg class="topbar-search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>`;
  const searchInput = el('input', '');
  searchInput.type = 'search';
  searchInput.id = 'global-search';
  searchInput.placeholder = 'Search emails…';
  searchInput.setAttribute('aria-label', 'Search emails');
  searchWrapper.appendChild(searchInput);

  const topRight = el('div', 'topbar-right');
  const versionEl = el('span', 'topbar-version', 'v1.0');
  versionEl.id = 'app-version';

  const settingsBtn = el('button', 'btn btn-ghost btn-sm');
  settingsBtn.innerHTML = `${icon('settings', 13)} Settings`;
  settingsBtn.addEventListener('click', () => state.view.set('settings'));

  topRight.append(versionEl, settingsBtn);
  topbar.append(brand, searchWrapper, topRight);
  appEl.appendChild(topbar);

  const mainLayout = el('div', 'main-layout');
  appEl.appendChild(mainLayout);

  const sidebar = createSidebar();
  mainLayout.appendChild(sidebar);

  const centerPanel = el('div', '');
  centerPanel.style.display = 'flex';
  centerPanel.style.flexDirection = 'column';
  centerPanel.style.overflow = 'hidden';
  centerPanel.style.borderRight = '1px solid var(--border)';

  const filterBar = createTagFilter();
  centerPanel.appendChild(filterBar);

  const emailList = createEmailList();
  centerPanel.appendChild(emailList);

  mainLayout.appendChild(centerPanel);

  const detailArea = el('div', '');
  detailArea.style.flex = '1';
  detailArea.style.display = 'flex';
  detailArea.style.overflow = 'hidden';

  const emailDetail = createEmailDetail();
  detailArea.appendChild(emailDetail);

  const rulesPage = createRulesPage();
  rulesPage.style.display = 'none';
  detailArea.appendChild(rulesPage);

  const settingsPage = createSettingsPage();
  settingsPage.style.display = 'none';
  detailArea.appendChild(settingsPage);

  mainLayout.appendChild(detailArea);

  const toastContainer = el('div', 'toast-container');
  toastContainer.setAttribute('aria-live', 'polite');
  document.body.appendChild(toastContainer);

  toasts.subscribe(items => {
    toastContainer.innerHTML = '';
    items.forEach(toast => {
      const t = el('div', `toast ${toast.type}`);
      const icons: Record<ToastType, string> = {
        success: icon('check-circle', 15),
        error: icon('x-circle', 15),
        info: icon('info', 15),
      };
      t.innerHTML = `<span class="toast-icon">${icons[toast.type]}</span><span>${toast.message}</span>`;
      toastContainer.appendChild(t);
    });
  });

  state.view.subscribe(view => {
    const showRules = view === 'rules';
    const showSettings = view === 'settings';
    const showNormal = !showRules && !showSettings;
    centerPanel.style.display = showNormal ? 'flex' : 'none';
    emailDetail.style.display = showNormal ? '' : 'none';
    rulesPage.style.display = showRules ? '' : 'none';
    settingsPage.style.display = showSettings ? '' : 'none';
    updateHash();
  });

  const doSearch = debounce((val: unknown) => {
    state.search.set(val as string);
  }, 200);

  searchInput.addEventListener('input', () => doSearch(searchInput.value));

  state.search.subscribe(q => {
    if (searchInput.value !== q) searchInput.value = q;
    updateHash();
  });

  function updateHash() {
    setHash(getActiveFilters());
  }

  state.filterTag.subscribe(updateHash);
  state.filterRead.subscribe(updateHash);
  state.filterStarred.subscribe(updateHash);
  state.selectedEmailId.subscribe(updateHash);

  const hash = parseHash();
  if (hash.view || hash.q || hash.tag || hash.emailId) {
    applyHashState(hash);
    if (hash.q) searchInput.value = hash.q;
  }

  window.addEventListener('popstate', () => {
    const h = parseHash();
    applyHashState(h);
    if (h.q) searchInput.value = h.q || '';
  });

  loadInitialData();
  setupSSE();
  fetchVersion();
  requestNotificationPermission();
}

async function loadInitialData() {
  try {
    const [statsRes, tagsRes, foldersRes, mailboxesRes] = await Promise.all([getStats(), listTags(), listFolders(), listMailboxes()]);
    state.stats.set(statsRes);
    state.tags.set(tagsRes);
    state.folders.set(foldersRes);
    state.mailboxes.set(mailboxesRes);
  } catch (e) {
    console.error('Failed to load initial data', e);
  }
}

async function fetchVersion() {
  try {
    const health = await fetch('/api/v1/health').then(r => r.json());
    const el = document.getElementById('app-version');
    if (el && health.version) el.textContent = `v${health.version}`;
  } catch {}
}

let sseRetryTimer: ReturnType<typeof setTimeout> | null = null;
let sseBackoff = 1000;

function setupSSE() {
  if (sseRetryTimer) clearTimeout(sseRetryTimer);

  const es = new EventSource('/api/v1/events');

  es.onopen = () => {
    state.sseConnected.set(true);
    sseBackoff = 1000;
  };

  es.onerror = () => {
    state.sseConnected.set(false);
    es.close();
    sseRetryTimer = setTimeout(() => {
      setupSSE();
    }, Math.min(sseBackoff, 30000));
    sseBackoff = Math.min(sseBackoff * 2, 30000);
  };

  es.addEventListener('email.new', (e: MessageEvent) => {
    try {
      const event = JSON.parse(e.data);
      const email = event.payload as Email;
      if (email) {
        prependEmail(email);
        refreshStats();
        refreshTags();
        refreshFolders();
        refreshMailboxes();
        notifyNewEmail(email.from, email.subject);
        showToast(`New email from ${email.from.replace(/<[^>]+>/, '').trim() || email.from}`, 'info');
      }
    } catch {}
  });

  es.addEventListener('email.updated', (e: MessageEvent) => {
    try {
      const event = JSON.parse(e.data);
      const email = event.payload as Email;
      if (email) {
        updateEmailInList(email);
        refreshStats();
      }
    } catch {}
  });

  es.addEventListener('email.deleted', (e: MessageEvent) => {
    try {
      const event = JSON.parse(e.data);
      const payload = event.payload;
      if (payload?.id) {
        removeEmailFromList(payload.id);
      } else if (payload?.ids && Array.isArray(payload.ids)) {
        payload.ids.forEach((id: string) => removeEmailFromList(id));
      }
      refreshStats();
    } catch {}
  });

  es.addEventListener('stats.updated', (e: MessageEvent) => {
    try {
      const event = JSON.parse(e.data);
      if (event.payload) state.stats.set(event.payload);
    } catch {}
  });

  es.addEventListener('folders.updated', () => {
    refreshFolders();
  });
}

async function refreshStats() {
  try {
    const stats = await getStats();
    state.stats.set(stats);
  } catch {}
}

async function refreshTags() {
  try {
    const tags = await listTags();
    state.tags.set(tags);
  } catch {}
}

async function refreshFolders() {
  try {
    const folders = await listFolders();
    state.folders.set(folders);
  } catch {}
}

async function refreshMailboxes() {
  try {
    const mailboxes = await listMailboxes();
    state.mailboxes.set(mailboxes);
  } catch {}
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', boot);
} else {
  boot();
}
