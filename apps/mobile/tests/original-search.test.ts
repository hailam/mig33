import assert from 'node:assert/strict';
import test from 'node:test';
import { OriginalSearch, originalSearchForm, originalSearchResults } from '../src/mig33/original-search.ts';
import { LegacyPages, parseLegacyPage, type OriginalRequest } from '../src/mig33/legacy-pages.ts';
import { Packet, encodeUTF8 } from '../src/mig33/protocol/index.ts';
import { Mig33Client } from '../src/mig33/client.ts';
import { HTTPTransport } from '../src/mig33/transport/http.ts';
import { TCPTransport } from '../src/mig33/transport/tcp.ts';
import { nodeConnector } from './helpers/node-socket.ts';
import { startServer } from './helpers/server.ts';

const site = 'https://localhost:8443/sites/index.php';
const roomURL = `${site}?c=chatroom&a=search&v=midlet`;
const peopleURL = `${site}?c=profile&a=search&v=midlet`;
function form(action: string, label = 'Search') {
  return `<form method="post" action="${action}"><input name="query"><input type="submit" value="${label}"></form>`;
}
// Deliberately reorder the repeated query controls: native and HTML ordering
// must not accidentally select the language or keyword form.
const roomForms = form(`${roomURL}&field=language`, 'Filter Language')
  + form(`${roomURL}&field=query&language=&keywords=`) + form(`${roomURL}&field=keywords`, 'Filter Keywords');
function room(name: string) {
  return `<p><b>${name}</b></p><p>People: 2 / 50</p><p><a href="mig33:joinChatroom(${name})">Join Room</a></p>`
    + `<p><a href="?c=chatroom&a=setup&v=midlet&roomName=${encodeURIComponent(name)}">Room Info</a></p>`;
}
function person(username: string, title: string) {
  return `<p><b>${title}</b> (${username})</p><p><a href="?c=profile&a=home&v=midlet&username=${username}">View Profile</a></p>`
    + `<p><a href="mig33:privateChat(${username})">Chat</a></p><p><a href="mig33:inviteFriend(${username})">Add Friend</a></p>`;
}
function response(html: string, status = 200) { return [new Packet(923, 1).setInt(1, status).setBytes(2, encodeUTF8(html))]; }
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { resolve, promise }; }

test('original search selects the query form by verified route and controls, never repeated field order', () => {
  const selected = originalSearchForm(parseLegacyPage(roomForms, roomURL), site, 'rooms');
  assert.equal(new URL(selected.form.action).searchParams.get('field'), 'query');
  assert.equal(selected.input.name, 'query');
  assert.equal(selected.submit.value, 'Search');
  for (const html of [form(`${roomURL}&field=language`), roomForms + form(`${roomURL}&field=query`),
    form(roomURL.replace('v=midlet', 'v=other')), form('https://foreign.invalid/sites/index.php?c=chatroom&a=search&v=midlet')]) {
    assert.throws(() => originalSearchForm(parseLegacyPage(html, roomURL), site, 'rooms'));
  }
});

test('original result projection preserves server ordering, literal identities and all safe actions', () => {
  const rooms = originalSearchResults(parseLegacyPage(roomForms + room('Remote Two') + room('Remote One'), roomURL), site, 'rooms');
  assert.deepEqual(rooms.map((entry) => entry.title), ['Remote Two', 'Remote One']);
  assert.equal(rooms[0].detail, 'People: 2 / 50');
  assert.deepEqual(rooms[0].actions, [
    { label: 'Join Room', href: 'mig33:joinChatroom(Remote Two)' },
    { label: 'Room Info', href: `${site}?c=chatroom&a=setup&v=midlet&roomName=Remote%20Two` },
  ]);
  const people = originalSearchResults(parseLegacyPage(form(peopleURL) + person('bob', 'Bobby &amp; Co'), peopleURL), site, 'people');
  assert.deepEqual(people[0], { key: 'people:bob', title: 'Bobby & Co', detail: 'bob', actions: [
    { label: 'View Profile', href: `${site}?c=profile&a=home&v=midlet&username=bob` },
    { label: 'Chat', href: 'mig33:privateChat(bob)' }, { label: 'Add Friend', href: 'mig33:inviteFriend(bob)' },
  ] });
  assert.equal('presence' in people[0], false);
  assert.deepEqual(originalSearchResults(parseLegacyPage('<p>No friends found.</p>', peopleURL), site, 'people'), []);
  assert.deepEqual(originalSearchResults(parseLegacyPage('<p>No chat rooms found.</p>', roomURL), site, 'rooms'), []);
  for (const html of ['<p>Unrecognized response</p>', room('Lobby').replace('mig33:joinChatroom(Lobby)', 'mig33:bad\nlink'),
    person('bob', 'Bob').replace('?c=profile', 'https://other.invalid/?c=profile'),
    person('bob', 'Bob').replace('inviteFriend(bob)', 'inviteFriend(alice)')]) {
    assert.throws(() => originalSearchResults(parseLegacyPage(html, peopleURL), site, 'people'));
  }
});

