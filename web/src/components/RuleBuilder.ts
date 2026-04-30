import { state, showToast } from '../state';
import type { Rule, Condition, Action } from '../api';
import { listRules, createRule, updateRule, deleteRule, testRule, patchRule } from '../api';
import { el, formatDate } from '../utils';
import { icon } from '../icons';
import { confirm } from '../dialog';

export function createRulesPage(): HTMLElement {
  const page = el('div', 'rules-page');

  const header = el('div', 'rules-page-header');
  const title = el('h2', '', 'Rules');
  const newBtn = el('button', 'btn btn-primary', '+ New Rule');
  header.append(title, newBtn);
  page.appendChild(header);

  const listEl = el('div', '');
  listEl.id = 'rules-list';
  page.appendChild(listEl);

  async function loadRules() {
    listEl.innerHTML = '<div style="color:var(--text-muted);padding:20px">Loading...</div>';
    try {
      const rules = await listRules();
      renderRules(rules);
    } catch {
      listEl.innerHTML = '<div style="color:var(--red);padding:20px">Failed to load rules</div>';
    }
  }

  function renderRules(rules: Rule[]) {
    listEl.innerHTML = '';
    if (rules.length === 0) {
      const empty = el('div', 'rules-empty');
      empty.innerHTML = `
        <div style="margin-bottom:12px;color:var(--text-muted)">${icon('zap', 40)}</div>
        <div style="font-size:16px;color:var(--text-secondary);margin-bottom:8px">No rules yet</div>
        <div style="font-size:13px;color:var(--text-muted)">Create rules to automatically tag, color, or organize incoming emails</div>
      `;
      listEl.appendChild(empty);
      return;
    }

    rules.sort((a, b) => a.priority - b.priority).forEach(rule => {
      listEl.appendChild(renderRuleCard(rule));
    });
    setupDragDrop();
  }

  function setupDragDrop() {
    let dragId: string | null = null;

    listEl.addEventListener('dragstart', (e: DragEvent) => {
      const target = e.target as HTMLElement;
      if (target.closest('button, input, select, label')) { e.preventDefault(); return; }
      const card = target.closest('.rule-card') as HTMLElement;
      if (!card) return;
      dragId = card.dataset.id || null;
      card.classList.add('drag-dragging');
      e.dataTransfer!.effectAllowed = 'move';
    });

    listEl.addEventListener('dragend', () => {
      listEl.querySelectorAll('.rule-card').forEach(c => {
        c.classList.remove('drag-dragging', 'drag-over');
      });
      dragId = null;
    });

    listEl.addEventListener('dragover', (e: DragEvent) => {
      e.preventDefault();
      e.dataTransfer!.dropEffect = 'move';
      const over = (e.target as HTMLElement).closest('.rule-card') as HTMLElement;
      listEl.querySelectorAll('.rule-card').forEach(c => c.classList.remove('drag-over'));
      if (over && over.dataset.id !== dragId) over.classList.add('drag-over');
    });

    listEl.addEventListener('drop', async (e: DragEvent) => {
      e.preventDefault();
      const over = (e.target as HTMLElement).closest('.rule-card') as HTMLElement;
      if (!over || !dragId || over.dataset.id === dragId) return;

      const cards = Array.from(listEl.querySelectorAll('.rule-card')) as HTMLElement[];
      const fromIdx = cards.findIndex(c => c.dataset.id === dragId);
      const toIdx = cards.findIndex(c => c === over);
      if (fromIdx < 0 || toIdx < 0) return;

      const dragged = cards[fromIdx];
      if (fromIdx < toIdx) {
        over.after(dragged);
      } else {
        over.before(dragged);
      }

      const newOrder = Array.from(listEl.querySelectorAll('.rule-card')) as HTMLElement[];
      try {
        await Promise.all(newOrder.map((c, i) => {
          if (c.dataset.id) return patchRule(c.dataset.id, { priority: i });
        }));
      } catch {
        showToast('Failed to save order', 'error');
      }
    });
  }

  function renderRuleCard(rule: Rule): HTMLElement {
    const card = el('div', 'rule-card');
    card.dataset.id = rule.id;
    card.draggable = true;

    const top = el('div', 'rule-card-top');

    const dragHandle = el('span', 'rule-card-drag');
    dragHandle.innerHTML = icon('grip-vertical', 16);
    dragHandle.title = 'Drag to reorder';

    const nameInfo = el('div', '');
    nameInfo.style.flex = '1';
    const name = el('div', 'rule-card-name', rule.name);
    if (rule.description) {
      const desc = el('div', 'rule-card-desc', rule.description);
      nameInfo.append(name, desc);
    } else {
      nameInfo.appendChild(name);
    }

    const toggle = document.createElement('label');
    toggle.className = 'toggle-switch';
    toggle.title = rule.enabled ? 'Disable rule' : 'Enable rule';
    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = rule.enabled;
    const track = el('span', 'toggle-track');
    toggle.append(checkbox, track);

    checkbox.addEventListener('change', async () => {
      try {
        await patchRule(rule.id, { enabled: checkbox.checked });
        showToast(`Rule "${rule.name}" ${checkbox.checked ? 'enabled' : 'disabled'}`, 'success');
      } catch {
        checkbox.checked = !checkbox.checked;
        showToast('Failed to update rule', 'error');
      }
    });

    top.append(dragHandle, nameInfo, toggle);
    card.appendChild(top);

    const summary = el('div', 'rule-card-summary');
    const condSummary = el('div', 'rule-card-summary-item');
    condSummary.textContent = `${rule.conditions.length} condition${rule.conditions.length !== 1 ? 's' : ''} (${rule.logic})`;
    const actSummary = el('div', 'rule-card-summary-item');
    actSummary.textContent = `${rule.actions.length} action${rule.actions.length !== 1 ? 's' : ''}`;
    const matchSummary = el('div', 'rule-card-summary-item');
    matchSummary.textContent = `${rule.stats.match_count} match${rule.stats.match_count !== 1 ? 'es' : ''}`;
    if (rule.stats.last_match_at) {
      const last = el('div', 'rule-card-summary-item');
      last.textContent = `Last match: ${formatDate(rule.stats.last_match_at)}`;
      summary.append(condSummary, actSummary, matchSummary, last);
    } else {
      summary.append(condSummary, actSummary, matchSummary);
    }
    card.appendChild(summary);

    const actions = el('div', 'rule-card-actions');
    const editBtn = el('button', 'btn btn-ghost btn-sm');
    editBtn.innerHTML = `${icon('pencil', 13)} Edit`;
    const deleteBtn = el('button', 'btn btn-danger btn-sm');
    deleteBtn.innerHTML = `${icon('trash-2', 13)} Delete`;

    editBtn.addEventListener('click', () => {
      openRuleModal(rule, async (updated) => {
        try {
          const saved = await updateRule(rule.id, updated);
          card.replaceWith(renderRuleCard(saved));
          showToast(`Rule "${saved.name}" updated`, 'success');
        } catch {
          showToast('Failed to update rule', 'error');
        }
      });
    });

    deleteBtn.addEventListener('click', async () => {
      if (!await confirm(`Delete rule "${rule.name}"?`, true)) return;
      try {
        await deleteRule(rule.id);
        card.remove();
        showToast(`Rule "${rule.name}" deleted`, 'success');
        if (listEl.querySelectorAll('.rule-card').length === 0) {
          renderRules([]);
        }
      } catch {
        showToast('Failed to delete rule', 'error');
      }
    });

    actions.append(editBtn, deleteBtn);
    card.appendChild(actions);

    return card;
  }

  newBtn.addEventListener('click', () => {
    openRuleModal(null, async (ruleData) => {
      try {
        const saved = await createRule(ruleData);
        const emptyEl = listEl.querySelector('.rules-empty');
        if (emptyEl) {
          listEl.innerHTML = '';
        }
        listEl.insertBefore(renderRuleCard(saved), listEl.firstChild);
        showToast(`Rule "${saved.name}" created`, 'success');
      } catch {
        showToast('Failed to create rule', 'error');
      }
    });
  });

  state.view.subscribe(v => {
    if (v === 'rules') loadRules();
  });
  if (state.view.value === 'rules') loadRules();

  return page;
}

