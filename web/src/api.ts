export interface Attachment {
  filename: string;
  content_type: string;
  size: number;
}

export interface Email {
  id: string;
  message_id: string;
  from: string;
  to: string[];
  cc: string[];
  bcc: string[];
  subject: string;
  text: string;
  html: string;
  attachments: Attachment[];
  headers: Record<string, string[]>;
  tags: string[];
  color?: string;
  folder?: string;
  read: boolean;
  starred: boolean;
  size: number;
  received_at: string;
}

export interface Condition {
  field: 'from' | 'to' | 'subject' | 'body' | 'header' | 'tag' | 'size' | 'has_attachment';
  operator: 'contains' | 'not_contains' | 'equals' | 'not_equals' | 'starts_with' | 'ends_with' | 'regex' | 'gt' | 'lt' | 'exists';
  value: string;
  header_key?: string;
}

export interface Action {
  type: 'tag' | 'remove_tag' | 'color' | 'mark_read' | 'star' | 'delete' | 'webhook' | 'folder';
  value: string;
}

export interface Rule {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  priority: number;
  logic: 'AND' | 'OR';
  conditions: Condition[];
  actions: Action[];
  stats: { match_count: number; last_match_at?: string };
  created_at: string;
  updated_at: string;
}

export interface Stats {
  total: number;
  unread: number;
  starred: number;
  size_bytes: number;
  rules_count: number;
}

export interface EmailListResponse {
  emails: Email[];
  total: number;
  page: number;
  limit: number;
}

export interface EmailListParams {
  q?: string;
  tag?: string;
  folder?: string;
  read?: boolean;
  starred?: boolean;
  from?: string;
  to?: string;
  page?: number;
  limit?: number;
  sort?: string;
}

class APIError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'APIError';
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  });

  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      msg = body.error || msg;
    } catch {}
    throw new APIError(res.status, msg);
  }

  if (res.status === 204) return undefined as unknown as T;

  return res.json();
}

export async function listEmails(params: EmailListParams = {}): Promise<EmailListResponse> {
  const q = new URLSearchParams();
  if (params.q) q.set('q', params.q);
  if (params.tag) q.set('tag', params.tag);
  if (params.folder) q.set('folder', params.folder);
  if (params.read !== undefined) q.set('read', String(params.read));
  if (params.starred !== undefined) q.set('starred', String(params.starred));
  if (params.from) q.set('from', params.from);
  if (params.to) q.set('to', params.to);
  if (params.page !== undefined) q.set('page', String(params.page));
  if (params.limit !== undefined) q.set('limit', String(params.limit));
  if (params.sort) q.set('sort', params.sort);
  const qs = q.toString();
  return request<EmailListResponse>(`/emails${qs ? '?' + qs : ''}`);
}

export async function getEmail(id: string): Promise<Email> {
  return request<Email>(`/emails/${id}`);
}

export async function getEmailRaw(id: string): Promise<string> {
  const res = await fetch(`/api/v1/emails/${id}/raw`);
  if (!res.ok) throw new APIError(res.status, `HTTP ${res.status}`);
  return res.text();
}

export function exportEmailURL(id: string): string {
  return `/api/v1/emails/${id}/export`;
}

export async function deleteEmail(id: string): Promise<void> {
  return request(`/emails/${id}`, { method: 'DELETE' });
}

export async function deleteEmails(ids?: string[]): Promise<void> {
  return request(`/emails`, {
    method: 'DELETE',
    body: JSON.stringify({ ids }),
  });
}

