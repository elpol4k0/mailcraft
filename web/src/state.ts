import type { Email, Stats } from './api';
import type { HashState } from './utils';

export type ViewMode = 'inbox' | 'unread' | 'starred' | 'rules' | 'settings' | 'folder';

export interface AppState {
  emails: Email[];
  total: number;
  loading: boolean;
  selectedEmailId: string | null;
  selectedIds: Set<string>;
  view: ViewMode;
  search: string;
  filterTag: string | null;
  filterRead: boolean | null;
  filterStarred: boolean | null;
  stats: Stats;
  tags: Record<string, number>;
  sseConnected: boolean;
  smtpPort: string;
}

type Listener<T> = (value: T) => void;

class Observable<T> {
  private listeners: Set<Listener<T>> = new Set();
  private _value: T;

  constructor(initial: T) {
    this._value = initial;
  }

  get value(): T { return this._value; }

  set(next: T): void {
    this._value = next;
    this.listeners.forEach(fn => fn(next));
  }

  update(fn: (v: T) => T): void {
    this.set(fn(this._value));
  }

  subscribe(fn: Listener<T>): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }
}

const defaultStats: Stats = { total: 0, unread: 0, starred: 0, size_bytes: 0, rules_count: 0 };

export const state = {
  emails: new Observable<Email[]>([]),
  total: new Observable<number>(0),
  loading: new Observable<boolean>(false),
  selectedEmailId: new Observable<string | null>(null),
  selectedIds: new Observable<Set<string>>(new Set()),
  view: new Observable<ViewMode>('inbox'),
  search: new Observable<string>(''),
  filterTag: new Observable<string | null>(null),
  filterRead: new Observable<boolean | null>(null),
  filterStarred: new Observable<boolean | null>(null),
  stats: new Observable<Stats>(defaultStats),
  tags: new Observable<Record<string, number>>({}),
  folders: new Observable<Record<string, number>>({}),
  filterFolder: new Observable<string | null>(null),
  sseConnected: new Observable<boolean>(false),
  smtpPort: new Observable<string>('1025'),
};

export function getActiveFilters(): HashState {
  return {
    view: state.view.value,
    q: state.search.value || undefined,
    tag: state.filterTag.value || undefined,
    read: state.filterRead.value !== null ? (state.filterRead.value as boolean) : undefined,
    starred: state.filterStarred.value !== null ? (state.filterStarred.value as boolean) : undefined,
    emailId: state.selectedEmailId.value || undefined,
  };
}

export function applyHashState(hash: HashState): void {
  if (hash.view) state.view.set(hash.view);
  if (hash.q !== undefined) state.search.set(hash.q);
  if (hash.tag !== undefined) state.filterTag.set(hash.tag);
  if (hash.read !== undefined) state.filterRead.set(hash.read);
  if (hash.starred !== undefined) state.filterStarred.set(hash.starred);
  if (hash.emailId !== undefined) state.selectedEmailId.set(hash.emailId);
}

export function clearFilters(): void {
  state.search.set('');
  state.filterTag.set(null);
  state.filterRead.set(null);
  state.filterStarred.set(null);
  state.filterFolder.set(null);
}

export function updateEmailInList(updated: Email): void {
  state.emails.update(emails =>
    emails.map(e => e.id === updated.id ? updated : e)
  );
}

export function removeEmailFromList(id: string): void {
  state.emails.update(emails => emails.filter(e => e.id !== id));
  state.total.update(t => Math.max(0, t - 1));
  if (state.selectedEmailId.value === id) {
    state.selectedEmailId.set(null);
  }
  state.selectedIds.update(ids => {
    const next = new Set(ids);
    next.delete(id);
    return next;
  });
}

export function prependEmail(email: Email): void {
  state.emails.update(emails => [email, ...emails]);
  state.total.update(t => t + 1);
}

export type ToastType = 'success' | 'error' | 'info';

interface Toast {
  id: number;
  type: ToastType;
  message: string;
}

let toastCounter = 0;
export const toasts = new Observable<Toast[]>([]);

export function showToast(message: string, type: ToastType = 'info', duration = 3000): void {
  const id = ++toastCounter;
  toasts.update(t => [...t, { id, type, message }]);
  setTimeout(() => {
    toasts.update(t => t.filter(x => x.id !== id));
  }, duration);
}
