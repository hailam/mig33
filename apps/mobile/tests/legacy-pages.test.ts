import assert from 'node:assert/strict';
import test from 'node:test';
import { LegacyPages, decodeEntities, encodeLegacyForm, formEntries, legacyText, parseLegacyPage,
  parseNativeAction, resolveLegacyURL, type LegacyForm, type OriginalRequest } from '../src/mig33/legacy-pages.ts';
import { Packet, encodeUTF8 } from '../src/mig33/protocol/index.ts';

const base = 'http://127.0.0.1:38080/sites/index.php';
const reportURL = base + '?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=Lobby';
const reportHTML = '<html><head><title>Report &amp; review</title></head><body><p>Reason</p><form method="post" action="' + reportURL + '">'
  + '<input type="hidden" name="csrf" value="server-secret"><input type="hidden" name="nonce" value="once">'
  + '<p>Reason</p><select name="category"><option value="spam">Spam</option><option value="harassment" selected>Harassment</option></select>'
  + '<p>Details</p><textarea name="details">Initial &lt;text&gt;</textarea><input type="submit" value="Submit Report"></form></body></html>';

function response(html: string, status = 200): Packet { return new Packet(923, 1).setInt(1, status).setBytes(2, encodeUTF8(html)); }
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { resolve, promise }; }
function control(form: LegacyForm, name: string) { const result = form.controls.find((value) => value.name === name); assert.ok(result); return result; }

test('legacy parser retains literal text, quoted attributes, native links and successful form controls', () => {
  const page = parseLegacyPage(reportHTML.replace('<body>', '<body><script>doEvil()</script><style>body{}</style><p title="a > b">A &amp; B &#x1f600;</p>'), reportURL);
  assert.equal(page.title, 'Report & review');
  assert.ok(legacyText(page.body).includes('A & B 😀'));
  assert.ok(!legacyText(page.body).includes('doEvil'));
  assert.equal(page.forms.length, 1);
  const form = page.forms[0];
  assert.equal(form.action, reportURL);
  assert.equal(control(form, 'csrf').value, 'server-secret');
  assert.equal(control(form, 'category').value, 'harassment');
  assert.deepEqual(control(form, 'category').options?.map((value) => value.label), ['Spam', 'Harassment']);
  assert.equal(control(form, 'details').label, 'Details');
  assert.equal(control(form, 'details').value, 'Initial <text>');
  assert.equal(decodeEntities('&#0; &#xD800; &bogus;'), '� � &bogus;');
});

test('native report serialization keeps CSRF fixed and preserves the unrestricted raw text tail', () => {
  const form = parseLegacyPage(reportHTML, reportURL).forms[0];
  const text = 'Literal & target=alice + = 😀';
  const body = encodeLegacyForm(form, { [control(form, 'csrf').id]: 'not-the-server-token', [control(form, 'details').id]: text }, reportURL);
  assert.equal(body, 'csrf=server-secret&nonce=once&category=harassment&details=Literal%20&%20target=alice%20+%20=%20😀');
  assert.throws(() => encodeLegacyForm(form, { [control(form, 'details').id]: '%20' }, reportURL), /cannot preserve/);
  assert.throws(() => encodeLegacyForm(form, { [control(form, 'details').id]: 'x'.repeat(8193) }, reportURL), /too large/);
});

test('native forms retain checkbox, radio, selected option and clicked submit semantics', () => {
  const html = '<form method=post><input name=csrf type=hidden value=token><input type=checkbox name=agree checked>'
    + '<input type=checkbox name=off><input type=radio name=choice value=a checked><input type=radio name=choice value=b>'
    + '<input name=disabled value=no disabled><input type=submit name=action value=save><input type=submit name=action value=delete></form>';
  const form = parseLegacyPage(html, base).forms[0];
  const submit = form.controls.find((value) => value.kind === 'submit' && value.value === 'save')!;
  assert.deepEqual(formEntries(form, {}, submit.id), [['csrf', 'token'], ['agree', 'on'], ['choice', 'a'], ['action', 'save']]);
  assert.deepEqual(formEntries(form, { [control(form, 'agree').id]: false }, submit.id), [['csrf', 'token'], ['choice', 'a'], ['action', 'save']]);
});