export async function patchEmail(id: string, patch: { read?: boolean; starred?: boolean; tags?: string[]; color?: string; folder?: string }): Promise<Email> {
  return request<Email>(`/emails/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  });
}

export async function addTag(id: string, tag: string): Promise<Email> {
  return request<Email>(`/emails/${id}/tags`, {
    method: 'POST',
    body: JSON.stringify({ tag }),
  });
}

export async function removeTag(id: string, tag: string): Promise<Email> {
  return request<Email>(`/emails/${id}/tags/${encodeURIComponent(tag)}`, {
    method: 'DELETE',
  });
}

export async function listRules(): Promise<Rule[]> {
  return request<Rule[]>('/rules');
}

export async function createRule(rule: Omit<Rule, 'id' | 'stats' | 'created_at' | 'updated_at'>): Promise<Rule> {
  return request<Rule>('/rules', {
    method: 'POST',
    body: JSON.stringify(rule),
  });
}

export async function getRule(id: string): Promise<Rule> {
  return request<Rule>(`/rules/${id}`);
}

export async function updateRule(id: string, rule: Partial<Rule>): Promise<Rule> {
  return request<Rule>(`/rules/${id}`, {
    method: 'PUT',
    body: JSON.stringify(rule),
  });
}

export async function patchRule(id: string, patch: Partial<Rule>): Promise<Rule> {
  return request<Rule>(`/rules/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  });
}

export async function deleteRule(id: string): Promise<void> {
  return request(`/rules/${id}`, { method: 'DELETE' });
}

export async function testRule(id: string): Promise<{ match_count: number }> {
  return request<{ match_count: number }>(`/rules/${id}/test`, { method: 'POST' });
}

export async function reorderRules(ids: string[]): Promise<void> {
  return request('/rules/reorder', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  });
}

export async function listFolders(): Promise<Record<string, number>> {
  return request<Record<string, number>>('/folders');
}

export async function renameFolder(name: string, newName: string): Promise<void> {
  return request(`/folders/${encodeURIComponent(name)}`, {
    method: 'PUT',
    body: JSON.stringify({ new_name: newName }),
  });
}

export async function deleteFolderApi(name: string): Promise<void> {
  return request(`/folders/${encodeURIComponent(name)}`, { method: 'DELETE' });
}

export interface ConfigData {
  smtp_addr: string;
  http_addr: string;
  max_emails: number;
  base_path: string;
  log_level: string;
}

export async function patchConfig(patch: { log_level?: string; max_emails?: number }): Promise<ConfigData> {
  return request<ConfigData>('/config', {
    method: 'PATCH',
    body: JSON.stringify(patch),
  });
}

export async function listTags(): Promise<Record<string, number>> {
  return request<Record<string, number>>('/tags');
}

export async function deleteTagApi(name: string): Promise<void> {
  return request(`/tags/${encodeURIComponent(name)}`, { method: 'DELETE' });
}

export async function renameTag(name: string, newName: string): Promise<void> {
  return request(`/tags/${encodeURIComponent(name)}`, {
    method: 'PUT',
    body: JSON.stringify({ new_name: newName }),
  });
}

export interface LinkResult {
  url: string;
  type: 'link' | 'image' | 'stylesheet';
  status: number;
  status_text: string;
  redirect_to?: string;
  response_ms: number;
  error?: string;
}

export interface LinkCheckResponse {
  links: LinkResult[];
  total: number;
}

export async function checkLinks(emailId: string, followRedirects: boolean): Promise<LinkCheckResponse> {
  return request<LinkCheckResponse>(`/emails/${emailId}/linkcheck?follow=${followRedirects}`);
}

export interface HTMLCheckWarning {
  type: string;
  name: string;
  count: number;
  support: 'none' | 'partial' | 'good';
  score: number;
  description: string;
  clients: string;
}

export interface HTMLCheckResult {
  score: number;
  warnings: HTMLCheckWarning[];
  total: number;
}

export async function checkHTML(emailId: string): Promise<HTMLCheckResult> {
  return request<HTMLCheckResult>(`/emails/${emailId}/htmlcheck`);
}

export interface SpamCheckItem {
  name: string;
  category: string;
  score: number;
  description: string;
  pass: boolean;
  info: boolean;
}

export interface SpamCheckResult {
  score: number;
  level: 'ham' | 'maybe' | 'spam';
  checks: SpamCheckItem[];
}

export async function checkSpam(emailId: string): Promise<SpamCheckResult> {
  return request<SpamCheckResult>(`/emails/${emailId}/spamcheck`);
}

export async function getStats(): Promise<Stats> {
  return request<Stats>('/stats');
}

export async function getHealth(): Promise<{ status: string; version: string; uptime_s: number }> {
  return request('/health');
}
