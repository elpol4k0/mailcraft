export function requestNotificationPermission(): void {
  if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission();
  }
}

export function notifyNewEmail(from: string, subject: string): void {
  if (!('Notification' in window) || Notification.permission !== 'granted') return;

  const sender = from.replace(/<[^>]+>/, '').trim() || from;
  new Notification(`New email from ${sender}`, {
    body: subject || '(no subject)',
    tag: 'mailcraft-new-email',
    silent: false,
  });
}