test('independent search submits original922 query bytes and leaves another browser history untouched', async () => {
  const packets: Packet[] = [];
  const request: OriginalRequest = async (opcode, configure) => {
    const packet = new Packet(opcode, packets.length + 1); configure(packet); packets.push(packet);
    if (packet.text(2)!.includes('a=home')) return response('<p>Profile</p>');
    return response(packet.byte(1) === 1 ? roomForms : room('Remote Room'));
  };
  const other = new LegacyPages(request, site, () => {});
  await other.open(`${site}?c=profile&a=home&v=midlet&username=bob`);
  const before = other.getSnapshot();
  const search = new OriginalSearch(request, site, 'rooms');
  await search.search('A&B + 😀');
  assert.equal(search.getSnapshot().error, '');
  assert.equal(search.getSnapshot().results[0].title, 'Remote Room');
  assert.equal(search.getSnapshot().query, 'A&B + 😀');
  assert.deepEqual(packets.slice(1).map((packet) => [packet.opcode, packet.byte(1), packet.int(4)]), [[922, 1, 1], [922, 2, 1]]);
  assert.equal(packets[2].text(3), 'query=A&B%20+%20😀');
  assert.equal(new URL(packets[2].text(2)!).searchParams.get('field'), 'query');
  assert.equal(other.getSnapshot(), before);
  other.close(); search.close();
});

test('new queries and clear discard late search replies without closing the chat request source', async () => {
  const first = deferred<Packet[]>();
  const cleared = deferred<Packet[]>();
  let posts = 0;
  const request: OriginalRequest = async (opcode, configure) => {
    const packet = new Packet(opcode, 1); configure(packet);
    if (packet.byte(1) === 1) return response(roomForms);
    posts++;
    if (packet.text(3) === 'query=first') return first.promise;
    if (packet.text(3) === 'query=cleared') return cleared.promise;
    return response(room('Second'));
  };
  const search = new OriginalSearch(request, site, 'rooms');
  const pendingFirst = search.search('first');
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(posts, 1);
  await search.search('second');
  first.resolve(response(room('First'))); await pendingFirst;
  assert.equal(search.getSnapshot().query, 'second');
  assert.equal(search.getSnapshot().results[0].title, 'Second');
  const pendingClear = search.search('cleared');
  await new Promise<void>((resolve) => setImmediate(resolve));
  search.clear(); cleared.resolve(response(room('Cleared'))); await pendingClear;
  assert.equal(search.getSnapshot().searched, false);
  assert.deepEqual(search.getSnapshot().results, []);
  await search.search('second');
  assert.equal(search.getSnapshot().results[0].title, 'Second');
  search.close();
});

test('room filters bind through the original fetched form, including filter-only searches and Unicode', async () => {
  const packets: Packet[] = [];
  const request: OriginalRequest = async (opcode, configure) => {
    const packet = new Packet(opcode, packets.length + 1); configure(packet); packets.push(packet);
    if (packet.byte(1) === 1) {
      const action = new URL(packet.text(2)!);
      action.searchParams.set('field', 'query');
      return response(form(action.href));
    }
    return response(room('Remote Room'));
  };
  const search = new OriginalSearch(request, site, 'rooms');
  await search.search('   ', { language: ' English ', keywords: ' island & café + 😀 ' });
  const state = search.getSnapshot();
  assert.equal(state.error, '');
  assert.equal(state.query, '');
  assert.equal(state.language, 'English');
  assert.equal(state.keywords, 'island & café + 😀');
  for (const packet of packets) {
    const route = new URL(packet.text(2)!);
    assert.equal(route.searchParams.get('language'), 'English');
    assert.equal(route.searchParams.get('keywords'), 'island & café + 😀');
  }
  assert.equal(packets[1].text(3), 'query=');
  search.clear();
  assert.equal(search.getSnapshot().language, '');
  assert.equal(search.getSnapshot().keywords, '');
  const people = new OriginalSearch(async (opcode, configure) => {
    const packet = new Packet(opcode, 1); configure(packet);
    if (packet.byte(1) === 1) {
      assert.equal(new URL(packet.text(2)!).searchParams.has('language'), false);
      return response(form(peopleURL));
    }
    assert.equal(packet.text(3), 'query=bob');
    return response(person('bob', 'Bob'));
  }, site, 'people');
  await people.search(' bob ', { language: 'Ignored', keywords: 'Ignored' });
  assert.equal(people.getSnapshot().query, 'bob');
  assert.equal(people.getSnapshot().language, '');
  assert.equal(people.getSnapshot().keywords, '');
  search.close(); people.close();
});

