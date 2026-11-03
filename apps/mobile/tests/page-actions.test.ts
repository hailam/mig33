import assert from 'node:assert/strict';
import test from 'node:test';
import { encodeLegacyForm, parseLegacyPage } from '../src/mig33/legacy-pages.ts';
import { canPullRefreshLegacyPage, legacySearchForm } from '../src/ui/pages/pageActions.ts';

const base = 'https://localhost:8443/sites/index.php';

test('Modern search affordances identify only verified search forms and retain their submit payload', () => {
  for (const [controller, label] of [['profile', 'Search new friends'], ['chatroom', 'Search rooms']]) {
    const url = `${base}?c=${controller}&a=search`;
    const form = parseLegacyPage('<form method="post"><input type="hidden" name="csrf" value="t">'
      + '<input name="query"><input type="submit" name="action" value="Search"></form>', url).forms[0];
    const action = legacySearchForm(form, url);
    assert.ok(action);
    assert.equal(action.label, label);
    const query = form.controls.find((control) => control.name === 'query')!;
    const submit = form.controls.find((control) => control.kind === 'submit')!;
    assert.equal(action.inputId, query.id);
    assert.equal(action.submitId, submit.id);
    assert.equal(encodeLegacyForm(form, { [action.inputId]: 'Room & 😀' }, url, action.submitId),
      encodeLegacyForm(form, { [query.id]: 'Room & 😀' }, url, submit.id));
    assert.equal(legacySearchForm({ ...form, method: 'GET' }, url), undefined);
    assert.equal(legacySearchForm({ ...form, action: `${base}?c=store&a=search` }, url), undefined);
    assert.equal(legacySearchForm({ ...form, controls: form.controls.map((control) => control.id === submit.id
      ? { ...control, value: 'Filter Language' } : control) }, url), undefined);
    assert.equal(legacySearchForm({ ...form, controls: form.controls.map((control) => control.id === submit.id
      ? { ...control, disabled: true } : control) }, url), undefined);
  }
});

test('page pull refresh cannot discard editable form controls, including disabled and unfamiliar fields', () => {
  assert.equal(canPullRefreshLegacyPage(null), false);
  assert.equal(canPullRefreshLegacyPage(parseLegacyPage('<p>Read only</p>', base)), true);
  assert.equal(canPullRefreshLegacyPage(parseLegacyPage('<form method="post"><input name="csrf" type="hidden" value="t">'
    + '<input type="submit" value="Unblock"></form>', base)), true);
  for (const field of ['<input name="query">', '<input name="query" disabled>', '<textarea name="body"></textarea>',
    '<input name="photo" type="file">', '<input name="agree" type="checkbox">',
    '<select name="choice"><option>A</option></select>', '<input name="unknown" type="date">']) {
    assert.equal(canPullRefreshLegacyPage(parseLegacyPage(`<form>${field}</form>`, base)), false, field);
  }
});
