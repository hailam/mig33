import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { Parser } from '@asyncapi/parser';
import Ajv from 'ajv';
import { parse } from 'yaml';

const root = new URL('../../', import.meta.url);
const read = (path) => readFile(new URL(path, root), 'utf8');
const source = await read('api/asyncapi.yaml');
const spec = parse(source);
const { document, diagnostics } = await new Parser().parse(source);
const errors = diagnostics.filter((diagnostic) => diagnostic.severity === 0);
assert.ok(document && errors.length === 0,
  errors.map(({ message, path }) => `${path.join('.')}: ${message}`).join('\n'));
assert.equal(spec.asyncapi, '3.0.0');

// AsyncAPI extensions are annotations, not JSON Schema validation keywords.
const ajv = new Ajv({ allErrors: true, strict: false });
const schemaID = 'urn:mig33:v4:inspection';
ajv.addSchema({ $id: schemaID, components: { schemas: spec.components.schemas } });
const validators = new Map(Object.entries(spec.components.messages).map(([id, message]) => [
  id, ajv.compile({ $ref: schemaID + message.payload.$ref }),
]));
const validate = (id, value) => {
  const check = validators.get(id);
  assert.ok(check, `Unknown message ${id}`);
  assert.ok(check(value), `${id}: ${ajv.errorsText(check.errors)}`);
};
const reject = (id, value) => assert.equal(validators.get(id)(value), false,
  `${id} accepted an invalid inspection projection: ${JSON.stringify(value)}`);

// Decode only the framing required to project original serializer fixtures.
// This is an independent verification helper, not an application codec.
function project(hex) {
  assert.match(hex, /^(?:[0-9a-f]{2})+$/);
  const bytes = Buffer.from(hex, 'hex');
  assert.ok(bytes.length >= 9);
  assert.equal(bytes[0], 2);
  assert.equal(bytes.readInt32BE(5), bytes.length - 9);
  const fields = {};
  for (let offset = 9; offset < bytes.length;) {
    assert.ok(offset + 6 <= bytes.length, 'Truncated field header');
    const tag = bytes.readInt16BE(offset);
    const length = bytes.readInt32BE(offset + 2);
    assert.ok(length >= 0 && offset + 6 + length <= bytes.length, 'Invalid field length');
    fields[tag] = bytes.subarray(offset + 6, offset + 6 + length).toString('hex');
    offset += 6 + length;
  }
  return { opcode: bytes.readInt16BE(1), transaction: bytes.readInt16BE(3), fields };
}

const fixtures = JSON.parse(await read('internal/protocol/testdata/original_v4_packets.json')).packets;
const projections = new Map(fixtures.map((fixture) => {
  const value = project(fixture.original_java_hex);
  assert.equal(value.opcode, fixture.opcode);
  assert.equal(value.transaction, fixture.transaction);
  return [fixture.name, value];
}));
const exercised = new Set();
let exampleCount = 0;
for (const [id, message] of Object.entries(spec.components.messages)) {
  for (const example of message.examples ?? []) {
    validate(id, example.payload);
    const fixture = projections.get(example.name);
    assert.ok(fixture, `${id}: example ${example.name} must identify an original fixture`);
    assert.deepEqual(example.payload, fixture, `${id}: projection differs from original bytes`);
    exercised.add(example.name);
    exampleCount++;
  }
}
assert.deepEqual([...exercised].sort(), [...projections.keys()].sort(), 'Every original fixture needs coverage');

// Schema boundaries: retain unknown fields, empty bytes and signed short limits.
const ping = { opcode: 2, transaction: 1, fields: {} };
for (const transaction of [-32768, 0, 32767]) validate('Client2', { ...ping, transaction });
validate('Client2', { ...ping, fields: { '-32768': '', '32767': '00ff' } });
for (const fields of [{ '32768': '' }, { '-32769': '' }, { '01': '' }, { '-0': '' },
  { '1': '0' }, { '1': 'zz' }, { '1': null }, { arbitrary: '' }]) {
  reject('Client2', { ...ping, fields });
}
for (const transaction of [-32769, 32768, 0.5, '1']) reject('Client2', { ...ping, transaction });
reject('Client2', { ...ping, opcode: 3 });
reject('Client2', { opcode: 2, fields: {} });
reject('Client2', { ...ping, invented: true });

// The same raw field supports several interpretations; no accessor view becomes
// a compulsory wire type or an inferred server-required field.
validate('Server203', { opcode: 203, transaction: 1, fields: { '31': '00010002', '32': '00010002' } });
validate('Client907', { opcode: 907, transaction: 1, fields: { '30000': 'deadbeef' } });
validate('Server1002', { opcode: 1002, transaction: 1, fields: { '1': '656e', '4': '6672' } });

for (const [channelID, channel] of Object.entries(spec.channels)) {
  const ids = Object.keys(channel.messages);
  const opcodes = ids.map((id) => spec.components.messages[id]['x-mig33-observation'].opcode);
  assert.equal(new Set(opcodes).size, ids.length, `${channelID}: ambiguous same-opcode messages`);
  for (const id of ids) {
    const message = spec.components.messages[id];
    const observation = message['x-mig33-observation'];
    const transport = channelID.startsWith('tcp') ? 'tcp' : 'http-tunnel';
    assert.ok(observation.delivery.includes(transport), `${channelID}: non-gateway ${id}`);
    assert.equal(observation.direction, channel['x-mig33-direction']);
    const empty = { opcode: observation.opcode, transaction: 0, fields: {} };
    assert.equal(ids.filter((candidate) => validators.get(candidate)(empty)).length, 1,
      `${channelID}: projection must match exactly one message`);
  }
}
for (const operation of Object.values(spec.operations)) {
  const channel = spec.channels[operation.channel.$ref.split('/').at(-1)];
  assert.equal(operation.action, channel['x-mig33-direction'] === 'client-to-server' ? 'receive' : 'send');
}
console.log(`AsyncAPI valid: ${validators.size} messages, ${Object.keys(spec.channels).length} channels; ` +
  `${exampleCount} examples cover all ${fixtures.length} original fixtures; projection boundary checks pass.`);
if (diagnostics.length) console.log(`AsyncAPI recommended-rule diagnostics: ${diagnostics.length} (no errors).`);
