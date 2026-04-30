import { icon } from './icons';

export function relativeTime(dateStr: string): string {
  const now = Date.now();
  const date = new Date(dateStr).getTime();
  const diff = now - date;

  if (diff < 0) return 'just now';
  if (diff < 60_000) return 'just now';
  if (diff < 3_600_000) {
    const m = Math.floor(diff / 60_000);
    return `${m} min ago`;
  }
  if (diff < 86_400_000) {
    const h = Math.floor(diff / 3_600_000);
    return `${h}h ago`;
  }
  if (diff < 7 * 86_400_000) {
    const d = Math.floor(diff / 86_400_000);
    return d === 1 ? 'yesterday' : `${d} days ago`;
  }
  return new Date(dateStr).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

const AVATAR_COLORS = [
  '#7c3aed', '#2563eb', '#059669', '#d97706',
  '#dc2626', '#7c3aed', '#0891b2', '#9333ea',
  '#16a34a', '#ca8a04', '#b45309', '#0f766e',
];

export function avatarColor(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash) + str.charCodeAt(i);
    hash |= 0;
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

export function getInitials(from: string): string {
  const nameMatch = from.match(/^([^<]+)</);
  if (nameMatch) {
    const parts = nameMatch[1].trim().split(/\s+/);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
    }
    return parts[0].slice(0, 2).toUpperCase();
  }
  const emailMatch = from.match(/([^@<\s]+)@/);
  if (emailMatch) {
    return emailMatch[1].slice(0, 2).toUpperCase();
  }
  return from.slice(0, 2).toUpperCase();
}

export function senderName(from: string): string {
  const nameMatch = from.match(/^([^<]+)</);
  if (nameMatch) return nameMatch[1].trim();
  const emailMatch = from.match(/<([^>]+)>/);
  if (emailMatch) return emailMatch[1];
  return from;
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

export function debounce<T extends (...args: unknown[]) => unknown>(fn: T, ms: number): (...args: Parameters<T>) => void {
  let timer: ReturnType<typeof setTimeout> | null = null;
  return (...args: Parameters<T>) => {
    if (timer !== null) clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}

export function attachmentIcon(contentType: string): string {
  if (contentType.startsWith('image/')) return icon('image', 14);
  if (contentType.startsWith('video/')) return icon('film', 14);
  if (contentType.startsWith('audio/')) return icon('music', 14);
  if (contentType === 'application/pdf') return icon('file-text', 14);
  if (contentType.includes('zip') || contentType.includes('tar') || contentType.includes('gzip')) return icon('archive', 14);
  if (contentType.includes('excel') || contentType.includes('spreadsheet')) return icon('table', 14);
  if (contentType.startsWith('text/')) return icon('code', 14);
  return icon('paperclip', 14);
}

export function clamp(n: number, min: number, max: number): number {
  return Math.min(Math.max(n, min), max);
}

export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text) e.textContent = text;
  return e;
}

export interface HashState {
  q?: string;
  tag?: string;
  read?: boolean;
  starred?: boolean;
  emailId?: string;
  view?: 'inbox' | 'unread' | 'starred' | 'rules';
}

export function parseHash(): HashState {
  const hash = window.location.hash.slice(1);
  if (!hash) return {};
  const params = new URLSearchParams(hash);
  const state: HashState = {};
  if (params.has('q')) state.q = params.get('q')!;
  if (params.has('tag')) state.tag = params.get('tag')!;
  if (params.has('read')) state.read = params.get('read') === 'true';
  if (params.has('starred')) state.starred = params.get('starred') === 'true';
  if (params.has('id')) state.emailId = params.get('id')!;
  const v = params.get('view');
  if (v === 'inbox' || v === 'unread' || v === 'starred' || v === 'rules') state.view = v;
  return state;
}

export function buildHash(state: HashState): string {
  const params = new URLSearchParams();
  if (state.view && state.view !== 'inbox') params.set('view', state.view);
  if (state.q) params.set('q', state.q);
  if (state.tag) params.set('tag', state.tag);
  if (state.read !== undefined) params.set('read', String(state.read));
  if (state.starred !== undefined) params.set('starred', String(state.starred));
  if (state.emailId) params.set('id', state.emailId);
  const s = params.toString();
  return s ? '#' + s : '#';
}

export function setHash(state: HashState): void {
  const newHash = buildHash(state);
  if (window.location.hash !== newHash) {
    window.history.pushState(null, '', newHash || window.location.pathname);
  }
}
