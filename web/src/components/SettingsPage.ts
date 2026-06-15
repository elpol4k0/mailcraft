import { state, showToast } from '../state';
import { el } from '../utils';
import { icon } from '../icons';
import { confirm } from '../dialog';
import { patchConfig } from '../api';

interface ConfigResponse {
  smtp_addr: string;
  http_addr: string;
  max_emails: number;
  base_path: string;
  log_level: string;
}

interface HealthResponse {
  status: string;
  version: string;
  uptime_s: number;
}

interface StatsResponse {
  total: number;
  unread: number;
  starred: number;
  size_bytes: number;
  rules_count: number;
}

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  return `${h}h ${m}m ${s}s`;
}

export function createSettingsPage(): HTMLElement {
  const page = el('div', 'settings-page');
  page.style.display = 'none';

  const header = el('div', 'rules-page-header');
  const title = el('h2', '');
  title.innerHTML = `${icon('settings', 16)} Settings`;
  header.appendChild(title);
  page.appendChild(header);

  const content = el('div', '');
  content.style.display = 'flex';
  content.style.flexDirection = 'column';
  content.style.gap = '16px';
  page.appendChild(content);

  const serverCard = el('div', 'settings-card');
  const serverTitle = el('div', 'settings-card-title', 'Server Info');
  const serverRows = el('div', '');

  const smtpRow = buildRow('SMTP Address', '...');
  const httpRow = buildRow('HTTP Address', '...');
  const versionRow = buildRow('Version', '...');
  const uptimeRow = buildRow('Uptime', '...');

  serverRows.append(smtpRow.el, httpRow.el, versionRow.el, uptimeRow.el);
  serverCard.append(serverTitle, serverRows);

  const storageCard = el('div', 'settings-card');
  const storageTitle = el('div', 'settings-card-title', 'Storage');

  const emailCountRow = buildRow('Current Email Count', '...');
  const sizeRow = buildRow('Current Size', '...');

  const storageRows = el('div', '');
  storageRows.append(emailCountRow.el, sizeRow.el);
  storageCard.append(storageTitle, storageRows);

  const runtimeCard = el('div', 'settings-card');
  const runtimeTitle = el('div', 'settings-card-title', 'Runtime Configuration');

  const maxEmailsInput = el('input', 'settings-input') as HTMLInputElement;
  maxEmailsInput.type = 'number';
  maxEmailsInput.min = '1';
  maxEmailsInput.style.width = '100px';

  const logLevelSelect = el('select', 'settings-input') as HTMLSelectElement;
  ['debug', 'info', 'warn', 'error'].forEach(level => {
    const opt = document.createElement('option');
    opt.value = level;
    opt.textContent = level;
    logLevelSelect.appendChild(opt);
  });

  const maxEmailsRow = buildEditRow('Max Emails', maxEmailsInput);
  const logLevelRow = buildEditRow('Log Level', logLevelSelect);

  const saveBtn = el('button', 'btn btn-primary btn-sm', 'Save');
  saveBtn.style.marginTop = '12px';
  saveBtn.addEventListener('click', async () => {
    const maxEmails = parseInt(maxEmailsInput.value, 10);
    const logLevel = logLevelSelect.value;
    if (isNaN(maxEmails) || maxEmails < 1) {
      showToast('Max emails must be >= 1', 'error');
      return;
    }
    try {
      saveBtn.disabled = true;
      const updated = await patchConfig({ log_level: logLevel, max_emails: maxEmails });
      maxEmailsInput.value = String(updated.max_emails);
      logLevelSelect.value = updated.log_level;
      showToast('Settings saved', 'success');
    } catch {
      showToast('Failed to save settings', 'error');
    } finally {
      saveBtn.disabled = false;
    }
  });

  const runtimeRows = el('div', '');
  runtimeRows.append(maxEmailsRow, logLevelRow);
  runtimeCard.append(runtimeTitle, runtimeRows, saveBtn);

  const dangerCard = el('div', 'settings-card settings-danger');
  const dangerTitle = el('div', 'settings-card-title', 'Danger Zone');
  const dangerDesc = el('div', '');
  dangerDesc.style.cssText = 'font-size:13px;color:var(--text-secondary);margin-bottom:12px;';
  dangerDesc.textContent = 'Irreversible and destructive actions.';

  const deleteAllBtn = el('button', 'btn btn-danger', 'Delete all emails');
  deleteAllBtn.addEventListener('click', async () => {
    if (!await confirm('Delete ALL emails? This cannot be undone.', true)) return;
    try {
      const res = await fetch('/api/v1/emails', { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({}) });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      state.selectedEmailId.set(null);
      state.emails.set([]);
      state.total.set(0);
      document.dispatchEvent(new CustomEvent('mailcraft:refresh-list'));
      showToast('All emails deleted', 'success');
      await loadData();
    } catch {
      showToast('Failed to delete emails', 'error');
    }
  });

  dangerCard.append(dangerTitle, dangerDesc, deleteAllBtn);

  content.append(serverCard, storageCard, runtimeCard, dangerCard);

  async function loadData() {
    try {
      const [configRes, statsRes, healthRes] = await Promise.all([
        fetch('/api/v1/config').then(r => r.json() as Promise<ConfigResponse>),
        fetch('/api/v1/stats').then(r => r.json() as Promise<StatsResponse>),
        fetch('/api/v1/health').then(r => r.json() as Promise<HealthResponse>),
      ]);

      smtpRow.setValue(formatAddr(configRes.smtp_addr));
      httpRow.setValue(formatAddr(configRes.http_addr));
      versionRow.setValue(healthRes.version ? `v${healthRes.version}` : '-');
      uptimeRow.setValue(typeof healthRes.uptime_s === 'number' ? formatUptime(healthRes.uptime_s) : '-');

      emailCountRow.querySelector('.settings-value')!.textContent = String(statsRes.total ?? '-');
      const sizeMB = statsRes.size_bytes ? (statsRes.size_bytes / (1024 * 1024)).toFixed(2) + ' MB' : '0 MB';
      sizeRow.setValue(sizeMB);

      maxEmailsInput.value = String(configRes.max_emails ?? 5000);
      logLevelSelect.value = configRes.log_level ?? 'info';
    } catch (e) {
      console.error('Failed to load settings data', e);
    }
  }

  state.view.subscribe(view => {
    if (view === 'settings') {
      loadData();
    }
  });

  return page;
}

function formatAddr(addr: string): string {
  if (!addr) return '-';
  if (addr.startsWith(':')) return `0.0.0.0${addr}`;
  return addr;
}

function buildRow(label: string, initialValue: string): { el: HTMLElement; setValue: (v: string) => void } {
  const row = el('div', 'settings-row');
  const labelEl = el('div', 'settings-label', label);
  const valueEl = el('div', 'settings-value', initialValue);
  row.append(labelEl, valueEl);
  return {
    el: row,
    setValue: (v: string) => { valueEl.textContent = v; },
  };
}

function buildEditRow(label: string, inputEl: HTMLElement): HTMLElement {
  const row = el('div', 'settings-row');
  const labelEl = el('div', 'settings-label', label);
  row.append(labelEl, inputEl);
  return row;
}
