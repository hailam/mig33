import assert from 'node:assert/strict';
import test from 'node:test';
import { NativeAlertCoordinator, retainSheetSnapshot } from '../src/ui/nativePresentationState.ts';

test('queued alert waits for real native false and a closed commit, without closing its successor', () => {
  const state = new NativeAlertCoordinator<{ key: string }>();
  const error = { key: 'error:1' }, notice = { key: 'notice:2' };
  state.setDesired(error); assert.equal(state.commit(), true);
  const errorGeneration = state.state.generation;
  assert.equal(state.choose(errorGeneration), error);
  state.setDesired(notice);
  assert.equal(state.commit(), false);
  assert.equal(state.state.phase, 'waiting-native');
  assert.equal(state.state.alert, error);
  assert.equal(state.nativeClosed(errorGeneration), null, 'action already closed its own episode');
  assert.equal(state.state.phase, 'closed', 'false must render before the queued true');
  assert.equal(state.commit(), true);
  assert.equal(state.state.alert, notice);
  assert.equal(state.state.phase, 'presented');
  const noticeState = state.state;
  assert.equal(state.nativeClosed(errorGeneration), null);
  assert.equal(state.choose(errorGeneration), null);
  assert.equal(state.state, noticeState, 'late old callbacks cannot close or act on the next alert');
});

test('a superseded alert action is ignored and the pending priority is presented after native dismissal', () => {
  const state = new NativeAlertCoordinator<{ key: string }>();
  state.setDesired({ key: 'confirmation:1' }); state.commit();
  const generation = state.state.generation;
  const replacement = { key: 'error:2' };
  state.setDesired(replacement);
  assert.equal(state.choose(generation), null);
  assert.equal(state.nativeClosed(generation), null);
  state.commit();
  assert.equal(state.state.alert, replacement);
});

test('native dismissal without an action closes once, and repeated logical episodes have distinct keys', () => {
  const state = new NativeAlertCoordinator<{ key: string }>();
  const first = { key: 'error:1' }, repeated = { key: 'error:2' };
  state.setDesired(first); state.commit();
  const generation = state.state.generation;
  assert.equal(state.nativeClosed(generation), first);
  assert.equal(state.nativeClosed(generation), null);
  state.commit();
  assert.equal(state.state.phase, 'idle', 'unchanged dismissed input must not reopen');
  state.setDesired(repeated); state.commit();
  assert.equal(state.state.alert, repeated);
});

test('programmatic removal commits false without waiting for a native event Expo deliberately suppresses', () => {
  const state = new NativeAlertCoordinator<{ key: string }>();
  state.setDesired({ key: 'error:1' }); state.commit();
  state.setDesired(null); state.commit();
  assert.equal(state.state.phase, 'closed');
  state.commit(); assert.equal(state.state.phase, 'idle');
});

test('sheet dismissal and editor replacement preserve the latest edited snapshot', () => {
  const opening = { key: 'roster:1', draft: '' };
  const edited = { key: 'roster:1', draft: 'A later edit' };
  let retained = retainSheetSnapshot(null, opening, opening);
  retained = retainSheetSnapshot(retained, opening, edited);
  assert.equal(retainSheetSnapshot(retained, opening, null), edited);
  const child = { key: 'roster:2', draft: 'New group' };
  assert.equal(retainSheetSnapshot(retained, opening, child), edited);
  assert.equal(retainSheetSnapshot(retained, child, child), child);
  assert.equal(retainSheetSnapshot(retained, null, null), null);
});
