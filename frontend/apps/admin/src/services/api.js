const API_BASE_URL = '/api/v1';
let currentToken = (typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('catbrasil_admin_token') : '') || '';

async function request(endpoint, options = {}) {
    const token = options.token || currentToken;
    const headers = {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        ...(options.headers || {})
    };

    const config = {
        method: options.method || 'GET',
        headers,
        ...(options.body ? { body: JSON.stringify(options.body) } : {})
    };

    try {
        const res = await fetch(`${API_BASE_URL}${endpoint}`, config);
        const data = await res.json().catch(() => ({}));
        if (!res.ok) {
            if (res.status === 401 && token && endpoint !== '/admin/auth/login' && endpoint !== '/admin/auth/logout' && typeof window !== 'undefined') {
                window.dispatchEvent(new CustomEvent('admin:unauthorized'));
            }
            const error = new Error(data.error || data.message || `Erro do Servidor Backend (${res.status})`);
            error.errorCode = data.errorCode;
            error.status = res.status;
            throw error;
        }
        return data;
    } catch (err) {
        console.error(`[API Error] ${endpoint}:`, err.message);
        throw err;
    }
}

function getDeviceFingerprint() {
    try {
        const nav = window.navigator || {};
        const screen = window.screen || {};
        const str = `${nav.userAgent || ''}-${nav.language || ''}-${screen.width || 0}x${screen.height || 0}`;
        let hash = 0;
        for (let i = 0; i < str.length; i++) {
            hash = ((hash << 5) - hash) + str.charCodeAt(i);
            hash |= 0;
        }
        return `FP-${Math.abs(hash)}`;
    } catch (e) {
        return 'FP-GENERIC';
    }
}

