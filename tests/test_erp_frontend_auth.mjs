import assert from 'node:assert/strict';

const values = new Map();
globalThis.sessionStorage = {
  getItem: key => values.get(key) ?? null,
  setItem: (key, value) => values.set(key, value),
  removeItem: key => values.delete(key),
};

const { api } = await import('../frontend/apps/admin/src/services/api.js');
const { gestaoService } = await import('../frontend/apps/admin/src/services/gestaoService.js');

const seen = [];
globalThis.fetch = async (url, options) => {
  seen.push({ url, options });
  if (url.endsWith('/admin/profile')) {
    return { ok: true, json: async () => ({ role: 'user', username: 'operator' }) };
  }
  throw new Error(`Unexpected request: ${url}`);
};

api.setToken('operator-token');
assert.equal(await gestaoService.restoreErpSession(), null);
assert.equal(Boolean(api.getToken()), false, 'ordinary user token must be discarded');

globalThis.fetch = async (url, options) => {
  seen.push({ url, options });
  return { ok: true, json: async () => ({ role: 'gestor', username: 'legacy-manager', twoFactorEnabled: false }) };
};
api.setToken('manager-without-2fa');
assert.equal(await gestaoService.restoreErpSession(), null);
assert.equal(Boolean(api.getToken()), false, 'manager without 2FA must be discarded');

globalThis.fetch = async (url, options) => {
  seen.push({ url, options });
  if (url.endsWith('/admin/profile')) {
    return { ok: true, json: async () => ({ role: 'diretoria', username: 'director', twoFactorEnabled: true }) };
  }
  if (url.endsWith('/admin/auth/login')) {
    return { ok: true, json: async () => ({ data: { token: 'admin-token', gestor: { role: 'diretoria', username: 'director' } } }) };
  }
  throw new Error(`Unexpected request: ${url}`);
};

api.setToken('existing-admin-token');
assert.equal((await gestaoService.restoreErpSession()).role, 'diretoria');
const result = await gestaoService.loginErp(' director ', 'strong-password', 'external-pin', '123456');
assert.equal(result.success, true);
assert.equal(api.getToken(), 'admin-token');
const request = seen.findLast(item => item.url.endsWith('/admin/auth/login'));
assert.equal(request.options.method, 'POST');
assert.deepEqual(JSON.parse(request.options.body), {
  username: 'director', password: 'strong-password', pin: 'external-pin', twoFactorCode: '123456',
});
assert.equal(seen.some(item => item.url === '/api/v1/auth/login'), false);

console.log('ERP frontend auth PASS');
