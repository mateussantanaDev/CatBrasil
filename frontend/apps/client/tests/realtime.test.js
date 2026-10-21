import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { parseRealtimeMessage } from '../src/services/realtimeContract.js';
import { realtimeToast } from '../src/services/realtimeNotifications.js';
import { notificationService } from '../src/services/notificationService.js';
import { websocketService } from '../src/services/websocketService.js';
import { ws } from '../src/services/ws.js';

const originalNotification = notificationService.notifyPixConfirmed;
const sockets = [];
class FakeWebSocket {
  static OPEN = 1;
  static CONNECTING = 0;
  readyState = FakeWebSocket.CONNECTING;
  constructor(url, protocols) {
    this.url = url;
    this.protocols = protocols;
    sockets.push(this);
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
  receive(message) {
    this.onmessage?.({ data: JSON.stringify(message) });
  }
}

globalThis.WebSocket = FakeWebSocket;
globalThis.location = { protocol: 'https:', host: 'catbrasil.test' };
globalThis.localStorage = { getItem: () => 'test-token' };

afterEach(() => {
  websocketService.disconnect();
  ws.disconnect();
  notificationService.notifyPixConfirmed = originalNotification;
  sockets.length = 0;
});

test('contrato WS aceita event/payload e rejeita formato antigo ou inválido', () => {
  assert.deepEqual(parseRealtimeMessage('{"event":"balance_updated","payload":{"newBalance":120.5},"userId":"u-1"}'), {
    event: 'balance_updated', payload: { newBalance: 120.5 }, userId: 'u-1'
  });
  assert.equal(parseRealtimeMessage('{"type":"balance_updated","newBalance":120.5}'), null);
  assert.equal(parseRealtimeMessage('{"event":"pix_confirmed","payload":null}'), null);
  assert.equal(parseRealtimeMessage('not json'), null);
});

test('notificações usam payload canônico de saldo, PIX e saque', () => {
  assert.match(realtimeToast({ event: 'balance_updated', payload: { newBalance: 120.5 } }).message, /120\.50/);
  assert.match(realtimeToast({ event: 'pix_confirmed', payload: { paidAmount: 50, outcome: 'applied' } }).message, /50\.00/);
  assert.match(realtimeToast({ event: 'withdraw_paid', payload: { paidAmount: 75, outcome: 'applied' } }).message, /75\.00/);
  assert.match(realtimeToast({ event: 'withdraw_rejected', payload: { status: 'cancelado' } }).message, /voltou ao saldo/);
  assert.equal(realtimeToast({ event: 'pix_confirmed', payload: { paidAmount: 50, outcome: 'duplicate' } }), null);
  assert.equal(realtimeToast({ event: 'withdraw_paid', payload: { paidAmount: 75, outcome: 'queued' } }), null);
  assert.equal(realtimeToast({ event: 'balance_updated', payload: {} }), null);
});

test('cliente principal despacha envelope e notifica PIX aplicado uma vez', () => {
  const received = [];
  const notifications = [];
  notificationService.notifyPixConfirmed = (amount) => notifications.push(amount);
  websocketService.connect('u-1', (message) => received.push(message));
  assert.equal(sockets[0].url, 'wss://catbrasil.test/api/v1/ws');
  assert.deepEqual(sockets[0].protocols, ['bearer', 'test-token']);
  sockets[0].receive({ event: 'pix_confirmed', payload: { paidAmount: 50, outcome: 'applied' }, userId: 'u-1' });
  sockets[0].receive({ event: 'withdraw_paid', payload: { paidAmount: 75, outcome: 'applied' }, userId: 'u-1' });
  sockets[0].receive({ event: 'balance_updated', payload: { newBalance: 120.5 }, userId: 'u-1' });
  sockets[0].receive({ event: 'pix_confirmed', payload: { paidAmount: 50, outcome: 'duplicate' }, userId: 'u-1' });
  sockets[0].receive({ type: 'pix_confirmed', paidAmount: 999 });
  assert.deepEqual(received.map((message) => message.event), ['pix_confirmed', 'withdraw_paid', 'balance_updated', 'pix_confirmed']);
  assert.deepEqual(notifications, [50]);
});

test('cliente de estado entrega somente payload no evento de saldo', () => {
  const balances = [];
  const unsubscribe = ws.on('balance_updated', (payload) => balances.push(payload.newBalance));
  ws.connect('test-token');
  sockets[0].receive({ event: 'balance_updated', payload: { newBalance: 99.25 }, userId: 'u-1' });
  sockets[0].receive({ type: 'balance_updated', newBalance: 999 });
  assert.deepEqual(balances, [99.25]);
  unsubscribe();
});
