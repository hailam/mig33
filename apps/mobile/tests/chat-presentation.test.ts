import assert from 'node:assert/strict';
import test from 'node:test';
import type { Contact } from '../src/mig33/client.ts';
import { Packet } from '../src/mig33/protocol/index.ts';
import { inviteContacts, modernGroupTitle } from '../src/ui/session/model.ts';

function contact(username: string, presence: number, display = username): Contact {
  return { id: 1, groupId: -1, username, presence, display, avatar: '', status: '', packet: new Packet(402, 0) };
}
const participants = (...names: string[]) => ({ participants: names.map((username) => ({ username, moderator: false, muted: false })) });

test('Modern invite list includes offline native friends without importing external identities', () => {
  const phoneOnly = contact('', 99, 'Phone contact');
  phoneOnly.packet.setText(8, '123456789');
  const external = contact('', 1, 'External identity');
  external.packet.setText(30, 'external-bob').setByte(31, 2);
  const contacts = [contact('alice', 1), contact('bob', 99), contact('carol', 3), external,
    contact('dave', 27), phoneOnly, contact('BOB', 99, 'Bobby'), contact('erin', 4)];

  assert.deepEqual(inviteContacts(contacts, [' ALICE ', 'CAROL'], true).map((entry) => [entry.username, entry.presence]),
    [['BOB', 99], ['dave', 27], ['erin', 4]]);
  assert.deepEqual(inviteContacts(contacts, ['alice', 'carol'], false).map((entry) => entry.username), ['erin']);
  assert.equal(contacts.length, 8, 'presentation filtering preserves the shared roster');
});

test('Modern group titles identify known participants in server order and exclude the signed-in user', () => {
  const group = participants('CAROL', 'Alice', 'bob', ' BOB ', '', 'dave', 'erin');
  assert.equal(modernGroupTitle(group, ' ALICE ', [contact('bob', 99, 'Bobby'), contact('carol', 1, 'Carol')]),
    'Carol, Bobby +2');
  assert.equal(group.participants.length, 7, 'a display label never changes membership');
});

test('Modern group titles fall back without inventing members or replacing group identifiers', () => {
  assert.equal(modernGroupTitle(undefined, 'alice', []), 'Group Chat');
  assert.equal(modernGroupTitle(participants('', ' Alice '), 'alice', []), 'Group Chat');
  assert.equal(modernGroupTitle(participants('bob', 'carol'), 'alice', [contact('bob', 1, '  ')]), 'bob, carol');
  assert.notEqual(modernGroupTitle(participants('bob', 'carol'), 'alice', []),
    modernGroupTitle(participants('bob', 'dave'), 'alice', []));
});
