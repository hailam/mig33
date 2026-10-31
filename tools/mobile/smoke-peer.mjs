import { appendFile, writeFile } from 'node:fs/promises';
import { parseArgs } from 'node:util';
import { createServer } from 'node:http';
import { Mig33Client } from '../../apps/mobile/src/mig33/client.ts';
import { LegacyPages } from '../../apps/mobile/src/mig33/legacy-pages.ts';
import { HTTPTransport } from '../../apps/mobile/src/mig33/transport/http.ts';

const { values } = parseArgs({ options: {
  origin: { type: 'string' }, ready: { type: 'string' }, events: { type: 'string' },
  rooms: { type: 'boolean', default: false },
  features: { type: 'boolean', default: false },
  unread: { type: 'boolean', default: false },
  commands: { type: 'boolean', default: false },
  'control-port': { type: 'string' }, 'control-token': { type: 'string' },
} });
if (!values.origin || !values.ready || !values.events) throw new Error('Provide --origin, --ready and --events');
const origin = new URL(values.origin);
if (origin.hostname !== '127.0.0.1' || ['8080', '9119'].includes(origin.port)) throw new Error('Use an isolated loopback listener');
const events = values.events;
const client = new Mig33Client(new HTTPTransport(origin.href, { pollIntervalMs: 150 }));
const actors = new Map([['bob', client]]);
const groupIDs = new Set();
const receivedPrivate = [];
let control;
let replying = Promise.resolve();
const record = (event) => appendFile(events, JSON.stringify(event) + '\n');
let finish;
const stopped = new Promise((resolve) => { finish = resolve; });
process.once('SIGTERM', () => finish());
process.once('SIGINT', () => finish());
client.subscribe((event) => {
  if (values.features && event.type === 'group-created') groupIDs.add(event.id);
  if (values.features && event.type === 'message' && event.message.kind === 2) groupIDs.add(event.message.target);
  if (event.type !== 'message' || event.message.sender !== 'alice') return;
  const { body, kind, target } = event.message;
  if (values.commands && event.message.content === 6 && body === 'alice waves.' && [1, 3].includes(kind)) {
    // Bounded fixture evidence only: count the explicit native smoke actions
    // without replying or adding a control endpoint to the application.
    replying = replying.then(() => record({ event: 'command-action', kind, target, body }));
  }
  if (values.features && kind === 1) receivedPrivate.push(body);
  const room = values.rooms && kind === 3 && target === 'Lobby'
    && ['Hello room from iOS HTTPS', 'Hello room from iOS TLS'].includes(body);
  const privateChat = kind === 1 && ['Hello from iOS HTTPS', 'Hello from iOS TLS'].includes(body);
  const group = values.features && kind === 2 && ['Hello group from iOS HTTPS', 'Hello group from iOS TLS'].includes(body);
  if (!room && !privateChat && !group) return;
  replying = replying.then(async () => {
    await record({ event: 'received', sender: 'alice', kind, target, body });
    if (room) await client.sendRoomMessage(target, 'Room reply: ' + body);
    else if (group) await client.sendGroupMessage(target, 'Group reply: ' + body);
    else await client.sendMessage('alice', 'Received: ' + body);
    await record({ event: 'replied', kind, target, body });
  }).catch(async (error) => {
    await record({ event: 'error', message: String(error) });
    client.close();
    process.exitCode = 1;
    finish();
  });
});
await client.login({ username: 'bob', password: 'bob-demo', platform: 'isolated native UI smoke peer' });
await client.syncContacts();
if (values.rooms || values.features) {
  for (const name of ['Lobby', 'Gaming']) {
    const room = await client.joinRoom(name);
    await record({ event: 'room-joined', room: room.name });
  }
}
if (values.features || values.unread) {
  const controlPort = Number(values['control-port']);
  const controlToken = values['control-token'];
  if (!Number.isInteger(controlPort) || controlPort < 1024 || controlPort > 65535 || !controlToken
    || ['8080', '9119', origin.port].includes(String(controlPort))) throw new Error('Provide an isolated --control-port and --control-token');
  for (const [username, password] of values.features ? [['carol', 'carol-demo'], ['dave', 'dave-demo'], ['hailam', 'hailam']] : []) {
    const actor = new Mig33Client(new HTTPTransport(origin.href, { pollIntervalMs: 150 }));
    await actor.login({ username, password, platform: 'isolated native feature acceptance peer' });
    await actor.syncContacts();
    actors.set(username, actor);
  }
  async function createVoteRoom(name) {
    // Fixture setup uses the same original922 forms as the UI. Bob owns this
    // room, so the UI's Alice exercises regular-user706 voting, not moderator
    // immediate kick. No role grants, direct DB writes or product test API.
    const browser = new LegacyPages(client.requestOriginal, origin.href + 'sites/index.php', () => {});
    const checkedPage = () => {
      const state = browser.getSnapshot();
      if (state.error || !state.page || state.page.status !== 200) throw new Error(state.error || `Page failed: ${state.page?.status}`);
      return state.page;
    };
    await browser.open('/sites/index.php?c=chatroom&a=create&v=midlet');
    const nameForm = checkedPage().forms.find((form) => form.controls.some((field) => field.name === 'name'));
    if (!nameForm) throw new Error('Original room name form unavailable');
    const nameField = nameForm.controls.find((field) => field.name === 'name');
    await browser.submit(nameForm, { [nameField.id]: name });
    const detailsForm = checkedPage().forms.find((form) => form.controls.some((field) => field.name === 'description'));
    if (!detailsForm) throw new Error('Original room details form unavailable');
    const detailsField = detailsForm.controls.find((field) => field.name === 'description');
    await browser.submit(detailsForm, { [detailsField.id]: 'Isolated native vote acceptance room' });
    if (checkedPage().title !== 'Room Created') throw new Error('Original room creation did not complete');
    browser.close();
    for (const actor of actors.values()) await actor.joinRoom(name);
    await record({ event: 'feature-fixture-created', name, owner: 'bob' });
    return { room: name };
  }
  async function command(input) {
    const actor = actors.get(input.actor ?? 'bob');
    if (!actor) throw new Error('Unknown fixture actor');
    switch (input.action) {
      case 'create-vote': return createVoteRoom(input.room);
      case 'join': {
        const users = input.actors ?? [...actors.keys()];
        for (const username of users) {
          const peer = actors.get(username);
          if (!peer) throw new Error('Unknown fixture actor');
          await peer.joinRoom(input.room);
        }
        await record({ event: 'feature-joined', room: input.room, users });
        return { joined: users };
      }
      case 'room': return actor.getRoomParticipants(input.room);
      case 'room-send': {
        try {
          await actor.sendRoomMessage(input.room, input.body);
          await record({ event: 'feature-room-send', actor: input.actor ?? 'bob', room: input.room, accepted: true, body: input.body });
          return { accepted: true };
        } catch (error) {
          await record({ event: 'feature-room-send', actor: input.actor ?? 'bob', room: input.room, accepted: false, body: input.body });
          return { accepted: false, error: String(error) };
        }
      }
      case 'kick':
        await actor.moderateRoom(input.room, input.target, 'kick');
        await record({ event: 'feature-vote', actor: input.actor, room: input.room, target: input.target });
        return { accepted: true };
      case 'private-send':
        await actor.sendMessage('alice', input.body);
        await record({ event: 'feature-private-send', actor: input.actor ?? 'bob', body: input.body });
        return { accepted: true };
      case 'private-received': return { bodies: receivedPrivate };
      case 'photo-send': {
        // Use the backend's real image fixture, as the protocol integration
        // test does; a PNG signature alone is not a decodable photograph.
        const image = await actor.getEmoticon('[smile]');
        if (!image.image || !image.imageURI) throw new Error('Original image fixture is unavailable');
        await actor.sendPhoto(1, 'alice', image.image);
        await record({ event: 'feature-photo-send', actor: input.actor ?? 'bob', transport: input.transport });
        return { accepted: true };
      }
      case 'groups': return { groups: [...groupIDs] };
      case 'group': {
        const id = input.id ?? [...groupIDs].at(-1);
        if (!id) throw new Error('No original group conversation has been observed');
        return { id, ...(await actor.getGroupParticipants(id)) };
      }
      default: throw new Error('Unknown fixture command');
    }
  }
  control = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json');
    if (request.method !== 'POST' || request.url !== '/' || request.headers['x-smoke-token'] !== controlToken) {
      response.writeHead(403); response.end(JSON.stringify({ error: 'Forbidden' })); return;
    }
    try {
      let body = '';
      for await (const chunk of request) {
        body += chunk;
        if (body.length > 8192) throw new Error('Fixture request too large');
      }
      response.end(JSON.stringify(await command(JSON.parse(body))));
    } catch (error) { response.writeHead(500); response.end(JSON.stringify({ error: String(error) })); }
  });
  await new Promise((resolve, reject) => { control.once('error', reject); control.listen(controlPort, '127.0.0.1', resolve); });
}
await record({ event: 'ready' });
await writeFile(values.ready, 'ready\n');
await stopped;
await replying;
if (control) await new Promise((resolve) => control.close(resolve));
for (const actor of actors.values()) { try { await actor.logout(); } catch { actor.close(); } }
