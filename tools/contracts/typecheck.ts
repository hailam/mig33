import {
  messageDescriptors,
  type GatewayRequestID,
  type LocalJobID,
  type ObservedFieldViews,
  type PacketFor,
  type PacketProjection,
  type ResourceResultID,
} from '../../apps/mobile/src/mig33/generated/contracts';

// Client code retains raw bytes, unknown tags and present-empty fields.
const login: PacketFor<'Client200'> = {
  opcode: 200,
  transaction: -32768,
  fields: new Map([[32767, new Uint8Array()]]),
};
const projection: PacketProjection<200> = { opcode: 200, transaction: 1, fields: { '5': '' } };
const loginViews: ObservedFieldViews<'Server203'> = {
  '31': { I: 65538, S: 1 },
  '32': { I: 65538, T: '\u0000\u0001\u0000\u0002' },
};
const timestamp: ObservedFieldViews<'Client419'> = { '1': { L: 9223372036854775807n } };
const gateway: GatewayRequestID = 'Client200';
const local: LocalJobID = 'Client1001';
const resource: ResourceResultID = 'Server1002';
const opcode: 200 = messageDescriptors.Client200.opcode;

// These fail if generation broadens the API or silently loses precision/views.
// @ts-expect-error Login packets must retain their opcode discriminator.
const wrongOpcode: PacketFor<'Client200'> = { opcode: 202, transaction: 1, fields: new Map() };
// @ts-expect-error Java long values are bigint, never imprecise JS numbers.
const imprecise: ObservedFieldViews<'Client419'> = { '1': { L: 123 } };
// @ts-expect-error A presence-only byte test is not a byte value interpretation.
const presenceAsValue: ObservedFieldViews<'Server203'> = { '33': { B: 1 } };
// @ts-expect-error Local jobs are not requests sent through the gateway.
const localAsGateway: GatewayRequestID = 'Client1001';
// @ts-expect-error Resource downloads are not client gateway operations.
const resourceAsGateway: GatewayRequestID = 'Server1002';
void [login, projection, loginViews, timestamp, gateway, local, resource, opcode,
  wrongOpcode, imprecise, presenceAsValue, localAsGateway, resourceAsGateway];
