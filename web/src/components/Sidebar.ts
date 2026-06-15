import { state, showToast } from '../state';
import type { ViewMode } from '../state';
import { el } from '../utils';
import { icon } from '../icons';
import type { IconName } from '../icons';
import { confirm, prompt } from '../dialog';
import { renameFolder, deleteFolderApi, listFolders } from '../api';

export function createSidebar(): HTMLElement {
  const sidebar = el('aside', 'sidebar');

  const nav = el('nav', 'sidebar-nav');

  const navItems: Array<{ id: ViewMode; icon: IconName; label: string }> = [
    { id: 'inbox',   icon: 'inbox',  label: 'All Mail' },
    { id: 'unread',  icon: 'mail',   label: 'Unread' },
    { id: 'starred', icon: 'star',   label: 'Starred' },
  ];

  const navButtons: Map<ViewMode, HTMLButtonElement> = new Map();

  navItems.forEach(item => {
    const btn = el('button', 'sidebar-nav-item');
    const iconSpan = el('span', 'nav-icon');
    iconSpan.innerHTML = icon(item.icon, 15);
    const label = el('span', '', item.label);
    const badge = el('span', 'nav-badge', '0');
    badge.dataset.navBadge = item.id;
    btn.append(iconSpan, label, badge);

    btn.addEventListener('click', () => {
      state.view.set(item.id);
      if (item.id === 'unread') {
        state.filterRead.set(false);
        state.filterStarred.set(null);
      } else if (item.id === 'starred') {
        state.filterStarred.set(true);
        state.filterRead.set(null);
      } else {
        state.filterRead.set(null);
        state.filterStarred.set(null);
      }
      state.filterTag.set(null);
      state.filterFolder.set(null);
    });

    navButtons.set(item.id, btn);
    nav.appendChild(btn);
  });

  const rulesBtn = el('button', 'sidebar-nav-item');
  rulesBtn.innerHTML = `<span class="nav-icon">${icon('zap', 15)}</span><span>Rules</span>`;
  rulesBtn.addEventListener('click', () => state.view.set('rules'));
  navButtons.set('rules', rulesBtn);
  nav.appendChild(rulesBtn);

  sidebar.appendChild(nav);

  const mailboxesSection = el('div', 'sidebar-section sidebar-mailbox-section');
  const mailboxesTitle = el('div', 'sidebar-section-title', 'Mailboxes');
  const mailboxesList = el('div', '');
  mailboxesList.id = 'sidebar-mailboxes-list';
  mailboxesSection.append(mailboxesTitle, mailboxesList);
  sidebar.appendChild(mailboxesSection);

  const tagsSection = el('div', 'sidebar-section');
  const tagsTitle = el('div', 'sidebar-section-title', 'Tags');
  const tagsList = el('div', '');
  tagsList.id = 'sidebar-tags-list';
  tagsSection.append(tagsTitle, tagsList);
  sidebar.appendChild(tagsSection);

  const foldersSection = el('div', 'sidebar-section sidebar-folder-section');
  const foldersTitle = el('div', 'sidebar-section-title', 'Rule Based');
  const foldersList = el('div', '');
  foldersList.id = 'sidebar-folders-list';
  foldersSection.append(foldersTitle, foldersList);
  sidebar.appendChild(foldersSection);

  const footer = el('div', 'sidebar-footer');
  const sseDot = el('div', 'sse-dot');
  sseDot.id = 'sse-dot';
  const sseLabel = el('span', '', 'Disconnected');
  sseLabel.id = 'sse-label';
  const smtpPort = el('span', 'smtp-port', 'SMTP :1025');
  smtpPort.id = 'smtp-port-label';
  footer.append(sseDot, sseLabel, smtpPort);
  sidebar.appendChild(footer);

  function updateActiveNav(view: ViewMode) {
    navButtons.forEach((btn, id) => {
      btn.classList.toggle('active', id === view);
    });
  }
  state.view.subscribe(updateActiveNav);
  updateActiveNav(state.view.value);

  function updateBadges() {
    const stats = state.stats.value;
    const allBadge = sidebar.querySelector('[data-nav-badge="inbox"]');
    if (allBadge) allBadge.textContent = String(stats.total);
    const unreadBadge = sidebar.querySelector('[data-nav-badge="unread"]') as HTMLElement;
    if (unreadBadge) {
      unreadBadge.textContent = String(stats.unread);
      unreadBadge.className = 'nav-badge' + (stats.unread > 0 ? ' unread-badge' : '');
    }
    const starredBadge = sidebar.querySelector('[data-nav-badge="starred"]');
    if (starredBadge) starredBadge.textContent = String(stats.starred);
  }
  state.stats.subscribe(updateBadges);
  updateBadges();

  function updateTags(tags: Record<string, number>) {
    tagsList.innerHTML = '';
    const entries = Object.entries(tags).sort((a, b) => b[1] - a[1]);
    if (entries.length === 0) {
      const empty = el('div', 'sidebar-tag-item', 'No tags');
      empty.style.color = 'var(--text-muted)';
      empty.style.cursor = 'default';
      tagsList.appendChild(empty);
      return;
    }
    entries.forEach(([tag, count]) => {
      const item = el('div', 'sidebar-tag-item');
      const dot = el('span', 'sidebar-tag-dot');
      const name = el('span', 'sidebar-tag-name', '#' + tag);
      const cnt = el('span', 'sidebar-tag-count', String(count));
      item.append(dot, name, cnt);
      item.addEventListener('click', () => {
        state.filterTag.set(tag);
        state.view.set('inbox');
        state.filterRead.set(null);
        state.filterStarred.set(null);
      });
      tagsList.appendChild(item);
    });
  }
  state.tags.subscribe(updateTags);
  updateTags(state.tags.value);

  state.filterTag.subscribe(tag => {
    tagsList.querySelectorAll('.sidebar-tag-item').forEach(item => {
      const nameEl = item.querySelector('.sidebar-tag-name');
      if (nameEl) {
        const itemTag = nameEl.textContent?.slice(1);
        (item as HTMLElement).classList.toggle('active', itemTag === tag);
      }
    });
  });

  function updateMailboxes(mailboxes: Record<string, number>) {
    mailboxesList.innerHTML = '';
    const entries = Object.entries(mailboxes).sort((a, b) => b[1] - a[1]);
    if (entries.length === 0) {
      const empty = el('div', 'sidebar-mailbox-item', 'No mailboxes');
      empty.style.color = 'var(--text-muted)';
      empty.style.cursor = 'default';
      mailboxesList.appendChild(empty);
      return;
    }
    entries.forEach(([mailbox, count]) => {
      const item = el('div', 'sidebar-mailbox-item');
      const iconEl = el('span', 'sidebar-mailbox-icon');
      iconEl.innerHTML = icon('inbox', 13);
      const nameEl = el('span', 'sidebar-mailbox-name', mailbox);
      const cnt = el('span', 'sidebar-mailbox-count', String(count));
      item.append(iconEl, nameEl, cnt);
      item.classList.toggle('active', state.filterMailbox.value === mailbox);
      item.addEventListener('click', () => {
        state.filterMailbox.set(mailbox);
        state.view.set('inbox');
        state.filterRead.set(null);
        state.filterStarred.set(null);
        state.filterTag.set(null);
        state.filterFolder.set(null);
      });
      mailboxesList.appendChild(item);
    });
  }
  state.mailboxes.subscribe(updateMailboxes);
  updateMailboxes(state.mailboxes.value);

  state.filterMailbox.subscribe(mailbox => {
    mailboxesList.querySelectorAll('.sidebar-mailbox-item').forEach(item => {
      const nameEl = item.querySelector('.sidebar-mailbox-name');
      if (nameEl) {
        (item as HTMLElement).classList.toggle('active', nameEl.textContent === mailbox);
      }
    });
  });

  function updateFolders(folders: Record<string, number>) {
    foldersList.innerHTML = '';
    const entries = Object.entries(folders).sort((a, b) => b[1] - a[1]);
    if (entries.length === 0) {
      const empty = el('div', 'sidebar-folder-item', 'No folders');
      empty.style.color = 'var(--text-muted)';
      empty.style.cursor = 'default';
      foldersList.appendChild(empty);
      return;
    }
    entries.forEach(([folder, count]) => {
      const item = el('div', 'sidebar-folder-item');
      const iconEl = el('span', 'sidebar-folder-icon');
      iconEl.innerHTML = icon('folder', 13);
      const nameEl = el('span', 'sidebar-folder-name', folder);
      const cnt = el('span', 'sidebar-folder-count', String(count));

      const actions = el('div', 'sidebar-folder-actions');

      const renameBtn = el('button', 'sidebar-folder-action-btn');
      renameBtn.innerHTML = icon('pencil', 11);
      renameBtn.title = 'Rename folder';
      renameBtn.addEventListener('click', async e => {
        e.stopPropagation();
        const newName = await prompt(`Rename folder "${folder}" to:`, folder);
        if (!newName || newName === folder) return;
        try {
          await renameFolder(folder, newName);
          if (state.filterFolder.value === folder) state.filterFolder.set(newName);
          const updated = await listFolders();
          state.folders.set(updated);
          showToast('Folder renamed', 'success');
        } catch {
          showToast('Failed to rename folder', 'error');
        }
      });

      const deleteBtn = el('button', 'sidebar-folder-action-btn');
      deleteBtn.innerHTML = icon('trash-2', 11);
      deleteBtn.title = 'Delete folder';
      deleteBtn.addEventListener('click', async e => {
        e.stopPropagation();
        if (!await confirm(`Remove all emails from folder "${folder}"?`, true)) return;
        try {
          await deleteFolderApi(folder);
          if (state.filterFolder.value === folder) state.filterFolder.set(null);
          const updated = await listFolders();
          state.folders.set(updated);
          showToast('Folder deleted', 'success');
        } catch {
          showToast('Failed to delete folder', 'error');
        }
      });

      actions.append(renameBtn, deleteBtn);
      item.append(iconEl, nameEl, actions, cnt);
      item.classList.toggle('active', state.filterFolder.value === folder);
      item.addEventListener('click', () => {
        state.filterFolder.set(folder);
        state.view.set('inbox');
        state.filterRead.set(null);
        state.filterStarred.set(null);
        state.filterTag.set(null);
      });
      foldersList.appendChild(item);
    });
  }
  state.folders.subscribe(updateFolders);
  updateFolders(state.folders.value);

  state.filterFolder.subscribe(folder => {
    foldersList.querySelectorAll('.sidebar-folder-item').forEach(item => {
      const nameEl = item.querySelector('.sidebar-folder-name');
      if (nameEl) {
        (item as HTMLElement).classList.toggle('active', nameEl.textContent === folder);
      }
    });
  });

  state.sseConnected.subscribe(connected => {
    const dot = document.getElementById('sse-dot');
    const label = document.getElementById('sse-label');
    if (dot) dot.classList.toggle('connected', connected);
    if (label) label.textContent = connected ? 'Live' : 'Disconnected';
  });

  state.smtpPort.subscribe(port => {
    const el = document.getElementById('smtp-port-label');
    if (el) el.textContent = `SMTP :${port}`;
  });

  return sidebar;
}