test('original page routes use the corresponding Go raw-tail field, not percent-encoded free text', () => {
  for (const [query, name] of [
    ['c=chatroom&a=search', 'query'], ['c=chatroom&a=create&stage=details', 'description'],
    ['c=chatroom&a=setup&manage=1', 'body'], ['c=profile&a=edit', 'value'],
    ['c=group&a=home&op=post', 'body'], ['c=store&a=home&op=purchase', 'note'],
    ['c=settings&a=change_password', 'password'], ['c=photo&a=home', 'body'],
  ]) {
    const url = base + '?' + query;
    const form = parseLegacyPage(`<form method=post><input name=csrf type=hidden value=t><textarea name=${name}></textarea></form>`, url).forms[0];
    assert.equal(encodeLegacyForm(form, { [control(form, name).id]: 'A&B + 😀' }, url), `csrf=t&${name}=A&B%20+%20😀`);
  }
  const url = base + '?c=settings&a=block_list';
  const form = parseLegacyPage('<form method=post><input name=username value="alice+bob"><input name=csrf type=hidden value=t></form>', url).forms[0];
  assert.equal(encodeLegacyForm(form, {}, url), 'username=alice%2Bbob&csrf=t');
});

test('page URLs stay on the authenticated origin and native commands remain literal non-code data', () => {
  assert.equal(resolveLegacyURL('?c=profile&a=home', base), base + '?c=profile&a=home');
  assert.equal(resolveLegacyURL('/assets/photos/1', base), 'http://127.0.0.1:38080/assets/photos/1');
  for (const address of ['https://other.invalid/path', '//other.invalid/path', 'javascript:alert(1)', 'http://name@127.0.0.1:38080/sites/']) {
    assert.throws(() => resolveLegacyURL(address, base));
  }
  assert.deepEqual(parseNativeAction('mig33:joinChatroom(Lobby%20One)'), { name: 'joinChatroom', argument: 'Lobby%20One' });
  assert.equal(parseNativeAction('/sites/index.php'), undefined);
  assert.throws(() => parseNativeAction('mig33:privateChat(alice)\n'));
});

test('922 controller fetches pages and forms on the same session; back never replays a POST', async () => {
  const packets: Packet[] = [];
  const request: OriginalRequest = async (opcode, configure, replies, complete) => {
    const packet = new Packet(opcode, packets.length + 1); configure(packet); packets.push(packet);
    assert.deepEqual(replies, [923]); assert.equal(complete, 923);
    return [response(packet.byte(1) === 2 ? '<title>Saved</title><body><p>Saved successfully</p></body>' : reportHTML)];
  };
  const actions: unknown[] = [];
  const browser = new LegacyPages(request, base, (action) => { actions.push(action); });
  await browser.open(reportURL);
  assert.equal(packets[0].opcode, 922); assert.equal(packets[0].int(4), 1); assert.equal(packets[0].text(7), '');
  const form = browser.getSnapshot().page!.forms[0];
  await browser.submit(form, { [control(form, 'details').id]: 'Report & details' });
  assert.equal(packets[1].byte(1), 2); assert.match(packets[1].text(3)!, /&details=Report%20&%20details$/);
  assert.equal(browser.getSnapshot().page!.title, 'Saved');
  assert.equal(browser.back(), true); assert.equal(packets.length, 2); assert.equal(browser.getSnapshot().page!.title, 'Report & review');
  await browser.refresh(); assert.equal(packets.at(-1)!.byte(1), 1);
  await browser.follow('mig33:privateChat(bob)'); assert.deepEqual(actions, [{ name: 'privateChat', argument: 'bob' }]);
  const before = packets.length;
  await browser.submit(form, {}); assert.equal(packets.length, before); assert.match(browser.getSnapshot().error, /no longer current/);
});