test('server errors and unknown result markup remain visible errors rather than false empty searches', async () => {
  for (const [html, status, expected] of [
    ['<p>Search Rooms</p><p>Search unavailable for this account.</p>', 403, /unavailable for this account/],
    ['<p>Changed search response.</p>', 200, /unfamiliar search response/],
  ] as const) {
    const request: OriginalRequest = async (opcode, configure) => {
      const packet = new Packet(opcode, 1); configure(packet);
      return packet.byte(1) === 1 ? response(roomForms) : response(html, status);
    };
    const search = new OriginalSearch(request, site, 'rooms');
    await search.search('Lobby');
    assert.match(search.getSnapshot().error, expected);
    assert.equal(search.getSnapshot().loading, false);
    search.close();
  }
});

test('the original room cap explanation is retained only when the server sends it', async () => {
  const notice = 'Showing up to 100 rooms. Narrow your search to find a room.';
  const hundredRooms = Array.from({ length: 100 }, (_, index) => room(`Room ${index + 1}`)).join('');
  let includeNotice = true;
  const request: OriginalRequest = async (opcode, configure) => {
    const packet = new Packet(opcode, 1); configure(packet);
    return response(packet.byte(1) === 1 ? roomForms
      : `<p><b>Search Rooms</b></p>${includeNotice ? `<p>${notice}</p>` : ''}${hundredRooms}`);
  };
  const search = new OriginalSearch(request, site, 'rooms');
  await search.search('Room');
  assert.equal(search.getSnapshot().error, '');
  assert.equal(search.getSnapshot().results.length, 100);
  assert.equal(search.getSnapshot().notice, notice);
  includeNotice = false;
  await search.search('Room');
  assert.equal(search.getSnapshot().results.length, 100);
  assert.equal(search.getSnapshot().notice, '', 'a row count alone cannot establish a server limit');
  search.clear();
  assert.equal(search.getSnapshot().notice, '');
});

for (const kind of ['http', 'tcp'] as const) {
  test(`inline original search over ${kind} finds people outside the roster and rooms never loaded locally`,
    { skip: !process.env.MIG33_TEST_SERVER_BINARY, timeout: 45000 }, async (t) => {
      const server = await startServer(); t.after(() => server.close());
      const client = new Mig33Client(kind === 'http' ? new HTTPTransport(server.httpURL, { pollIntervalMs: 100 })
        : new TCPTransport(nodeConnector('127.0.0.1', server.tcpPort)));
      t.after(() => client.close());
      let siteURL = '';
      client.subscribe((event) => { if (event.type === 'packet' && event.packet.opcode === 203) siteURL = event.packet.text(26) ?? ''; });
      await client.login({ username: 'hailam', password: 'hailam' });
      assert.equal((await client.syncContacts()).length, 0, 'fixture address book must start empty');
      const people = new OriginalSearch(client.requestOriginal, siteURL, 'people');
      const rooms = new OriginalSearch(client.requestOriginal, siteURL, 'rooms');
      t.after(() => { people.close(); rooms.close(); });
      await people.search('BO');
      assert.equal(people.getSnapshot().error, '');
      assert.deepEqual(people.getSnapshot().results.map((entry) => entry.key), ['people:bob']);
      assert.deepEqual(people.getSnapshot().results[0].actions.map((entry) => entry.label), ['View Profile', 'Chat', 'Add Friend']);
      assert.equal(client.contacts.length, 0, 'search must not add anyone to the roster');
      // No category/list opcode has been requested; this must reach server data.
      await rooms.search('Lobby');
      assert.equal(rooms.getSnapshot().error, '');
      assert.deepEqual(rooms.getSnapshot().results.map((entry) => entry.title), ['Lobby']);
      assert.deepEqual(rooms.getSnapshot().results[0].actions.map((entry) => entry.label), ['Join Room', 'Room Info']);
      await rooms.search('Missing room 92817');
      assert.equal(rooms.getSnapshot().error, '');
      assert.deepEqual(rooms.getSnapshot().results, []);
      await people.search('Nobody92817');
      assert.equal(people.getSnapshot().error, '');
      assert.deepEqual(people.getSnapshot().results, []);
    });
}