export const api = {
    setToken(token) {
        currentToken = token;
        if (typeof sessionStorage !== 'undefined') {
            if (token) {
                sessionStorage.setItem('catbrasil_admin_token', token);
            } else {
                sessionStorage.removeItem('catbrasil_admin_token');
            }
        }
    },

    getToken() {
        return currentToken || (typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('catbrasil_admin_token') : '');
    },

    async get(endpoint, options = {}) {
        return request(endpoint, { ...options, method: 'GET' });
    },

    async post(endpoint, body, options = {}) {
        return request(endpoint, { ...options, method: 'POST', body });
    },

    async put(endpoint, body, options = {}) {
        return request(endpoint, { ...options, method: 'PUT', body });
    },

    async patch(endpoint, body, options = {}) {
        return request(endpoint, { ...options, method: 'PATCH', body });
    },

    async delete(endpoint, options = {}) {
        return request(endpoint, { ...options, method: 'DELETE' });
    },

    // Autenticação Real no Servidor Go com Fingerprinting
    async login(username, password) {
        const fingerprint = getDeviceFingerprint();
        const data = await request('/auth/login', {
            method: 'POST',
            body: { username, password, fingerprint }
        });
        if (data.token) {
            this.setToken(data.token);
        }
        const user = await this.getProfile(data.token);
        return { token: data.token, user, message: data.message };
    },

    // Autenticação de Gestão ERP
    async loginErp(username, password, pin = '', twoFactorCode = '') {
        this.setToken('');
        const data = await request('/admin/auth/login', {
            method: 'POST',
            body: {
                username: (username || '').trim(),
                password,
                pin: (pin || '').trim(),
                twoFactorCode: (twoFactorCode || '').trim()
            }
        });
        const token = data?.data?.token;
        const role = data?.data?.gestor?.role;
        if (!token || !['admin', 'gestor', 'diretoria'].includes(role)) {
            throw new Error('Acesso de gestão não autorizado.');
        }
        this.setToken(token);
        return data;
    },

    async logoutErp() {
        try {
            await request('/admin/auth/logout', { method: 'POST' });
        } catch (e) {
            // ignore
        }
        this.setToken('');
    },

    async register(username, password, inviteCode) {
        const fingerprint = getDeviceFingerprint();
        const data = await request('/auth/register', {
            method: 'POST',
            body: { username, password, inviteCode, fingerprint }
        });
        if (data.token) {
            this.setToken(data.token);
        }
        const user = await this.getProfile(data.token);
        return { token: data.token, user, message: data.message };
    },

    // Perfil do Usuário
    async getProfile(token) {
        return await request('/user', { token });
    },

    async saveSecurityPassword(token, password) {
        return await request('/user/security-password', {
            method: 'POST',
            token,
            body: { password }
        });
    },

    async levelUp(token) {
        return await request('/user/level-up', {
            method: 'POST',
            token
        });
    },

    // Garagem & Máquinas
    async getGarage(token) {
        const res = await request('/garage', { token });
        return Array.isArray(res) ? res : (res?.data || []);
    },

    async collectYield(token, machineId) {
        return await request('/garage/collect', {
            method: 'POST',
            token,
            body: { machineId }
        });
    },

    // Loja (Catálogo de Máquinas e Aluguel)
    async getStoreMachines() {
        const res = await request('/shop/catalog');
        return Array.isArray(res) ? res : (res?.data || []);
    },

    async rentMachine(token, machineModel) {
        let pending;
        try { pending = JSON.parse(sessionStorage.getItem('cb_pending_rent') || 'null'); } catch { pending = null; }
        if (!pending || pending.machineModel !== machineModel) {
            pending = { machineModel, key: crypto.randomUUID() };
            sessionStorage.setItem('cb_pending_rent', JSON.stringify(pending));
        }
        const data = await request('/shop/rent', {
            method: 'POST',
            token,
            headers: { 'Idempotency-Key': pending.key },
            body: { machineId: machineModel }
        });
        sessionStorage.removeItem('cb_pending_rent');
        return {
            newMachine: data.newMachine,
            newBalance: data.balance,
            xp: data.xp,
            tratorRented: data.tratorRented,
            message: data.message
        };
    },

    // Depósito PIX (Segurança: Pendente até confirmação do gateway/webhook)
    async generatePix(token, amount) {
        return await request('/pix/generate', {
            method: 'POST',
            token,
            body: { amount }
        });
    },

    // Saque PIX (Segurança: Validação de Senha & Saldo no Servidor)
    async requestWithdrawal(token, amount, pixKey, securityPassword) {
        const intent = JSON.stringify({ amount, pixKey });
        let pending;
        try { pending = JSON.parse(sessionStorage.getItem('cb_pending_withdrawal') || 'null'); } catch { pending = null; }
        if (!pending || pending.intent !== intent) {
            pending = { intent, key: crypto.randomUUID() };
            sessionStorage.setItem('cb_pending_withdrawal', JSON.stringify(pending));
        }
        const data = await request('/transactions/withdraw', {
            method: 'POST',
            token,
            headers: { 'Idempotency-Key': pending.key },
            body: { amount, pixKey, password: securityPassword }
        });
        sessionStorage.removeItem('cb_pending_withdrawal');
        return { newBalance: data.newBalance, fee: data.fee, status: data.status, message: data.message };
    },

    // Histórico de Transações e Rede
    async getTransactions(token) {
        const res = await request('/transactions', { token });
        return Array.isArray(res) ? res : (res?.data || []);
    },

    async getReferrals(token) {
        const res = await request('/referrals', { token });
        return Array.isArray(res) ? res : (res?.data || []);
    },

    // Ranking Semanal da Corrida de Mineração
    async getWeeklyRanking(token) {
        const res = await request('/ranking/weekly', { token });
        return res?.data || res || [];
    },

    // Comunicados Ativos para Clientes
    async getActiveAnnouncements() {
        const res = await request('/announcements/active');
        return Array.isArray(res) ? res : (res?.data || []);
    },

    // Roleta Diária & Oficina de Peças
    async spinRoulette(token, mode = 'random') {
        return await request('/roulette/spin', {
            method: 'POST',
            token,
            body: { mode }
        });
    },

    async getRouletteConfig() {
        const res = await request('/roulette/config');
        return Array.isArray(res) ? res : (res?.data || []);
    },

    async assembleMachineFromParts(token) {
        return await request('/parts/assemble', {
            method: 'POST',
            token
        });
    },

    async resetRoulette(token) {
        return await request('/roulette/reset', {
            method: 'POST',
            token
        });
    },

    // ==========================================
    // PAINEL DE GESTÃO ERP (ADMIN / GESTOR)
    // ==========================================
    async getAdminUsers(query = '', status = '', limit = 100, offset = 0) {
        const params = new URLSearchParams();
        if (query) params.append('query', query);
        if (status) params.append('status', status);
        params.append('limit', limit);
        params.append('offset', offset);

        const res = await request(`/admin/users?${params.toString()}`);
        return res?.data || (Array.isArray(res) ? res : []);
    },

    async adjustUserBalance(userId, amount, type = 'add', reason = '') {
        const intent = JSON.stringify({ userId, amount, type, reason });
        let pending;
        try { pending = JSON.parse(sessionStorage.getItem('cb_pending_adjust') || 'null'); } catch { pending = null; }
        if (!pending || pending.intent !== intent) {
            pending = { intent, key: crypto.randomUUID() };
            sessionStorage.setItem('cb_pending_adjust', JSON.stringify(pending));
        }
        const result = await request(`/admin/users/${userId}/adjust-balance`, {
            method: 'POST',
            headers: { 'Idempotency-Key': pending.key },
            body: { amount, type, reason }
        });
        sessionStorage.removeItem('cb_pending_adjust');
        return result;
    },

    async updateUserLevel(userId, level) {
        return await request(`/admin/users/${userId}/level`, {
            method: 'PUT',
            body: { level: parseInt(level) }
        });
    },

    async toggleWithdrawLock(userId) {
        return await request(`/admin/users/${userId}/toggle-withdraw-lock`, {
            method: 'PATCH'
        });
    },

    async toggleCommissionLock(userId) {
        return await request(`/admin/users/${userId}/toggle-commission-lock`, {
            method: 'PATCH'
        });
    },

    async injectMachineBonus(userId, machineModel) {
        return await request(`/admin/users/${userId}/inject-machine`, {
            method: 'POST',
            body: { machineModel }
        });
    },

    async getAdminTransactions(status = '', limit = 100, offset = 0) {
        const params = new URLSearchParams();
        if (status) params.append('status', status);
        params.append('limit', limit);
        params.append('offset', offset);

        const res = await request(`/admin/withdrawals?${params.toString()}`);
        return res?.data || (Array.isArray(res) ? res : []);
    },

    async getAdminDre() {
        const res = await request('/admin/dre');
        return res?.data || res || {};
    },

    async approveWithdrawal(txId) {
        return await request(`/admin/withdrawals/${txId}/approve`, {
            method: 'POST'
        });
    },

    async rejectWithdrawal(txId, reason = '', lockAccount = false) {
        return await request(`/admin/withdrawals/${txId}/reject`, {
            method: 'POST',
            body: { reason, lockAccount }
        });
    },

    async getSystemConfig() {
        const res = await request('/admin/config');
        return res?.data || res || {};
    },

    async updateGatewaySettings(settings) {
        return await request('/admin/config/gateway', {
            method: 'PUT',
            body: settings
        });
    },

    async toggleGlobalWithdrawLock(locked) {
        return await request('/admin/config/toggle-global-lock', {
            method: 'POST',
            body: { locked }
        });
    },

    async updateAutoApproveSettings(enabled, underAmount) {
        return await request('/admin/config/auto-approve', {
            method: 'PUT',
            body: {
                autoApproveEnabled: enabled,
                autoApproveWithdrawalsUnder: underAmount
            }
        });
    },

    async updateReferralsSettings(settings) {
        return await request('/admin/config/referrals', {
            method: 'PUT',
            body: settings
        });
    },

    async getRouletteMatrix() {
        const res = await request('/admin/roulette/matrix');
        return res?.data || res || {};
    },

    async updateRouletteMatrix(slices, rules = null) {
        return await request('/admin/roulette/matrix', {
            method: 'PUT',
            body: { slices, rules }
        });
    },

    async updateRouletteConfig(slices, rules = null) {
        return await request('/admin/roulette/config', {
            method: 'POST',
            body: { slices, rules }
        });
    },

    async saveFleetCatalog(catalog) {
        return await request('/admin/fleet/catalog', {
            method: 'PUT',
            body: { catalog }
        });
    },

    async getAdminAnnouncements() {
        const res = await request('/admin/announcements');
        return res?.data || (Array.isArray(res) ? res : []);
    },

    async createAnnouncement(ann) {
        return await request('/admin/announcements', {
            method: 'POST',
            body: ann
        });
    },

    async toggleAnnouncement(id) {
        return await request(`/admin/announcements/${id}/toggle`, {
            method: 'PATCH'
        });
    },

    async deleteAnnouncement(id) {
        return await request(`/admin/announcements/${id}`, {
            method: 'DELETE'
        });
    },

    async getAdminSecurityLogs(limit = 100, offset = 0, userId = '') {
        const query = new URLSearchParams({ limit, offset });
        if (userId) query.append('userId', userId);
        const res = await request(`/admin/logs/security?${query.toString()}`);
        return res?.data || (Array.isArray(res) ? res : []);
    },

    async getAdminUsageLogs(limit = 100, offset = 0) {
        const query = new URLSearchParams({ limit, offset });
        const res = await request(`/admin/logs/usage?${query.toString()}`);
        return res?.data || (Array.isArray(res) ? res : []);
    },

    // 10. PASSE DE BATALHA & GESTÃO DOS 12 NÍVEIS
    async getLevels() {
        const res = await request('/admin/levels');
        return res?.data || (Array.isArray(res) ? res : []);
    },

    async updateLevel(levelNum, levelConfig) {
        return await request(`/admin/levels/${levelNum}`, {
            method: 'PUT',
            body: levelConfig
        });
    },

    async resetSeason(durationDays = 75) {
        return await request('/admin/season/reset', {
            method: 'POST',
            body: { durationDays }
        });
    },

    async updateLevel12Maintenance(maintenanceDays = 30, requiredReferrals = 1) {
        return await request('/admin/levels/maintenance', {
            method: 'PUT',
            body: { maintenanceDays, requiredReferrals }
        });
    }
};

export const adminApi = api;
export default api;
