import { state, updateEmailInList, removeEmailFromList, showToast } from '../state';
import type { Email, Attachment, LinkResult, LinkCheckResponse, HTMLCheckResult, HTMLCheckWarning, SpamCheckResult, SpamCheckItem } from '../api';
import { getEmail, getEmailRaw, deleteEmail, patchEmail, addTag, removeTag, listTags, checkLinks, checkHTML, checkSpam, listEmails, exportEmailURL, previewAttachmentURL } from '../api';
import {
  relativeTime, formatDate, avatarColor, getInitials, senderName,
  attachmentIcon, formatSize, escapeHtml, el
} from '../utils';
import { icon } from '../icons';
import { confirm } from '../dialog';

type DetailTab = 'html' | 'text' | 'raw' | 'html-source' | 'headers' | 'links' | 'htmlcheck' | 'spamcheck' | 'smtp-log';

export function createEmailDetail(): HTMLElement {
  const pane = el('div', 'email-detail-pane');

  const emptyEl = el('div', 'email-detail-empty');
  emptyEl.innerHTML = `
    <div class="email-detail-empty-icon" style="color:var(--text-muted)">${icon('mail', 40)}</div>
    <div style="font-size:16px;color:var(--text-secondary)">Select an email to read</div>
    <div style="font-size:13px;color:var(--text-muted)">Use ↑/↓ to navigate, Del to delete</div>
  `;
  pane.appendChild(emptyEl);

  const detailEl = el('div', '');
  detailEl.id = 'email-detail-content';
  detailEl.style.display = 'none';
  detailEl.style.flex = '1';
  detailEl.style.display = 'none';
  detailEl.style.flexDirection = 'column';
  detailEl.style.overflow = 'hidden';
  pane.appendChild(detailEl);

  let currentEmail: Email | null = null;
  let currentTab: DetailTab = 'html';
  let rawContent = '';
  let linkCheckRunning = false;
  let htmlCheckRunning = false;
  let spamCheckRunning = false;
  let smtpLogLoaded = false;

  const actionBar = el('div', 'email-detail-actions');
  const starBtn = el('button', 'btn btn-ghost btn-sm');
  starBtn.innerHTML = `${icon('star', 13)} Star`;
  starBtn.title = 'Toggle star';
  const readBtn = el('button', 'btn btn-ghost btn-sm');
  readBtn.innerHTML = `${icon('eye', 13)} Read`;
  readBtn.title = 'Mark as read';
  const unreadBtn = el('button', 'btn btn-ghost btn-sm');
  unreadBtn.innerHTML = `${icon('eye-off', 13)} Unread`;
  unreadBtn.title = 'Mark as unread';
  const compareBtn = el('button', 'btn btn-ghost btn-sm');
  compareBtn.innerHTML = `${icon('arrows-left-right', 13)} Compare`;
  compareBtn.title = 'Compare with another email';
  const exportBtn = el('button', 'btn btn-ghost btn-sm');
  exportBtn.innerHTML = `${icon('download', 13)} Export`;
  exportBtn.title = 'Download as .eml file';
  const spacer = el('span', '');
  spacer.style.flex = '1';
  const deleteBtn = el('button', 'btn btn-danger btn-sm');
  deleteBtn.innerHTML = `${icon('trash-2', 13)} Delete`;
  deleteBtn.title = 'Delete';
  actionBar.append(starBtn, readBtn, unreadBtn, compareBtn, exportBtn, spacer, deleteBtn);
  detailEl.appendChild(actionBar);

  const headerEl = el('div', 'email-detail-header');
  detailEl.appendChild(headerEl);

  const tabBar = el('div', 'email-detail-tabs');
  const tabs: Array<{ id: DetailTab; label: string }> = [
    { id: 'html', label: 'HTML Preview' },
    { id: 'text', label: 'Plaintext' },
    { id: 'raw', label: 'Raw Source' },
    { id: 'html-source', label: 'HTML Source' },
    { id: 'headers', label: 'Headers' },
    { id: 'links', label: 'Link Check' },
    { id: 'htmlcheck', label: 'HTML Check' },
    { id: 'spamcheck', label: 'Content Check' },
    { id: 'smtp-log', label: 'SMTP Log' },
  ];
  const tabBtns: Map<DetailTab, HTMLButtonElement> = new Map();
  tabs.forEach(t => {
    const btn = el('button', 'email-detail-tab', t.label);
    btn.addEventListener('click', () => switchTab(t.id));
    tabBtns.set(t.id, btn);
    tabBar.appendChild(btn);
  });
  detailEl.appendChild(tabBar);

  const bodyEl = el('div', 'email-detail-body');

  const htmlPanel = el('div', 'email-detail-body-panel');
  htmlPanel.dataset.tab = 'html';
  const iframe = document.createElement('iframe');
  iframe.sandbox.add('allow-same-origin');
  iframe.title = 'Email HTML preview';
  htmlPanel.appendChild(iframe);

  const textPanel = el('div', 'email-detail-body-panel');
  textPanel.dataset.tab = 'text';
  const textContent = el('pre', 'email-detail-plaintext');
  textPanel.appendChild(textContent);

  const rawPanel = el('div', 'email-detail-body-panel');
  rawPanel.dataset.tab = 'raw';
  const rawContent_ = el('pre', 'email-detail-raw');
  rawPanel.appendChild(rawContent_);

  const htmlSourcePanel = el('div', 'email-detail-body-panel');
  htmlSourcePanel.dataset.tab = 'html-source';
  const htmlSourcePre = el('pre', 'email-html-source');
  htmlSourcePanel.appendChild(htmlSourcePre);

  const headersPanel = el('div', 'email-detail-body-panel');
  headersPanel.dataset.tab = 'headers';
  const headersTable = document.createElement('table');
  headersTable.className = 'email-headers-table';
  headersPanel.appendChild(headersTable);

  const linksPanel = el('div', 'email-detail-body-panel');
  linksPanel.dataset.tab = 'links';
  linksPanel.innerHTML = buildLinkCheckPanelHTML();
  const linkCheckFollowToggle = linksPanel.querySelector('#lc-follow-redirects') as HTMLInputElement;
  const linkCheckAutoToggle = linksPanel.querySelector('#lc-auto-check') as HTMLInputElement;
  const linkCheckRunBtn = linksPanel.querySelector('#lc-run-btn') as HTMLButtonElement;
  const linkCheckResults = linksPanel.querySelector('#lc-results') as HTMLElement;

  linkCheckFollowToggle.checked = localStorage.getItem('mc_follow_redirects') === 'true';
  linkCheckAutoToggle.checked = localStorage.getItem('mc_auto_linkcheck') === 'true';

  linkCheckFollowToggle.addEventListener('change', () => {
    localStorage.setItem('mc_follow_redirects', String(linkCheckFollowToggle.checked));
  });
  linkCheckAutoToggle.addEventListener('change', () => {
    localStorage.setItem('mc_auto_linkcheck', String(linkCheckAutoToggle.checked));
  });
  linkCheckRunBtn.addEventListener('click', () => {
    if (currentEmail && !linkCheckRunning) runLinkCheck(currentEmail.id);
  });

  const htmlCheckPanel = el('div', 'email-detail-body-panel');
  htmlCheckPanel.dataset.tab = 'htmlcheck';
  htmlCheckPanel.innerHTML = buildHTMLCheckPanelHTML();
  const htmlCheckRunBtn = htmlCheckPanel.querySelector('#hc-run-btn') as HTMLButtonElement;
  const htmlCheckResults = htmlCheckPanel.querySelector('#hc-results') as HTMLElement;

  htmlCheckRunBtn.addEventListener('click', () => {
    if (currentEmail && !htmlCheckRunning) runHTMLCheck(currentEmail.id);
  });

  const spamCheckPanel = el('div', 'email-detail-body-panel');
  spamCheckPanel.dataset.tab = 'spamcheck';

  const spamRunBtn = el('button', 'btn btn-primary btn-sm') as HTMLButtonElement;
  spamRunBtn.innerHTML = `${icon('check-circle', 13)} Analyze Content`;
  const spamResults = el('div', '');

  const spamInner = el('div', '');
  spamInner.style.cssText = 'padding:16px;display:flex;flex-direction:column;gap:12px;';
  const spamBtnRow = el('div', '');
  spamBtnRow.style.cssText = 'display:flex;align-items:center;gap:12px;';
  spamBtnRow.appendChild(spamRunBtn);
  spamInner.append(spamBtnRow, spamResults);
  spamCheckPanel.appendChild(spamInner);

  spamRunBtn.addEventListener('click', () => {
    if (currentEmail && !spamCheckRunning) runSpamCheck(currentEmail.id);
  });

  const smtpLogPanel = el('div', 'email-detail-body-panel');
  smtpLogPanel.dataset.tab = 'smtp-log';
  const smtpLogPre = el('pre', 'smtp-log-pre');
  smtpLogPanel.appendChild(smtpLogPre);

  bodyEl.append(htmlPanel, textPanel, rawPanel, htmlSourcePanel, headersPanel, linksPanel, htmlCheckPanel, spamCheckPanel, smtpLogPanel);
  detailEl.appendChild(bodyEl);

  function buildHeader(email: Email) {
    headerEl.innerHTML = '';

    const subject = el('div', 'email-detail-subject', email.subject || '(no subject)');
    headerEl.appendChild(subject);

    const fromRow = el('div', 'email-detail-from-row');
    const color = avatarColor(email.from);
    const initials = getInitials(email.from);
    const avatar = el('div', 'email-item-avatar');
    avatar.style.background = color;
    avatar.textContent = initials;
    const fromInfo = el('div', '');
    fromInfo.innerHTML = `
      <div style="font-weight:600;color:var(--text-primary)">${escapeHtml(senderName(email.from))}</div>
      <div style="font-size:12px;color:var(--text-muted)">${escapeHtml(email.from)}</div>
    `;
    fromRow.append(avatar, fromInfo);
    headerEl.appendChild(fromRow);

    const meta = el('div', 'email-detail-meta');
    const addMeta = (label: string, value: string) => {
      if (!value) return;
      const l = el('div', 'email-detail-meta-label', label);
      const v = el('div', 'email-detail-meta-value');
      v.innerHTML = escapeHtml(value);
      meta.append(l, v);
    };

    addMeta('To', (email.to || []).join(', '));
    if (email.cc && email.cc.length > 0) addMeta('CC', email.cc.join(', '));
    if (email.bcc && email.bcc.length > 0) addMeta('BCC', email.bcc.join(', '));
    addMeta('Date', formatDate(email.received_at));
    headerEl.appendChild(meta);

    const tagsRow = el('div', 'email-detail-tags');
    renderTagChips(email, tagsRow);
    headerEl.appendChild(tagsRow);

    if (email.attachments && email.attachments.length > 0) {
      const attRow = el('div', 'email-detail-attachments');
      email.attachments.forEach(att => {
        const chip = buildAttachmentChip(email.id, att);
        attRow.appendChild(chip);
      });
      headerEl.appendChild(attRow);
    }
  }

  function renderTagChips(email: Email, container: HTMLElement) {
    container.innerHTML = '';
    (email.tags || []).forEach(tag => {
      const chip = el('span', 'tag-chip removable interactive');
      const name = el('span', '', '#' + tag);
      const rmBtn = el('button', 'tag-chip-remove', '×');
      rmBtn.title = `Remove tag "${tag}"`;
      rmBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        try {
          const updated = await removeTag(email.id, tag);
          updateEmailInList(updated);
          currentEmail = updated;
          renderTagChips(updated, container);
        } catch {
          showToast('Failed to remove tag', 'error');
        }
      });
      chip.append(name, rmBtn);
      container.appendChild(chip);
    });

    const addBtn = el('button', 'tag-add-btn', '+ tag');
    addBtn.addEventListener('click', () => showTagInput(email, container, addBtn));
    container.appendChild(addBtn);
  }

  function showTagInput(email: Email, container: HTMLElement, anchor: HTMLElement) {
    document.querySelectorAll('.tag-input-popover').forEach(e => e.remove());

    const popover = el('div', 'tag-input-popover');
    popover.style.position = 'absolute';

    const input = el('input', '');
    input.placeholder = 'Add tag...';
    input.type = 'text';

    const suggestions = el('div', 'tag-input-suggestions');

    popover.append(input, suggestions);

    const rect = anchor.getBoundingClientRect();
    popover.style.top = (rect.bottom + window.scrollY + 4) + 'px';
    popover.style.left = (rect.left + window.scrollX) + 'px';
    document.body.appendChild(popover);

    const allTags = Object.keys(state.tags.value);
    let highlighted = -1;

    function updateSuggestions(query: string) {
      suggestions.innerHTML = '';
      highlighted = -1;
      const filtered = allTags
        .filter(t => t.toLowerCase().includes(query.toLowerCase()) && !email.tags?.includes(t))
        .slice(0, 8);
      filtered.forEach((tag, i) => {
        const item = el('div', 'tag-input-suggestion', '#' + tag);
        item.addEventListener('mousedown', (e) => {
          e.preventDefault();
          submitTag(tag);
        });
        suggestions.appendChild(item);
      });
    }

    async function submitTag(tag: string) {
      const trimmed = tag.trim().replace(/^#/, '');
      if (!trimmed) return;
      popover.remove();
      try {
        const updated = await addTag(email.id, trimmed);
        updateEmailInList(updated);
        currentEmail = updated;
        renderTagChips(updated, container);
        const tags = await listTags();
        state.tags.set(tags);
      } catch {
        showToast('Failed to add tag', 'error');
      }
    }

    input.addEventListener('input', () => updateSuggestions(input.value));
    input.addEventListener('keydown', (e) => {
      const items = suggestions.querySelectorAll('.tag-input-suggestion');
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        highlighted = Math.min(highlighted + 1, items.length - 1);
        items.forEach((it, i) => it.classList.toggle('highlighted', i === highlighted));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        highlighted = Math.max(highlighted - 1, -1);
        items.forEach((it, i) => it.classList.toggle('highlighted', i === highlighted));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (highlighted >= 0 && items[highlighted]) {
          const text = (items[highlighted] as HTMLElement).textContent?.slice(1) || '';
          submitTag(text);
        } else {
          submitTag(input.value);
        }
      } else if (e.key === 'Escape') {
        popover.remove();
      }
    });

    function handleOutside(e: MouseEvent) {
      if (!popover.contains(e.target as Node)) {
        popover.remove();
        document.removeEventListener('mousedown', handleOutside);
      }
    }
    setTimeout(() => document.addEventListener('mousedown', handleOutside), 10);

    input.focus();
    updateSuggestions('');
  }

  function buildAttachmentChip(emailId: string, att: Attachment): HTMLElement {
    const isPreviewable = att.content_type.startsWith('image/') || att.content_type === 'application/pdf';

    const chip = document.createElement('a');
    chip.className = 'attachment-chip';
    chip.href = `/api/v1/emails/${emailId}/attachments/${encodeURIComponent(att.filename)}`;
    chip.download = att.filename;
    chip.target = '_blank';
    chip.innerHTML = `
      <span class="attachment-chip-icon">${attachmentIcon(att.content_type)}</span>
      <span class="truncate" style="max-width:180px" title="${escapeHtml(att.filename)}">${escapeHtml(att.filename)}</span>
      <span class="attachment-chip-size">${formatSize(att.size)}</span>
    `;

    if (!isPreviewable) return chip;

    const wrapper = el('div', 'attachment-chip-group');
    wrapper.appendChild(chip);

    const previewBtn = el('button', 'attachment-preview-btn');
    previewBtn.innerHTML = icon('eye', 12);
    previewBtn.title = 'Preview';
    previewBtn.addEventListener('click', (e) => {
      e.preventDefault();
      openAttachmentPreview(emailId, att);
    });
    wrapper.appendChild(previewBtn);

    return wrapper;
  }

  function openAttachmentPreview(emailId: string, att: Attachment) {
    const overlay = el('div', 'modal-overlay');
    const modal = el('div', 'attachment-preview-modal');

    const header = el('div', 'attachment-preview-header');
    const title = el('span', 'attachment-preview-title', att.filename);
    const closeBtn = el('button', 'modal-close', '✕');
    closeBtn.addEventListener('click', () => overlay.remove());
    header.append(title, closeBtn);
    modal.appendChild(header);

    const body = el('div', 'attachment-preview-body');
    const url = previewAttachmentURL(emailId, att.filename);

    if (att.content_type.startsWith('image/')) {
      const img = document.createElement('img');
      img.src = url;
      img.className = 'attachment-preview-img';
      img.alt = att.filename;
      body.appendChild(img);
    } else {
      const frame = document.createElement('iframe');
      frame.src = url;
      frame.className = 'attachment-preview-pdf';
      frame.title = att.filename;
      body.appendChild(frame);
    }

    modal.appendChild(body);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
  }

  function switchTab(tab: DetailTab) {
    currentTab = tab;
    tabBtns.forEach((btn, id) => btn.classList.toggle('active', id === tab));
    htmlPanel.classList.toggle('active', tab === 'html');
    textPanel.classList.toggle('active', tab === 'text');
    rawPanel.classList.toggle('active', tab === 'raw');
    htmlSourcePanel.classList.toggle('active', tab === 'html-source');
    headersPanel.classList.toggle('active', tab === 'headers');
    linksPanel.classList.toggle('active', tab === 'links');
    htmlCheckPanel.classList.toggle('active', tab === 'htmlcheck');
    spamCheckPanel.classList.toggle('active', tab === 'spamcheck');
    smtpLogPanel.classList.toggle('active', tab === 'smtp-log');

    if (tab === 'raw' && currentEmail && !rawContent) {
      loadRaw(currentEmail.id);
    }

    if (tab === 'links' && currentEmail && linkCheckAutoToggle.checked && !linkCheckRunning) {
      runLinkCheck(currentEmail.id);
    }

    if (tab === 'smtp-log' && currentEmail && !smtpLogLoaded) {
      fetch('/api/v1/emails/' + currentEmail.id + '/smtplog')
        .then(r => r.text())
        .then(text => { renderSMTPLog(text); smtpLogLoaded = true; });
    }
  }

  async function loadRaw(id: string) {
    rawContent_.textContent = 'Loading...';
    try {
      const raw = await getEmailRaw(id);
      rawContent = raw;
      rawContent_.textContent = raw;
    } catch {
      rawContent_.textContent = '(failed to load raw source)';
    }
  }

  function renderSMTPLog(text: string) {
    smtpLogPre.innerHTML = '';
    text.split('\n').forEach(line => {
      if (!line) return;
      const span = document.createElement('span');
      if (line.startsWith('C:')) {
        span.className = 'smtp-log-client';
      } else if (line.startsWith('S:')) {
        span.className = 'smtp-log-server';
      } else {
        span.className = 'smtp-log-other';
      }
      span.textContent = line;
      smtpLogPre.appendChild(span);
      smtpLogPre.appendChild(document.createElement('br'));
    });
  }

  async function loadEmail(id: string) {
    currentEmail = null;
    rawContent = '';
    smtpLogLoaded = false;
    detailEl.style.display = 'flex';
    emptyEl.style.display = 'none';

    try {
      const email = await getEmail(id);
      currentEmail = email;
      renderEmail(email);
    } catch {
      showToast('Failed to load email', 'error');
      detailEl.style.display = 'none';
      emptyEl.style.display = 'flex';
    }
  }

  function renderEmail(email: Email) {
    buildHeader(email);
    updateActionBar(email);

    const htmlContent = email.html
      ? `<html><head><meta charset="utf-8"><style>
          body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;font-size:14px;line-height:1.5;color:#111;margin:0;padding:16px}
          img{max-width:100%}a{color:#7c3aed}
        </style></head><body>${email.html}</body></html>`
      : `<html><head><meta charset="utf-8"><style>
          body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;font-size:14px;line-height:1.7;color:#111;margin:0;padding:16px;white-space:pre-wrap}
        </style></head><body>${escapeHtml(email.text || '(no content)')}</body></html>`;

    const blob = new Blob([htmlContent], { type: 'text/html' });
    const blobUrl = URL.createObjectURL(blob);
    iframe.src = blobUrl;

    textContent.textContent = email.text || '(no plaintext content)';

    if (email.html) {
      htmlSourcePre.innerHTML = escapeHtml(email.html);
    } else {
      htmlSourcePre.innerHTML = '<span style="color:var(--text-muted)">(no HTML content)</span>';
    }

    buildHeadersTable(email);

    linkCheckResults.innerHTML = '';
    htmlCheckResults.innerHTML = '';
    spamResults.innerHTML = '';

    if (currentTab === 'raw') {
      loadRaw(email.id);
    }

    const defaultTab: DetailTab = email.html ? 'html' : 'text';
    switchTab(defaultTab);
  }

  function updateActionBar(email: Email) {
    starBtn.innerHTML = email.starred
      ? `${icon('star-filled', 13)} Unstar`
      : `${icon('star', 13)} Star`;
  }

  starBtn.addEventListener('click', async () => {
    if (!currentEmail) return;
    try {
      const updated = await patchEmail(currentEmail.id, { starred: !currentEmail.starred });
      currentEmail = updated;
      updateEmailInList(updated);
      updateActionBar(updated);
      document.dispatchEvent(new CustomEvent('mailcraft:refresh-list'));
    } catch {
      showToast('Failed to update email', 'error');
    }
  });

  readBtn.addEventListener('click', async () => {
    if (!currentEmail) return;
    try {
      const updated = await patchEmail(currentEmail.id, { read: true });
      currentEmail = updated;
      updateEmailInList(updated);
      document.dispatchEvent(new CustomEvent('mailcraft:refresh-list'));
    } catch {
      showToast('Failed to mark as read', 'error');
    }
  });

  unreadBtn.addEventListener('click', async () => {
    if (!currentEmail) return;
    try {
      const updated = await patchEmail(currentEmail.id, { read: false });
      currentEmail = updated;
      updateEmailInList(updated);
      document.dispatchEvent(new CustomEvent('mailcraft:refresh-list'));
    } catch {
      showToast('Failed to mark as unread', 'error');
    }
  });

  deleteBtn.addEventListener('click', async () => {
    if (!currentEmail) return;
    if (!await confirm('Delete this email?', true)) return;
    const id = currentEmail.id;
    try {
      await deleteEmail(id);
    } catch (e: any) {
      if (e?.status !== 404) {
        showToast('Failed to delete email', 'error');
        return;
      }
    }
    removeEmailFromList(id);
    currentEmail = null;
    rawContent = '';
    detailEl.style.display = 'none';
    emptyEl.style.display = 'flex';
    showToast('Email deleted', 'success');
  });

  exportBtn.addEventListener('click', () => {
    if (!currentEmail) return;
    const a = document.createElement('a');
    a.href = exportEmailURL(currentEmail.id);
    a.download = '';
    a.click();
  });

  state.selectedEmailId.subscribe(id => {
    if (!id) {
      detailEl.style.display = 'none';
      emptyEl.style.display = 'flex';
      currentEmail = null;
    } else {
      loadEmail(id);
    }
  });

  state.emails.subscribe(emails => {
    if (currentEmail) {
      const updated = emails.find(e => e.id === currentEmail!.id);
      if (updated && JSON.stringify(updated) !== JSON.stringify(currentEmail)) {
        currentEmail = updated;
        buildHeader(updated);
        updateActionBar(updated);
      }
    }
  });

  function buildHeadersTable(email: Email) {
    headersTable.innerHTML = '';
    const headers = email.headers || {};
    const importantHeaders = new Set(['from', 'to', 'subject', 'date', 'message-id', 'content-type', 'dkim-signature', 'spf', 'dmarc']);

    const sortedKeys = Object.keys(headers).sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    const tbody = document.createElement('tbody');

    sortedKeys.forEach(key => {
      const values = headers[key] || [];
      const tr = document.createElement('tr');
      const tdKey = document.createElement('td');
      tdKey.className = importantHeaders.has(key.toLowerCase()) ? 'header-important' : '';
      tdKey.textContent = key;

      const tdVal = document.createElement('td');
      tdVal.textContent = values.join('\n');

      tr.append(tdKey, tdVal);
      tbody.appendChild(tr);
    });

    if (sortedKeys.length === 0) {
      const tr = document.createElement('tr');
      const td = document.createElement('td');
      td.colSpan = 2;
      td.style.color = 'var(--text-muted)';
      td.style.padding = '16px';
      td.textContent = '(no headers available)';
      tr.appendChild(td);
      tbody.appendChild(tr);
    }

    headersTable.appendChild(tbody);
  }

  function buildLinkCheckPanelHTML(): string {
    return `
      <div style="padding:16px;display:flex;flex-direction:column;gap:12px;">
        <div class="link-check-warning">
          Warning: Link Check makes real HTTP requests to URLs found in the email. Only run on emails you trust.
        </div>
        <div class="link-check-options">
          <label style="display:flex;align-items:center;gap:6px;font-size:13px;cursor:pointer;">
            <input type="checkbox" id="lc-follow-redirects"> Follow HTTP redirects
          </label>
          <label style="display:flex;align-items:center;gap:6px;font-size:13px;cursor:pointer;">
            <input type="checkbox" id="lc-auto-check"> Auto-check on open
          </label>
          <button id="lc-run-btn" class="btn btn-primary btn-sm">Run Link Check</button>
        </div>
        <div id="lc-results"></div>
      </div>
    `;
  }

  function buildHTMLCheckPanelHTML(): string {
    return `
      <div style="padding:16px;display:flex;flex-direction:column;gap:12px;">
        <div style="display:flex;align-items:center;gap:12px;">
          <button id="hc-run-btn" class="btn btn-primary btn-sm">Run HTML Check</button>
        </div>
        <div id="hc-results"></div>
      </div>
    `;
  }

  async function runLinkCheck(emailId: string) {
    if (linkCheckRunning) return;
    linkCheckRunning = true;
    linkCheckRunBtn.disabled = true;
    linkCheckRunBtn.textContent = 'Running…';
    linkCheckResults.innerHTML = '';

    try {
      const response = await checkLinks(emailId, linkCheckFollowToggle.checked);
      renderLinkResults(response);
    } catch (e) {
      linkCheckResults.innerHTML = `<div style="color:var(--red);font-size:13px;">Failed to run link check: ${escapeHtml(String(e))}</div>`;
    } finally {
      linkCheckRunning = false;
      linkCheckRunBtn.disabled = false;
      linkCheckRunBtn.textContent = 'Run Link Check';
    }
  }

  function renderLinkResults(response: LinkCheckResponse) {
    if (!response.links || response.links.length === 0) {
      linkCheckResults.innerHTML = '<div style="color:var(--text-muted);font-size:13px;padding:8px 0;">No links found in this email.</div>';
      return;
    }

    const list = el('div', 'link-result-list');
    const summary = el('div', '');
    summary.style.fontSize = '12px';
    summary.style.color = 'var(--text-muted)';
    summary.style.marginBottom = '8px';
    summary.textContent = `${response.total} link${response.total !== 1 ? 's' : ''} found`;
    list.appendChild(summary);

    response.links.forEach(link => {
      const item = el('div', 'link-result-item');

      const badge = el('span', 'link-result-status');
      if (link.error || link.status === 0) {
        badge.classList.add('error');
        badge.textContent = 'ERR';
      } else if (link.status >= 200 && link.status < 300) {
        badge.classList.add('ok');
        badge.textContent = String(link.status);
      } else if (link.status >= 300 && link.status < 400) {
        badge.classList.add('redirect');
        badge.textContent = String(link.status);
      } else {
        badge.classList.add('fail');
        badge.textContent = String(link.status);
      }

      const typeIcon = el('span', 'link-result-type');
      typeIcon.innerHTML = link.type === 'image' ? icon('image', 13) : link.type === 'stylesheet' ? icon('file-text', 13) : icon('link', 13);
      typeIcon.title = link.type;

      const urlEl = el('span', 'link-result-url');
      urlEl.textContent = link.url;
      urlEl.title = link.url;

      const msEl = el('span', 'link-result-ms');
      if (link.error) {
        msEl.textContent = 'error';
        msEl.title = link.error;
        msEl.style.color = 'var(--red)';
      } else {
        msEl.textContent = link.response_ms + 'ms';
      }

      item.append(badge, typeIcon, urlEl, msEl);

      if (link.redirect_to) {
        const redirectEl = el('div', '');
        redirectEl.style.cssText = 'font-size:11px;color:var(--text-muted);padding:2px 0 0 0;flex-basis:100%;';
        redirectEl.innerHTML = `&#x2192; <span style="color:var(--accent-light)">${escapeHtml(link.redirect_to)}</span>`;
        item.appendChild(redirectEl);
        item.style.flexWrap = 'wrap';
      }

      list.appendChild(item);
    });

    linkCheckResults.innerHTML = '';
    linkCheckResults.appendChild(list);
  }

  async function runHTMLCheck(emailId: string) {
    if (htmlCheckRunning) return;
    htmlCheckRunning = true;
    htmlCheckRunBtn.disabled = true;
    htmlCheckRunBtn.textContent = 'Running…';
    htmlCheckResults.innerHTML = '';

    try {
      const result = await checkHTML(emailId);
      renderHTMLCheckResults(result);
    } catch (e) {
      htmlCheckResults.innerHTML = `<div style="color:var(--red);font-size:13px;">Failed to run HTML check: ${escapeHtml(String(e))}</div>`;
    } finally {
      htmlCheckRunning = false;
      htmlCheckRunBtn.disabled = false;
      htmlCheckRunBtn.textContent = 'Run HTML Check';
    }
  }

  function renderHTMLCheckResults(result: HTMLCheckResult) {
    const container = el('div', '');
    container.style.display = 'flex';
    container.style.flexDirection = 'column';
    container.style.gap = '12px';

    const scoreClass = result.score >= 80 ? 'good' : result.score >= 50 ? 'ok' : 'bad';
    const scoreRow = el('div', '');
    scoreRow.style.display = 'flex';
    scoreRow.style.alignItems = 'center';
    scoreRow.style.gap = '16px';

    const circle = el('div', `html-check-score-circle ${scoreClass}`);
    circle.textContent = result.score + '%';

    const scoreInfo = el('div', '');
    scoreInfo.innerHTML = `
      <div style="font-size:15px;font-weight:600;color:var(--text-primary)">Compatibility Score</div>
      <div style="font-size:12px;color:var(--text-muted)">Based on ${result.total} HTML elements analyzed</div>
    `;

    scoreRow.append(circle, scoreInfo);
    container.appendChild(scoreRow);

    if (!result.warnings || result.warnings.length === 0) {
      const ok = el('div', '');
      ok.style.cssText = 'color:var(--green);font-size:13px;display:flex;align-items:center;gap:6px;';
      ok.innerHTML = '<span style="font-size:16px;">&#x2713;</span> No compatibility issues found';
      container.appendChild(ok);
    } else {
      const warningList = el('div', '');
      result.warnings.forEach(w => {
        const item = el('div', 'html-check-warning-item');

        const row1 = el('div', '');
        row1.style.display = 'flex';
        row1.style.alignItems = 'center';
        row1.style.gap = '8px';
        row1.style.marginBottom = '4px';

        const badge = el('span', w.support === 'none' ? 'support-badge-none' : 'support-badge-partial');
        badge.textContent = w.support === 'none' ? 'None' : 'Partial';

        const name = el('code', '');
        name.style.cssText = 'font-family:var(--font-mono);font-size:12px;color:var(--text-primary);background:var(--bg-muted);padding:1px 5px;border-radius:3px;';
        name.textContent = w.name;

        const countBadge = el('span', '');
        countBadge.style.cssText = 'margin-left:auto;font-size:11px;color:var(--text-muted);background:var(--bg-muted);padding:1px 6px;border-radius:3px;';
        countBadge.textContent = w.count + ' occurrence' + (w.count !== 1 ? 's' : '');

        row1.append(badge, name, countBadge);

        const desc = el('div', '');
        desc.style.cssText = 'font-size:12px;color:var(--text-secondary);margin-bottom:2px;';
        desc.textContent = w.description;

        const clients = el('div', '');
        clients.style.cssText = 'font-size:11px;color:var(--text-muted);';
        clients.textContent = 'Clients affected: ' + w.clients;

        item.append(row1, desc, clients);
        warningList.appendChild(item);
      });
      container.appendChild(warningList);
    }

    htmlCheckResults.innerHTML = '';
    htmlCheckResults.appendChild(container);
  }

  async function runSpamCheck(emailId: string) {
    if (spamCheckRunning) return;
    spamCheckRunning = true;
    spamRunBtn.disabled = true;
    spamRunBtn.textContent = 'Running…';
    spamResults.innerHTML = '';

    try {
      const result = await checkSpam(emailId);
      renderSpamResults(result);
    } catch (e) {
      spamResults.innerHTML = `<div style="color:var(--red);font-size:13px;">Failed to run spam check: ${escapeHtml(String(e))}</div>`;
    } finally {
      spamCheckRunning = false;
      spamRunBtn.disabled = false;
      spamRunBtn.innerHTML = `${icon('check-circle', 13)} Analyze Content`;
    }
  }

  function renderSpamCheckItem(check: SpamCheckItem): HTMLElement {
    const item = el('div', 'spam-check-item');
    const ic = el('span', `spam-check-icon ${check.pass ? 'pass' : 'fail'}`, check.pass ? '✓' : '✗');
    const nameEl = el('span', '');
    nameEl.style.cssText = 'font-weight:600;font-size:12px;color:var(--text-primary);min-width:180px;';
    nameEl.textContent = check.name;
    const desc = el('span', 'spam-check-desc', check.description);
    item.append(ic, nameEl, desc);
    if (!check.info) {
      const scoreEl = el('span', `spam-check-score ${check.score > 0 ? 'positive' : 'negative'}`);
      scoreEl.textContent = (check.score > 0 ? '+' : '') + check.score.toFixed(1);
      item.appendChild(scoreEl);
    }
    return item;
  }

  function spamSectionTitle(text: string): HTMLElement {
    const t = el('div', '');
    t.style.cssText = 'font-size:11px;font-weight:600;color:var(--text-muted);text-transform:uppercase;letter-spacing:0.05em;padding:0 16px 6px;';
    t.textContent = text;
    return t;
  }

  function renderSpamResults(result: SpamCheckResult) {
    const container = el('div', '');
    container.style.cssText = 'display:flex;flex-direction:column;gap:12px;';

    // Score header — framed as a content hint, not a spam probability.
    const scoreDisplay = el('div', 'spam-score-display');
    const scoreNum = el('div', `spam-score-number ${result.level}`);
    scoreNum.textContent = result.score.toFixed(1) + ' / 10';

    const levelInfo = el('div', '');
    levelInfo.style.cssText = 'display:flex;flex-direction:column;gap:6px;';
    const levelLabel = result.level === 'ham'
      ? '✓ LOOKS CLEAN'
      : result.level === 'maybe'
        ? '⚠ COULD LOOK SUSPICIOUS'
        : '✗ LIKELY TO BE FLAGGED';
    const levelBadge = el('span', `spam-level-badge ${result.level}`, levelLabel);
    const scoreDesc = el('div', '');
    scoreDesc.style.cssText = 'font-size:12px;color:var(--text-muted);';
    scoreDesc.textContent = 'Content score (lower is better). Heuristic preview of how filters may view your message content.';
    levelInfo.append(levelBadge, scoreDesc);
    scoreDisplay.append(scoreNum, levelInfo);
    container.appendChild(scoreDisplay);

    const scored = result.checks.filter(c => !c.info);
    const infoChecks = result.checks.filter(c => c.info);
    const issues = scored.filter(c => !c.pass);

    // Content issues affecting the score.
    if (issues.length > 0) {
      const failSection = el('div', '');
      failSection.appendChild(spamSectionTitle(`Content issues (${issues.length})`));
      issues.forEach(c => failSection.appendChild(renderSpamCheckItem(c)));
      container.appendChild(failSection);
    } else {
      const ok = el('div', '');
      ok.style.cssText = 'padding:0 16px;font-size:13px;color:var(--green);';
      ok.textContent = '✓ No content issues found.';
      container.appendChild(ok);
    }

    // Passing scored checks (collapsed).
    const passingChecks = scored.filter(c => c.pass);
    if (passingChecks.length > 0) {
      let shown = false;
      const toggleBtn = el('button', 'btn btn-ghost btn-sm');
      toggleBtn.style.cssText = 'align-self:flex-start;margin-left:16px;';
      toggleBtn.textContent = `Show ${passingChecks.length} passing checks`;
      const allSection = el('div', '');
      allSection.style.display = 'none';
      passingChecks.forEach(c => allSection.appendChild(renderSpamCheckItem(c)));
      toggleBtn.addEventListener('click', () => {
        shown = !shown;
        allSection.style.display = shown ? 'block' : 'none';
        toggleBtn.textContent = shown ? 'Hide passing checks' : `Show ${passingChecks.length} passing checks`;
      });
      container.append(toggleBtn, allSection);
    }

    // Authentication: informational only, never scored in a local catcher.
    if (infoChecks.length > 0) {
      const infoSection = el('div', '');
      infoSection.appendChild(spamSectionTitle('Authentication (not scored — added by your mail server)'));
      infoChecks.forEach(c => infoSection.appendChild(renderSpamCheckItem(c)));
      container.appendChild(infoSection);
    }

    spamResults.innerHTML = '';
    spamResults.appendChild(container);
  }

  compareBtn.addEventListener('click', async () => {
    if (!currentEmail) return;
    openCompareModal(currentEmail);
  });

  async function openCompareModal(emailA: Email) {
    const overlay = el('div', 'modal-overlay');
    const modal = el('div', 'compare-modal');

    const header = el('div', 'compare-modal-header');
    header.innerHTML = '<h3>Compare Emails</h3>';
    const closeBtn = el('button', 'modal-close', '✕');
    closeBtn.addEventListener('click', () => overlay.remove());
    header.appendChild(closeBtn);
    modal.appendChild(header);

    const pickerRow = el('div', 'compare-picker-row');
    const pickerLabel = el('label', '', 'Compare with:');
    const select = document.createElement('select');
    select.className = 'compare-select';

    try {
      const res = await listEmails({ limit: 100, sort: 'received_at:desc' });
      const others = res.emails.filter(e => e.id !== emailA.id);
      if (others.length === 0) {
        select.innerHTML = '<option value="">No other emails available</option>';
      } else {
        select.innerHTML = '<option value="">— Select an email —</option>';
        others.forEach(e => {
          const opt = document.createElement('option');
          opt.value = e.id;
          opt.textContent = `${e.from.substring(0, 30)} — ${e.subject.substring(0, 40) || '(no subject)'}`;
          select.appendChild(opt);
        });
      }
    } catch {
      select.innerHTML = '<option value="">Failed to load emails</option>';
    }

    pickerRow.append(pickerLabel, select);
    modal.appendChild(pickerRow);

    const diffContainer = el('div', 'compare-diff-container');
    modal.appendChild(diffContainer);

    select.addEventListener('change', async () => {
      const emailBId = select.value;
      if (!emailBId) { diffContainer.innerHTML = ''; return; }

      diffContainer.innerHTML = '<div style="padding:20px;color:var(--text-muted)">Loading...</div>';
      try {
        const emailB = await getEmail(emailBId);
        renderDiff(emailA, emailB, diffContainer);
      } catch {
        diffContainer.innerHTML = '<div style="padding:20px;color:var(--red)">Failed to load email</div>';
      }
    });

    overlay.appendChild(modal);
    document.body.appendChild(overlay);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
  }

  function renderDiff(emailA: Email, emailB: Email, container: HTMLElement) {
    container.innerHTML = '';

    // Persistent summary (From / Subject) above the tabs.
    const headerDiff = el('div', 'compare-section');
    headerDiff.innerHTML = `
      <div class="compare-section-title">Header Comparison</div>
      <div class="compare-fields">
        <div class="compare-col">
          <div class="compare-field-label">From</div>
          <div class="compare-field-value ${emailA.from !== emailB.from ? 'diff-changed' : ''}">${escapeHtml(emailA.from)}</div>
          <div class="compare-field-label" style="margin-top:8px">Subject</div>
          <div class="compare-field-value ${emailA.subject !== emailB.subject ? 'diff-changed' : ''}">${escapeHtml(emailA.subject || '(no subject)')}</div>
        </div>
        <div class="compare-col">
          <div class="compare-field-label">From</div>
          <div class="compare-field-value ${emailA.from !== emailB.from ? 'diff-changed' : ''}">${escapeHtml(emailB.from)}</div>
          <div class="compare-field-label" style="margin-top:8px">Subject</div>
          <div class="compare-field-value ${emailA.subject !== emailB.subject ? 'diff-changed' : ''}">${escapeHtml(emailB.subject || '(no subject)')}</div>
        </div>
      </div>
    `;
    container.appendChild(headerDiff);

    // Tab bar to switch between comparison views.
    const tabs: Array<{ id: string; label: string }> = [
      { id: 'text', label: 'Plain Text' },
      { id: 'html', label: 'HTML' },
      { id: 'headers', label: 'Headers' },
      { id: 'raw', label: 'Raw' },
    ];
    const tabBar = el('div', 'compare-tabs');
    const panelHost = el('div', 'compare-tab-panel');
    const buttons: Record<string, HTMLButtonElement> = {};

    function select(id: string) {
      Object.entries(buttons).forEach(([k, b]) => b.classList.toggle('active', k === id));
      renderPanel(id);
    }

    tabs.forEach(t => {
      const b = el('button', 'compare-tab', t.label);
      b.addEventListener('click', () => select(t.id));
      buttons[t.id] = b;
      tabBar.appendChild(b);
    });

    container.appendChild(tabBar);
    container.appendChild(panelHost);

    function renderPanel(id: string) {
      panelHost.innerHTML = '';
      if (id === 'text') panelHost.appendChild(renderTextDiff(emailA, emailB));
      else if (id === 'html') panelHost.appendChild(renderHtmlCompare(emailA, emailB));
      else if (id === 'headers') panelHost.appendChild(renderHeadersDiff(emailA, emailB));
      else if (id === 'raw') renderRawCompare(emailA, emailB, panelHost);
    }

    select('text');
  }

  function renderTextDiff(emailA: Email, emailB: Email): HTMLElement {
    const section = el('div', 'compare-section');
    section.appendChild(el('div', 'compare-section-title', 'Body Diff (Plain Text)'));

    const diffLines = computeDiff(emailA.text || '', emailB.text || '');
    const pre = el('pre', 'compare-diff-pre');
    diffLines.forEach(line => {
      const span = document.createElement('span');
      span.className = `diff-line diff-${line.type}`;
      const prefix = line.type === 'added' ? '+ ' : line.type === 'removed' ? '- ' : '  ';
      span.textContent = prefix + line.text;
      pre.appendChild(span);
      pre.appendChild(document.createElement('br'));
    });
    section.appendChild(pre);
    return section;
  }

  function renderHtmlCompare(emailA: Email, emailB: Email): HTMLElement {
    const section = el('div', 'compare-section');

    const titleRow = el('div', 'compare-html-toolbar');
    titleRow.appendChild(el('div', 'compare-section-title', 'HTML'));
    const toggle = el('div', 'compare-subtabs');
    const renderedBtn = el('button', 'compare-subtab', 'Rendered');
    const sourceBtn = el('button', 'compare-subtab', 'Source');
    toggle.append(renderedBtn, sourceBtn);
    titleRow.appendChild(toggle);
    section.appendChild(titleRow);

    const body = el('div', 'compare-html-body');
    section.appendChild(body);

    function showRendered() {
      renderedBtn.classList.add('active');
      sourceBtn.classList.remove('active');
      body.innerHTML = '';
      const grid = el('div', 'compare-html-grid');
      [emailA, emailB].forEach(email => {
        const col = el('div', 'compare-html-col');
        if (email.html) {
          const frame = document.createElement('iframe');
          frame.className = 'compare-html-frame';
          frame.sandbox.add('allow-same-origin');
          frame.title = 'HTML preview';
          const doc = `<html><head><meta charset="utf-8"><style>
            body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;font-size:13px;line-height:1.5;color:#111;margin:0;padding:12px}
            img{max-width:100%}a{color:#7c3aed}
          </style></head><body>${email.html}</body></html>`;
          frame.src = URL.createObjectURL(new Blob([doc], { type: 'text/html' }));
          col.appendChild(frame);
        } else {
          col.appendChild(el('div', 'compare-empty', '(no HTML content)'));
        }
        grid.appendChild(col);
      });
      body.appendChild(grid);
    }

    function showSource() {
      sourceBtn.classList.add('active');
      renderedBtn.classList.remove('active');
      body.innerHTML = '';
      if (!emailA.html && !emailB.html) {
        body.appendChild(el('div', 'compare-empty', '(no HTML content)'));
        return;
      }
      const diffLines = computeDiff(emailA.html || '', emailB.html || '');
      const pre = el('pre', 'compare-diff-pre');
      diffLines.forEach(line => {
        const span = document.createElement('span');
        span.className = `diff-line diff-${line.type}`;
        const prefix = line.type === 'added' ? '+ ' : line.type === 'removed' ? '- ' : '  ';
        span.textContent = prefix + line.text;
        pre.appendChild(span);
        pre.appendChild(document.createElement('br'));
      });
      body.appendChild(pre);
    }

    renderedBtn.addEventListener('click', showRendered);
    sourceBtn.addEventListener('click', showSource);
    showRendered();
    return section;
  }

  function renderHeadersDiff(emailA: Email, emailB: Email): HTMLElement {
    const section = el('div', 'compare-section');
    section.appendChild(el('div', 'compare-section-title', 'All Headers'));

    const flat = (h: Record<string, string[]>) => {
      const m: Record<string, string> = {};
      Object.entries(h || {}).forEach(([k, v]) => { m[k] = (v || []).join(', '); });
      return m;
    };
    const ha = flat(emailA.headers);
    const hb = flat(emailB.headers);
    const keys = Array.from(new Set([...Object.keys(ha), ...Object.keys(hb)]))
      .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));

    const table = el('table', 'compare-headers-table');
    keys.forEach(k => {
      const va = ha[k] ?? '';
      const vb = hb[k] ?? '';
      const changed = va !== vb;
      const row = document.createElement('tr');
      if (changed) row.className = 'diff-changed-row';
      const kc = el('td', 'compare-h-key', k);
      const ac = el('td', 'compare-h-val', va || '—');
      const bc = el('td', 'compare-h-val', vb || '—');
      row.append(kc, ac, bc);
      table.appendChild(row);
    });
    section.appendChild(table);
    return section;
  }

  async function renderRawCompare(emailA: Email, emailB: Email, host: HTMLElement) {
    host.innerHTML = '';
    const section = el('div', 'compare-section');

    // Metadata summary first.
    const meta = el('div', 'compare-fields');
    [emailA, emailB].forEach(email => {
      const col = el('div', 'compare-col');
      col.innerHTML = `
        <div class="compare-field-label">Size</div>
        <div class="compare-field-value">${formatSize(email.size)}</div>
        <div class="compare-field-label" style="margin-top:8px">Received</div>
        <div class="compare-field-value">${formatDate(email.received_at)}</div>
        <div class="compare-field-label" style="margin-top:8px">Attachments</div>
        <div class="compare-field-value">${(email.attachments || []).length}</div>
      `;
      meta.appendChild(col);
    });
    section.appendChild(el('div', 'compare-section-title', 'Metadata'));
    section.appendChild(meta);
    host.appendChild(section);

    const rawSection = el('div', 'compare-section');
    rawSection.appendChild(el('div', 'compare-section-title', 'Raw Source'));
    const grid = el('div', 'compare-html-grid');
    rawSection.appendChild(grid);
    host.appendChild(rawSection);

    try {
      const [rawA, rawB] = await Promise.all([getEmailRaw(emailA.id), getEmailRaw(emailB.id)]);
      [rawA, rawB].forEach(raw => {
        const pre = el('pre', 'compare-diff-pre compare-raw-pre');
        pre.textContent = raw;
        grid.appendChild(pre);
      });
    } catch {
      grid.appendChild(el('div', 'compare-empty', 'Failed to load raw source'));
    }
  }

  function computeDiff(textA: string, textB: string): Array<{type: 'same'|'added'|'removed'; text: string}> {
    const linesA = textA.split('\n').filter(l => l.trim());
    const linesB = textB.split('\n').filter(l => l.trim());
    const setA = new Set(linesA);
    const setB = new Set(linesB);
    const result: Array<{type: 'same'|'added'|'removed'; text: string}> = [];

    for (const line of linesA) {
      result.push({ type: setB.has(line) ? 'same' : 'removed', text: line });
    }
    for (const line of linesB) {
      if (!setA.has(line)) result.push({ type: 'added', text: line });
    }
    return result;
  }

  return pane;
}
