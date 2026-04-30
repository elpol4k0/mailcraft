import { el } from './utils';
import { icon } from './icons';

export function confirm(message: string, danger = false): Promise<boolean> {
  return new Promise(resolve => {
    const overlay = el('div', 'modal-overlay');

    const modal = el('div', 'modal');
    modal.style.maxWidth = '380px';
    modal.setAttribute('role', 'alertdialog');
    modal.setAttribute('aria-modal', 'true');

    const body = el('div', '');
    body.style.cssText = 'padding:24px 24px 8px;display:flex;gap:12px;align-items:flex-start;';

    const iconWrap = el('div', '');
    iconWrap.style.cssText = `flex-shrink:0;color:${danger ? 'var(--red)' : 'var(--text-muted)'};margin-top:2px;`;
    iconWrap.innerHTML = icon(danger ? 'x-circle' : 'info', 20);

    const text = el('p', '');
    text.style.cssText = 'font-size:14px;color:var(--text-primary);line-height:1.5;margin:0;';
    text.textContent = message;

    body.append(iconWrap, text);

    const footer = el('div', 'modal-footer');
    footer.style.justifyContent = 'flex-end';

    const cancelBtn = el('button', 'btn btn-ghost btn-sm', 'Cancel');
    const confirmBtn = el('button', `btn btn-sm ${danger ? 'btn-danger' : 'btn-primary'}`, danger ? 'Delete' : 'Confirm');

    footer.append(cancelBtn, confirmBtn);
    modal.append(body, footer);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    confirmBtn.focus();

    const close = (result: boolean) => {
      overlay.remove();
      resolve(result);
    };

    cancelBtn.addEventListener('click', () => close(false));
    confirmBtn.addEventListener('click', () => close(true));
    overlay.addEventListener('click', e => { if (e.target === overlay) close(false); });
    overlay.addEventListener('keydown', e => {
      if (e.key === 'Escape') close(false);
      if (e.key === 'Enter') close(true);
    });
  });
}
