import type { LegacyForm, LegacyPage } from '../../mig33/legacy-pages';

/** Refresh performs the existing GET. Limit the gesture to pages with no
 * editable form controls, so a downward drag cannot discard an unsent draft. */
export function canPullRefreshLegacyPage(page: LegacyPage | null): boolean {
  return page !== null && page.forms.every((form) => form.controls.every((control) =>
    ['hidden', 'submit', 'button', 'reset'].includes(control.kind)));
}

/** Only the two verified original search forms receive Modern search affordances.
 * Return their exact control IDs so keyboard and button submit remain identical. */
export function legacySearchForm(form: LegacyForm, pageURL: string): {
  inputId: number; submitId: number; label: string;
} | undefined {
  let route: URL;
  try { route = new URL(form.action, pageURL); } catch { return undefined; }
  const controller = route.searchParams.get('c');
  if (form.method !== 'POST' || route.searchParams.get('a') !== 'search'
    || (controller !== 'profile' && controller !== 'chatroom')) return undefined;
  const visible = form.controls.filter((control) => control.kind !== 'hidden');
  const input = visible.find((control) => ['text', 'search'].includes(control.kind) && control.name === 'query');
  const submit = visible.find((control) => control.kind === 'submit' && control.value.trim().toLowerCase() === 'search');
  if (visible.length !== 2 || !input || !submit || submit.disabled) return undefined;
  return { inputId: input.id, submitId: submit.id, label: controller === 'chatroom' ? 'Search rooms' : 'Search new friends' };
}
