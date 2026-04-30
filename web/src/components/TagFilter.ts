import { state, clearFilters } from '../state';
import { el } from '../utils';

export function createTagFilter(): HTMLElement {
  const bar = el('div', 'filter-chips-bar');
  bar.id = 'filter-chips-bar';

  function render() {
    bar.innerHTML = '';

    const filters: Array<{ label: string; key: string }> = [];

    if (state.search.value) {
      filters.push({ label: `Search: "${state.search.value}"`, key: 'q' });
    }
    if (state.filterTag.value) {
      filters.push({ label: `#${state.filterTag.value}`, key: 'tag' });
    }
    if (state.filterRead.value === true) {
      filters.push({ label: 'Read', key: 'read' });
    }
    if (state.filterRead.value === false) {
      filters.push({ label: 'Unread', key: 'read' });
    }
    if (state.filterStarred.value === true) {
      filters.push({ label: 'Starred', key: 'starred' });
    }
    if (state.filterFolder.value) {
      filters.push({ label: `Folder: ${state.filterFolder.value}`, key: 'folder' });
    }

    if (filters.length === 0) {
      bar.style.display = 'none';
      return;
    }

    bar.style.display = 'flex';

    filters.forEach(f => {
      const chip = el('div', 'filter-chip');
      chip.textContent = f.label;

      const removeBtn = el('button', 'filter-chip-remove', '×');
      removeBtn.title = 'Remove filter';
      removeBtn.addEventListener('click', () => {
        if (f.key === 'q') state.search.set('');
        if (f.key === 'tag') state.filterTag.set(null);
        if (f.key === 'read') state.filterRead.set(null);
        if (f.key === 'starred') state.filterStarred.set(null);
        if (f.key === 'folder') state.filterFolder.set(null);
      });

      chip.appendChild(removeBtn);
      bar.appendChild(chip);
    });

    if (filters.length > 1) {
      const clearBtn = el('button', 'filter-chips-clear', 'Clear all');
      clearBtn.addEventListener('click', clearFilters);
      bar.appendChild(clearBtn);
    }
  }

  state.search.subscribe(render);
  state.filterTag.subscribe(render);
  state.filterRead.subscribe(render);
  state.filterStarred.subscribe(render);
  state.filterFolder.subscribe(render);
  render();

  return bar;
}
