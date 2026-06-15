import { state, updateEmailInList, removeEmailFromList, showToast } from '../state';
import type { Email } from '../api';
import { deleteEmail, deleteEmails, patchEmail, listEmails } from '../api';
import { relativeTime, avatarColor, getInitials, senderName, el } from '../utils';
import { icon } from '../icons';
import { confirm } from '../dialog';

const PAGE_SIZE = 50;

export function createEmailList(): HTMLElement {
  const pane = el('div', 'email-list-pane');
  pane.setAttribute('tabindex', '0');

  const header = el('div', 'email-list-header');
  const title = el('h2', '', 'All Mail');
  const countEl = el('span', 'email-list-count', '');
  header.append(title, countEl);
  pane.appendChild(header);

  const toolbar = el('div', 'email-list-toolbar');
  toolbar.style.display = 'none';
  toolbar.innerHTML = `
    <button class="toolbar-btn" id="tb-mark-all">Mark All</button>
    <button class="toolbar-btn" id="tb-mark-read">Read</button>
    <button class="toolbar-btn" id="tb-mark-unread">Unread</button>
    <button class="toolbar-btn" id="tb-export">Export</button>
    <button class="toolbar-btn danger" id="tb-delete">Delete</button>
    <button class="toolbar-btn muted" id="tb-clear-sel">✕ Deselect</button>
  `;
  pane.appendChild(toolbar);

  const scrollEl = el('div', 'email-list-scroll');
  scrollEl.id = 'email-list-scroll';
  pane.appendChild(scrollEl);

  let lastClickIndex = -1;

  function renderList(emails: Email[]) {
    scrollEl.innerHTML = '';

    if (emails.length === 0) {
      const empty = el('div', 'email-list-empty');
      empty.innerHTML = `
        <div class="email-list-empty-icon">${icon('inbox', 40)}</div>
        <div>No emails found</div>
        <div style="font-size:12px;color:var(--text-muted)">Emails sent to the SMTP server will appear here</div>
      `;
      scrollEl.appendChild(empty);
      return;
    }

    emails.forEach((email, idx) => {
      scrollEl.appendChild(renderEmailItem(email, idx));
    });

    setupLoadMore(emails);
  }

  function renderEmailItem(email: Email, idx: number): HTMLElement {
    const item = el('div', 'email-item');
    item.dataset.id = email.id;
    item.dataset.idx = String(idx);
    if (!email.read) item.classList.add('unread');
    if (state.selectedEmailId.value === email.id) item.classList.add('active-detail');
    if (state.selectedIds.value.has(email.id)) item.classList.add('selected');
    if (email.color) {
      item.classList.add('has-color');
      item.style.setProperty('--item-color', email.color);
    }

    const color = avatarColor(email.from);
    const initials = getInitials(email.from);
    const name = senderName(email.from);
    const time = relativeTime(email.received_at);

    const checkbox = el('div', 'email-item-checkbox');
    checkbox.innerHTML = icon('check', 11);
    checkbox.title = 'Select';

    const avatar = el('div', 'email-item-avatar');
    avatar.style.background = color;
    avatar.textContent = initials;

    const content = el('div', 'email-item-content');

    const top = el('div', 'email-item-top');
    const fromEl = el('span', 'email-item-from', name);
    const timeEl = el('span', 'email-item-time', time);
    top.append(fromEl, timeEl);

    const subject = el('div', 'email-item-subject', email.subject || '(no subject)');
    const preview = el('div', 'email-item-preview', email.text?.slice(0, 100) || '');

    const meta = el('div', 'email-item-meta');
    if (!email.read) {
      const dot = el('span', 'email-item-unread-dot');
      dot.title = 'Unread';
      meta.appendChild(dot);
    }
    if (email.starred) {
      const star = el('span', 'email-item-starred');
      star.innerHTML = icon('star-filled', 12);
      meta.appendChild(star);
    }
    (email.tags || []).forEach(tag => {
      const chip = el('span', 'tag-chip', '#' + tag);
      meta.appendChild(chip);
    });

    content.append(top, subject, preview, meta);
    item.append(checkbox, avatar, content);

    item.addEventListener('click', (e) => {
      const target = e.target as HTMLElement;

      if (target.closest('.email-item-checkbox')) {
        e.stopPropagation();
        toggleSelect(email.id, idx, e.shiftKey);
        return;
      }

      if (e.shiftKey && lastClickIndex >= 0) {
        const emails = state.emails.value;
        const min = Math.min(lastClickIndex, idx);
        const max = Math.max(lastClickIndex, idx);
        const nextIds = new Set(state.selectedIds.value);
        emails.slice(min, max + 1).forEach(em => nextIds.add(em.id));
        state.selectedIds.set(nextIds);
        return;
      }

      state.selectedIds.set(new Set());
      state.selectedEmailId.set(email.id);
      lastClickIndex = idx;
    });

    checkbox.addEventListener('click', (e) => {
      e.stopPropagation();
      toggleSelect(email.id, idx, false);
    });

    return item;
  }

  function toggleSelect(id: string, idx: number, shift: boolean) {
    const emails = state.emails.value;
    const nextIds = new Set(state.selectedIds.value);
    if (shift && lastClickIndex >= 0) {
      const min = Math.min(lastClickIndex, idx);
      const max = Math.max(lastClickIndex, idx);
      emails.slice(min, max + 1).forEach(em => nextIds.add(em.id));
    } else if (nextIds.has(id)) {
      nextIds.delete(id);
    } else {
      nextIds.add(id);
    }
    state.selectedIds.set(nextIds);
    lastClickIndex = idx;
  }

  let loadingMore = false;
  let currentPage = 0;

  function setupLoadMore(emails: Email[]) {
    const total = state.total.value;
    if (emails.length >= total) return;

    const remaining = total - emails.length;
    const row = el('div', 'email-list-load-more');
    const btn = el('button', 'load-more-btn', `Load ${remaining} more`);
    row.appendChild(btn);
    scrollEl.appendChild(row);

    btn.addEventListener('click', async () => {
      btn.textContent = 'Loading…';
      btn.disabled = true;
      await loadMore();
    });
  }

  async function loadMore() {
    if (loadingMore || state.emails.value.length >= state.total.value) return;
    loadingMore = true;
    currentPage += 1;
    try {
      const params = buildParams();
      params.page = currentPage;
      params.limit = PAGE_SIZE;
      const res = await listEmails(params);
      state.emails.update(existing => [...existing, ...res.emails]);
    } catch (e) {
      console.error('load more failed', e);
    } finally {
      loadingMore = false;
    }
  }

  function buildParams() {
    const view = state.view.value;
    const params: Parameters<typeof listEmails>[0] = {
      page: 0,
      limit: PAGE_SIZE,
      sort: 'received_at:desc',
    };
    if (state.search.value) params.q = state.search.value;
    if (state.filterTag.value) params.tag = state.filterTag.value;
    if (state.filterFolder.value) params.folder = state.filterFolder.value;
    if (state.filterMailbox.value) params.mailbox = state.filterMailbox.value;
    if (view === 'unread' || state.filterRead.value !== null) {
      params.read = view === 'unread' ? false : (state.filterRead.value ?? undefined);
    }
    if (view === 'starred' || state.filterStarred.value !== null) {
      params.starred = view === 'starred' ? true : (state.filterStarred.value ?? undefined);
    }
    return params;
  }

  toolbar.querySelector('#tb-mark-read')!.addEventListener('click', async () => {
    const ids = Array.from(state.selectedIds.value);
    try {
      await Promise.all(ids.map(id => patchEmail(id, { read: true })));
      state.selectedIds.set(new Set());
      showToast(`Marked ${ids.length} email(s) as read`, 'success');
      await refreshList();
    } catch {
      showToast('Failed to mark as read', 'error');
    }
  });

  toolbar.querySelector('#tb-mark-unread')!.addEventListener('click', async () => {
    const ids = Array.from(state.selectedIds.value);
    try {
      await Promise.all(ids.map(id => patchEmail(id, { read: false })));
      state.selectedIds.set(new Set());
      showToast(`Marked ${ids.length} email(s) as unread`, 'success');
      await refreshList();
    } catch {
      showToast('Failed to mark as unread', 'error');
    }
  });

  toolbar.querySelector('#tb-delete')!.addEventListener('click', async () => {
    const ids = Array.from(state.selectedIds.value);
    if (!ids.length) return;
    if (!await confirm(`Delete ${ids.length} email(s)?`, true)) return;
    try {
      await deleteEmails(ids);
      ids.forEach(id => removeEmailFromList(id));
      state.selectedIds.set(new Set());
      showToast(`Deleted ${ids.length} email(s)`, 'success');
      await refreshList();
    } catch {
      showToast('Failed to delete emails', 'error');
    }
  });

  toolbar.querySelector('#tb-mark-all')!.addEventListener('click', () => {
    const allIds = new Set(state.emails.value.map(e => e.id));
    state.selectedIds.set(allIds);
  });

  toolbar.querySelector('#tb-clear-sel')!.addEventListener('click', () => {
    state.selectedIds.set(new Set());
  });

  toolbar.querySelector('#tb-export')!.addEventListener('click', () => {
    const ids = Array.from(state.selectedIds.value);
    if (!ids.length) return;
    fetch('/api/v1/emails/export', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    })
      .then(r => r.blob())
      .then(blob => {
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'mailcraft-export.zip';
        a.click();
        URL.revokeObjectURL(url);
      })
      .catch(() => showToast('Export failed', 'error'));
  });

  pane.addEventListener('keydown', (e: KeyboardEvent) => {
    const emails = state.emails.value;
    if (!emails.length) return;

    const curId = state.selectedEmailId.value;
    const curIdx = curId ? emails.findIndex(em => em.id === curId) : -1;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const next = Math.min(curIdx + 1, emails.length - 1);
      state.selectedEmailId.set(emails[next].id);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      const prev = Math.max(curIdx - 1, 0);
      state.selectedEmailId.set(emails[prev].id);
    } else if (e.key === 'Delete' || e.key === 'Backspace') {
      if (curId) {
        e.preventDefault();
        handleDeleteSingle(curId, curIdx, emails);
      }
    } else if (e.key === 'r' || e.key === 'R') {
      if (curId) {
        e.preventDefault();
        patchEmail(curId, { read: true }).then(updated => {
          updateEmailInList(updated);
        });
      }
    }
  });

  async function handleDeleteSingle(id: string, idx: number, emails: Email[]) {
    if (!await confirm('Delete this email?', true)) return;
    try {
      await deleteEmail(id);
    } catch (e: any) {
      if (e?.status !== 404) { showToast('Failed to delete email', 'error'); return; }
    }
    removeEmailFromList(id);
    showToast('Email deleted', 'success');
    const remaining = state.emails.value;
    if (remaining.length > 0) {
      const next = remaining[Math.min(idx, remaining.length - 1)];
      state.selectedEmailId.set(next.id);
    }
  }

  async function refreshList() {
    currentPage = 0;
    loadingMore = false;
    state.loading.set(true);
    try {
      const res = await listEmails({ ...buildParams(), page: 0, limit: PAGE_SIZE });
      state.emails.set(res.emails);
      state.total.set(res.total);
    } catch (e) {
      console.error('refresh failed', e);
    } finally {
      state.loading.set(false);
    }
  }

  function updateHeader(view: string) {
    if (state.filterMailbox.value) {
      title.textContent = state.filterMailbox.value;
      return;
    }
    if (state.filterFolder.value) {
      title.textContent = state.filterFolder.value;
      return;
    }
    const titles: Record<string, string> = {
      inbox: 'All Mail', unread: 'Unread', starred: 'Starred', rules: 'Rules'
    };
    title.textContent = titles[view] || 'All Mail';
  }

  state.view.subscribe(v => {
    updateHeader(v);
    refreshList();
  });

  state.emails.subscribe(emails => {
    renderList(emails);
  });

  state.total.subscribe(t => {
    const cur = state.emails.value.length;
    countEl.textContent = cur < t ? `${cur} of ${t}` : `${t}`;
  });

  state.selectedIds.subscribe(ids => {
    const count = ids.size;
    toolbar.style.display = count > 0 ? 'flex' : 'none';

    scrollEl.querySelectorAll('.email-item').forEach(item => {
      const id = (item as HTMLElement).dataset.id;
      if (id) {
        item.classList.toggle('selected', ids.has(id));
      }
    });
  });

  state.selectedEmailId.subscribe(id => {
    scrollEl.querySelectorAll('.email-item').forEach(item => {
      const itemId = (item as HTMLElement).dataset.id;
      item.classList.toggle('active-detail', itemId === id);
    });
    if (id) {
      const el = scrollEl.querySelector(`[data-id="${id}"]`) as HTMLElement;
      if (el) {
        const top = el.offsetTop;
        const bottom = top + el.offsetHeight;
        if (top < scrollEl.scrollTop || bottom > scrollEl.scrollTop + scrollEl.clientHeight) {
          el.scrollIntoView({ block: 'nearest' });
        }
      }
    }
  });

  state.search.subscribe(() => refreshList());
  state.filterTag.subscribe(() => refreshList());
  state.filterRead.subscribe(() => refreshList());
  state.filterStarred.subscribe(() => refreshList());
  state.filterFolder.subscribe(() => {
    updateHeader(state.view.value);
    refreshList();
  });
  state.filterMailbox.subscribe(() => {
    updateHeader(state.view.value);
    refreshList();
  });

  pane.addEventListener('refresh-list', () => refreshList());

  document.addEventListener('mailcraft:refresh-list', () => refreshList());

  updateHeader(state.view.value);
  renderList(state.emails.value);
  refreshList();

  return pane;
}
