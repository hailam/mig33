import assert from 'node:assert/strict';
import test from 'node:test';
import { Mig33Client } from '../src/mig33/client.ts';
import { LegacyPages, legacyText, type LegacyForm, type LegacyPage, type LegacyNode } from '../src/mig33/legacy-pages.ts';
import { HTTPTransport } from '../src/mig33/transport/http.ts';
import { TCPTransport } from '../src/mig33/transport/tcp.ts';
import { nodeConnector } from './helpers/node-socket.ts';
import { startServer } from './helpers/server.ts';

function page(browser: LegacyPages, status = 200): LegacyPage {
  const state = browser.getSnapshot();
  assert.equal(state.error, ''); assert.equal(state.loading, false); assert.ok(state.page);
  assert.equal(state.page.status, status, legacyText(state.page.body));
  return state.page;
}
function form(browser: LegacyPages, predicate: (form: LegacyForm) => boolean = () => true): LegacyForm {
  const found = page(browser).forms.find(predicate); assert.ok(found, 'Expected server-rendered form'); return found;
}
function has(form: LegacyForm, name: string, value?: string): boolean {
  return form.controls.some((control) => control.name === name && (value === undefined || control.value === value));
}
async function submit(browser: LegacyPages, selected: LegacyForm, values: Record<string, string> = {}): Promise<void> {
  const entries = Object.fromEntries(Object.entries(values).map(([name, value]) => {
    const control = selected.controls.find((control) => control.name === name);
    assert.ok(control, `Expected ${name} control`); return [control.id, value];
  }));
  await browser.submit(selected, entries, selected.controls.find((control) => control.kind === 'submit')?.id);
  page(browser);
}
function nodes(node: LegacyNode): LegacyNode[] { return [node, ...node.children.flatMap(nodes)]; }