type RuleSavePayload = Omit<Rule, 'id' | 'stats' | 'created_at' | 'updated_at'>;

function openRuleModal(rule: Rule | null, onSave: (r: RuleSavePayload) => Promise<void>): void {
  const overlay = el('div', 'modal-overlay');

  const modal = el('div', 'modal');
  modal.setAttribute('role', 'dialog');
  modal.setAttribute('aria-modal', 'true');

  const mHeader = el('div', 'modal-header');
  const mTitle = el('h2', 'modal-title', rule ? 'Edit Rule' : 'New Rule');
  const closeBtn = el('button', 'modal-close');
  closeBtn.innerHTML = icon('x', 16);
  mHeader.append(mTitle, closeBtn);

  const mBody = el('div', 'modal-body');

  const nameGroup = el('div', 'form-group');
  const nameLabel = el('label', 'form-label', 'Name');
  const nameInput = el('input', 'form-input');
  nameInput.type = 'text';
  nameInput.placeholder = 'Rule name...';
  nameInput.value = rule?.name || '';
  nameGroup.append(nameLabel, nameInput);

  const descGroup = el('div', 'form-group');
  const descLabel = el('label', 'form-label', 'Description (optional)');
  const descInput = el('input', 'form-input');
  descInput.type = 'text';
  descInput.placeholder = 'Brief description...';
  descInput.value = rule?.description || '';
  descGroup.append(descLabel, descInput);

  mBody.append(nameGroup, descGroup);

  const logicGroup = el('div', 'form-group');
  const logicLabel = el('label', 'form-label', 'Match logic');
  const logicToggle = el('div', 'logic-toggle');
  const andBtn = el('button', 'logic-toggle-btn', 'AND');
  const orBtn = el('button', 'logic-toggle-btn', 'OR');
  let logic: 'AND' | 'OR' = rule?.logic || 'AND';

  function setLogic(v: 'AND' | 'OR') {
    logic = v;
    andBtn.classList.toggle('active', v === 'AND');
    orBtn.classList.toggle('active', v === 'OR');
  }

  andBtn.addEventListener('click', () => setLogic('AND'));
  orBtn.addEventListener('click', () => setLogic('OR'));
  setLogic(logic);

  logicToggle.append(andBtn, orBtn);
  logicGroup.append(logicLabel, logicToggle);
  mBody.appendChild(logicGroup);

  const condTitle = el('div', 'form-section-title', 'Conditions');
  mBody.appendChild(condTitle);

  const condList = el('div', '');
  condList.id = 'condition-list';
  mBody.appendChild(condList);

  const addCondBtn = el('button', 'add-row-btn', '+ Add Condition');
  mBody.appendChild(addCondBtn);

  const actTitle = el('div', 'form-section-title', 'Actions');
  mBody.appendChild(actTitle);

  const actList = el('div', '');
  actList.id = 'action-list';
  mBody.appendChild(actList);

  const addActBtn = el('button', 'add-row-btn', '+ Add Action');
  mBody.appendChild(addActBtn);

  const mFooter = el('div', 'modal-footer');
  const testBtn = el('button', 'btn btn-ghost');
  testBtn.innerHTML = `${icon('play', 14)} Test`;
  const testResult = el('span', '', '');
  testResult.style.fontSize = '13px';
  testResult.style.color = 'var(--text-muted)';
  const spacer = el('div', 'modal-footer-spacer');
  const cancelBtn = el('button', 'btn btn-ghost', 'Cancel');
  const saveBtn = el('button', 'btn btn-primary', 'Save Rule');
  mFooter.append(testBtn, testResult, spacer, cancelBtn, saveBtn);

  modal.append(mHeader, mBody, mFooter);
  overlay.appendChild(modal);
  document.body.appendChild(overlay);

  const FIELDS = [
    { value: 'from', label: 'From' },
    { value: 'to', label: 'To' },
    { value: 'subject', label: 'Subject' },
    { value: 'body', label: 'Body' },
    { value: 'header', label: 'Header' },
    { value: 'tag', label: 'Tag' },
    { value: 'size', label: 'Size' },
    { value: 'has_attachment', label: 'Has Attachment' },
  ];

  const OPERATORS_TEXT = [
    { value: 'contains', label: 'contains' },
    { value: 'not_contains', label: 'not contains' },
    { value: 'equals', label: 'equals' },
    { value: 'not_equals', label: 'not equals' },
    { value: 'starts_with', label: 'starts with' },
    { value: 'ends_with', label: 'ends with' },
    { value: 'regex', label: 'matches regex' },
  ];

  const OPERATORS_NUM = [
    { value: 'gt', label: 'greater than' },
    { value: 'lt', label: 'less than' },
    { value: 'equals', label: 'equals' },
  ];

  const OPERATORS_BOOL = [
    { value: 'exists', label: 'exists' },
  ];

  function getOperators(field: string) {
    if (field === 'size') return OPERATORS_NUM;
    if (field === 'has_attachment') return OPERATORS_BOOL;
    return OPERATORS_TEXT;
  }

  function buildConditionRow(cond?: Condition): HTMLElement {
    const row = el('div', 'condition-row');

    const fieldSel = el('select', '');
    FIELDS.forEach(f => {
      const opt = document.createElement('option');
      opt.value = f.value;
      opt.textContent = f.label;
      if (cond?.field === f.value) opt.selected = true;
      fieldSel.appendChild(opt);
    });

    const opSel = el('select', '');
    const valueInput = el('input', '');
    valueInput.type = 'text';
    valueInput.placeholder = 'Value...';
    if (cond?.value) valueInput.value = cond.value;

    const headerKeyInput = el('input', '');
    headerKeyInput.type = 'text';
    headerKeyInput.placeholder = 'Header name...';
    headerKeyInput.style.display = 'none';
    if (cond?.header_key) headerKeyInput.value = cond.header_key;

    function updateOperators() {
      const ops = getOperators(fieldSel.value);
      opSel.innerHTML = '';
      ops.forEach(op => {
        const opt = document.createElement('option');
        opt.value = op.value;
        opt.textContent = op.label;
        if (cond?.operator === op.value) opt.selected = true;
        opSel.appendChild(opt);
      });
      headerKeyInput.style.display = fieldSel.value === 'header' ? '' : 'none';
      valueInput.style.display = fieldSel.value === 'has_attachment' ? 'none' : '';
    }

    fieldSel.addEventListener('change', updateOperators);
    updateOperators();

    const rmBtn = el('button', 'row-remove-btn');
    rmBtn.innerHTML = icon('x', 14);
    rmBtn.title = 'Remove condition';
    rmBtn.addEventListener('click', () => row.remove());

    row.append(fieldSel, opSel, headerKeyInput, valueInput, rmBtn);
    return row;
  }

  const ACTION_TYPES = [
    { value: 'tag', label: 'Add tag', placeholder: 'tag-name', needsValue: true },
    { value: 'remove_tag', label: 'Remove tag', placeholder: 'tag-name', needsValue: true },
    { value: 'color', label: 'Set color', placeholder: '#ff0000 or color name', needsValue: true },
    { value: 'mark_read', label: 'Mark as read', placeholder: '', needsValue: false },
    { value: 'star', label: 'Star email', placeholder: '', needsValue: false },
    { value: 'delete', label: 'Delete email', placeholder: '', needsValue: false },
    { value: 'webhook', label: 'Webhook POST', placeholder: 'https://...', needsValue: true },
    { value: 'folder', label: 'Move to Folder', placeholder: 'Folder name...', needsValue: true },
  ];

  function buildActionRow(act?: Action): HTMLElement {
    const row = el('div', 'action-row');

    const typeSel = el('select', '');
    ACTION_TYPES.forEach(t => {
      const opt = document.createElement('option');
      opt.value = t.value;
      opt.textContent = t.label;
      if (act?.type === t.value) opt.selected = true;
      typeSel.appendChild(opt);
    });

    const valueInput = el('input', '');
    valueInput.type = 'text';
    if (act?.value) valueInput.value = act.value;

    function updateValue() {
      const def = ACTION_TYPES.find(t => t.value === typeSel.value);
      if (def) {
        valueInput.placeholder = def.placeholder || '';
        valueInput.style.display = def.needsValue ? '' : 'none';
      }
    }
    typeSel.addEventListener('change', updateValue);
    updateValue();

    const rmBtn = el('button', 'row-remove-btn');
    rmBtn.innerHTML = icon('x', 14);
    rmBtn.title = 'Remove action';
    rmBtn.addEventListener('click', () => row.remove());

    row.append(typeSel, valueInput, rmBtn);
    return row;
  }

  if (rule?.conditions && rule.conditions.length > 0) {
    rule.conditions.forEach(c => condList.appendChild(buildConditionRow(c)));
  } else {
    condList.appendChild(buildConditionRow());
  }

  if (rule?.actions && rule.actions.length > 0) {
    rule.actions.forEach(a => actList.appendChild(buildActionRow(a)));
  } else {
    actList.appendChild(buildActionRow());
  }

  addCondBtn.addEventListener('click', () => condList.appendChild(buildConditionRow()));
  addActBtn.addEventListener('click', () => actList.appendChild(buildActionRow()));

  function collectData(): RuleSavePayload | null {
    const name = nameInput.value.trim();
    if (!name) {
      nameInput.focus();
      showToast('Rule name is required', 'error');
      return null;
    }

    const conditions: Condition[] = [];
    condList.querySelectorAll('.condition-row').forEach(row => {
      const fieldSel = row.querySelector('select:first-child') as HTMLSelectElement;
      const opSel = row.querySelector('select:nth-child(2)') as HTMLSelectElement;
      const headerKeyIn = row.querySelector('input:nth-child(3)') as HTMLInputElement;
      const valueIn = row.querySelector('input:nth-child(4)') as HTMLInputElement;

      const cond: Condition = {
        field: fieldSel.value as Condition['field'],
        operator: opSel.value as Condition['operator'],
        value: valueIn.style.display === 'none' ? '' : valueIn.value,
      };
      if (headerKeyIn.style.display !== 'none' && headerKeyIn.value) {
        cond.header_key = headerKeyIn.value;
      }
      conditions.push(cond);
    });

    const actions: Action[] = [];
    actList.querySelectorAll('.action-row').forEach(row => {
      const typeSel = row.querySelector('select') as HTMLSelectElement;
      const valueIn = row.querySelector('input') as HTMLInputElement;
      actions.push({
        type: typeSel.value as Action['type'],
        value: valueIn.style.display === 'none' ? '' : valueIn.value,
      });
    });

    if (actions.length === 0) {
      showToast('At least one action is required', 'error');
      return null;
    }

    return {
      name,
      description: descInput.value.trim(),
      enabled: rule?.enabled ?? true,
      priority: rule?.priority ?? 0,
      logic,
      conditions,
      actions,
    };
  }

  function close() {
    overlay.remove();
  }

  closeBtn.addEventListener('click', close);
  cancelBtn.addEventListener('click', close);

  overlay.addEventListener('click', (e) => {
    if (e.target === overlay) close();
  });

  const keyHandler = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      close();
      document.removeEventListener('keydown', keyHandler);
    }
  };
  document.addEventListener('keydown', keyHandler);

  if (rule) {
    testBtn.addEventListener('click', async () => {
      testBtn.textContent = 'Testing...';
      testBtn.setAttribute('disabled', '');
      try {
        const res = await testRule(rule.id);
        testResult.textContent = `Would match ${res.match_count} existing email${res.match_count !== 1 ? 's' : ''}`;
        testResult.style.color = res.match_count > 0 ? 'var(--green)' : 'var(--text-muted)';
      } catch {
        testResult.textContent = 'Test failed';
        testResult.style.color = 'var(--red)';
      } finally {
        testBtn.textContent = '🧪 Test';
        testBtn.removeAttribute('disabled');
      }
    });
  } else {
    testBtn.style.display = 'none';
  }

  saveBtn.addEventListener('click', async () => {
    const data = collectData();
    if (!data) return;
    saveBtn.textContent = 'Saving...';
    saveBtn.setAttribute('disabled', '');
    try {
      await onSave(data);
      close();
    } catch {
    } finally {
      saveBtn.textContent = 'Save Rule';
      saveBtn.removeAttribute('disabled');
    }
  });

  setTimeout(() => nameInput.focus(), 50);
}