test('late page replies cannot replace newer navigation or reopen a closed session', async () => {
  const requests: ReturnType<typeof deferred<Packet[]>>[] = [];
  const request: OriginalRequest = async () => { const next = deferred<Packet[]>(); requests.push(next); return next.promise; };
  const browser = new LegacyPages(request, base, () => {});
  const first = browser.open('?c=profile&a=home');
  const second = browser.open('?c=store&a=home');
  requests[1].resolve([response('<title>Store</title>')]); await second;
  requests[0].resolve([response('<title>Old profile</title>')]); await first;
  assert.equal(browser.getSnapshot().page!.title, 'Store');
  const late = browser.open('?c=chatroom&a=search');
  browser.close(); requests[2].resolve([response('<title>Late search</title>')]); await late;
  assert.equal(browser.getSnapshot().page, null);
  assert.equal(browser.getSnapshot().loading, false);
});

test('pending form submission prevents page links and native actions from replacing its result', async () => {
  const pending = deferred<Packet[]>();
  const packets: Packet[] = [];
  const actions: unknown[] = [];
  const browser = new LegacyPages(async (opcode, configure) => {
    const packet = new Packet(opcode, packets.length + 1); configure(packet); packets.push(packet);
    return packet.byte(1) === 2 ? pending.promise : [response(reportHTML)];
  }, base, (action) => { actions.push(action); });
  await browser.open(reportURL);
  const form = browser.getSnapshot().page!.forms[0];
  const submission = browser.submit(form, { [control(form, 'details').id]: 'Send this report once.' });
  assert.equal(browser.getSnapshot().loading, true);

  await browser.follow('?c=profile&a=home');
  await browser.follow('mig33:newBrowser(?c=store&a=home)');
  await browser.follow('mig33:privateChat(bob)');
  await browser.follow('mig33:uploadPhotoToScrapbook()');
  await browser.submit(form, {});
  assert.equal(packets.length, 2, 'Only the initial GET and the in-flight POST should reach the server');
  assert.deepEqual(actions, []);

  pending.resolve([response('<title>Report saved</title><p>Thank you</p>')]);
  await submission;
  assert.equal(browser.getSnapshot().page!.title, 'Report saved');
  assert.equal(browser.getSnapshot().loading, false);
  await browser.follow('mig33:privateChat(bob)');
  assert.deepEqual(actions, [{ name: 'privateChat', argument: 'bob' }]);
  await browser.follow('?c=profile&a=home');
  assert.equal(packets.length, 3, 'Page navigation resumes after the submitted result arrives');
});

test('authenticated page images use922 purpose2 and unsupported image data is rejected', async () => {
  const packets: Packet[] = [];
  const request: OriginalRequest = async (opcode, configure) => {
    const packet = new Packet(opcode, 1); configure(packet); packets.push(packet);
    return [new Packet(923, 1).setInt(1, 200).setBytes(2, Uint8Array.of(137, 80, 78, 71, 13, 10, 26, 10))];
  };
  const browser = new LegacyPages(request, base, () => {});
  assert.equal(await browser.loadImage('/assets/photos/1'), 'data:image/png;base64,iVBORw0KGgo=');
  assert.equal(packets[0].int(4), 2);
  await browser.loadImage('/assets/photos/1'); assert.equal(packets.length, 1);
  browser.close(); await browser.loadImage('/assets/photos/1'); assert.equal(packets.length, 2);
});

test('non-200 legacy responses retain their server explanation', async () => {
  const browser = new LegacyPages(async () => [response('Join this conversation before reporting.', 403)], base, () => {});
  await browser.open(reportURL);
  assert.equal(browser.getSnapshot().page!.status, 403);
  assert.equal(legacyText(browser.getSnapshot().page!.body), 'Join this conversation before reporting.');
});