for (const kind of ['http', 'tcp'] as const) {
  test(`original922 pages over ${kind} preserve session auth, forms, CSRF, room rights and native navigation`,
    { skip: !process.env.MIG33_TEST_SERVER_BINARY, timeout: 60000 }, async (t) => {
      const server = await startServer(); t.after(() => server.close());
      const client = new Mig33Client(kind === 'http' ? new HTTPTransport(server.httpURL, { pollIntervalMs: 100 })
        : new TCPTransport(nodeConnector('127.0.0.1', server.tcpPort)));
      t.after(() => client.close());
      let siteURL = '';
      client.subscribe((event) => { if (event.type === 'packet' && event.packet.opcode === 203) siteURL = event.packet.text(26) ?? ''; });
      await client.login({ username: 'alice', password: 'alice-demo' });
      assert.equal(siteURL, server.httpURL + '/sites/index.php');
      const actions: unknown[] = [];
      const browser = new LegacyPages(client.requestOriginal, siteURL, (action) => { actions.push(action); });
      t.after(() => browser.close());
      const url = (controller: string, action: string, more = '') => `${siteURL}?c=${controller}&a=${action}&v=midlet${more}`;

      await browser.open(url('profile', 'home', '&username=bob'));
      assert.match(legacyText(page(browser).body), /bob/);
      const privateLink = nodes(page(browser).body).find((node) => node.attributes.href === 'mig33:privateChat(bob)');
      assert.ok(privateLink); await browser.follow(privateLink.attributes.href);
      assert.deepEqual(actions, [{ name: 'privateChat', argument: 'bob' }]);
      await browser.open(url('profile', 'edit'));
      const statusText = 'Original & literal + = 😀';
      await submit(browser, form(browser, (item) => item.action.includes('field=status')), { value: statusText });
      assert.match(legacyText(page(browser).body), /Status message saved/);
      await browser.open(url('profile', 'home'));
      assert.ok(legacyText(page(browser).body).includes(statusText));

      await browser.open(url('chatroom', 'search'));
      await submit(browser, form(browser), { query: 'Lobby' });
      assert.ok(nodes(page(browser).body).some((node) => node.attributes.href === 'mig33:joinChatroom(Lobby)'));
      const room = `Pages ${kind}`;
      await browser.open(url('chatroom', 'create'));
      await submit(browser, form(browser), { name: room });
      assert.ok(has(form(browser), 'binding') && has(form(browser), 'csrf'));
      await submit(browser, form(browser), { description: 'Native description & + 😀' });
      const settings = url('chatroom', 'setup', '&manage=1&roomName=' + encodeURIComponent(room));
      await browser.open(settings);
      assert.ok(legacyText(page(browser).body).includes('Native description & + 😀'));
      await submit(browser, form(browser, (item) => has(item, 'op', 'description')), { body: 'Edited & + = 😀' });
      assert.ok(legacyText(page(browser).body).includes('Edited & + = 😀'));
      // Create/link through the original pages, then consume the original701
      // Community Group ID. No fixture-only API assigns this association.
      await browser.open(url('group', 'home'));
      await submit(browser, form(browser, (item) => has(item, 'private', '0')), { name: `Club ${kind}` });
      await submit(browser, form(browser), { description: 'Community & + 😀' });
      await browser.open(url('group', 'add_to_group', '&roomName=' + encodeURIComponent(room)));
      const link = form(browser);
      const communityID = Number(new URL(link.action).searchParams.get('cid')); assert.ok(communityID > 0);
      await submit(browser, link);
      assert.equal((await client.joinRoom(room)).communityId, communityID);
      await browser.open(url('group', 'list_chatrooms', '&cid=' + communityID));
      await submit(browser, form(browser, (item) => item.action.includes('op=unlink')));
      assert.equal((await client.joinRoom(room)).communityId, undefined, 'rejoin clears an unlinked community');
      await client.leaveRoom(room);
      await browser.open(settings);
      await submit(browser, form(browser, (item) => has(item, 'op', 'moderator-add')), { body: 'bob' });
      await submit(browser, form(browser, (item) => has(item, 'op', 'moderator-remove')), { body: 'bob' });
      await submit(browser, form(browser, (item) => has(item, 'op', 'ban')), { body: 'bob' });
      assert.match(legacyText(page(browser).body), /Banned users: bob/);
      await submit(browser, form(browser, (item) => has(item, 'op', 'unban')), { body: 'bob' });
      assert.ok(!legacyText(page(browser).body).includes('Banned users: bob'));
      await submit(browser, form(browser, (item) => has(item, 'op', 'owner')), { body: 'bob' });
      assert.match(legacyText(page(browser).body), /Ownership transferred to bob/);
      await browser.open(settings); page(browser, 403);

      const report = url('report', 'report_abuse', '&type=4&subject=Chatroom&subject_id=Lobby');
      await browser.open(report); page(browser, 403);
      await client.joinRoom('Lobby'); await browser.open(report);
      await submit(browser, form(browser), { category: 'other', details: 'Test report & + = 😀' });
      assert.match(legacyText(page(browser).body), /submitted|received|Report #/i);
      await client.leaveRoom('Lobby');

      await client.blockUser('bob'); await browser.open(url('settings', 'block_list'));
      await submit(browser, form(browser, (item) => has(item, 'username', 'bob')));
      assert.match(legacyText(page(browser).body), /bob has been unblocked/);
      await browser.open(url('account', 'recharge_credit'));
      await submit(browser, form(browser, (item) => has(item, 'amount', '10000')));
      assert.match(legacyText(page(browser).body), /No payment was taken/);
      await browser.open(url('store', 'home', '&ty=1&username=bob&item=rose'));
      await submit(browser, form(browser, (item) => has(item, 'item', 'rose') && has(item, 'nonce')), { note: 'Gift & + = 😀' });
      assert.match(legacyText(page(browser).body), /delivered to bob/);
      const smile = await client.getEmoticon('[smile]'); assert.ok(smile.image);
      const photosURL = await client.uploadPhoto(smile.image); assert.ok(photosURL);
      await browser.open(photosURL);
      const thumbnail = nodes(page(browser).body).find((node) => node.tag === 'img'); assert.ok(thumbnail);
      assert.match(await browser.loadImage(thumbnail.attributes.src), /^data:image\/png;base64,/);
      const fullPhoto = new URL(thumbnail.attributes.src); fullPhoto.search = '';
      assert.match(await browser.loadImage(fullPhoto.href), /^data:image\/png;base64,/);
      for (const [controller, action] of [['group', 'home'], ['group', 'list_my_groups'], ['account', 'home'],
        ['account', 'get_history'], ['photo', 'home'], ['help', 'main']]) {
        await browser.open(url(controller, action)); page(browser);
      }
      await client.logout();
      t.diagnostic(`Legacy page transport run: ${server.outputDirectory}`);
    });
}
