// Service Client de API REST Real (CatBrasil Backend Single Source of Truth)
const API_BASE_URL = 'http://localhost:5001/api';

async function request(endpoint, options = {}) {
    const headers = {
        'Content-Type': 'application/json',
        ...(options.token ? { 'Authorization': `Bearer ${options.token}` } : {}),
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
            throw new Error(data.error || data.message || `Erro do Servidor Backend (${res.status})`);
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
    // Autenticação Real no Servidor Express com Fingerprinting
    async login(username, password) {
        const fingerprint = getDeviceFingerprint();
        const data = await request('/auth/login', {
            method: 'POST',
            body: { username, password: password || username, fingerprint }
        });
        const user = await this.getProfile(data.token);
        return { token: data.token, user, message: data.message };
    },

    async register(username, password, inviteCode) {
        const fingerprint = getDeviceFingerprint();
        const data = await request('/auth/register', {
            method: 'POST',
            body: { username, password, inviteCode, fingerprint }
        });
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
        return await request('/garage', { token });
    },

    async collectYield(token, machineId) {
        return await request('/garage/collect', {
            method: 'POST',
            token,
            body: { machineId }
        });
    },

    // Loja (Alugar Máquinas com Validação Exclusiva do Servidor)
    async rentMachine(token, machineModel) {
        const data = await request('/shop/rent', {
            method: 'POST',
            token,
            body: { machineId: machineModel }
        });
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
        const data = await request('/transactions/withdraw', {
            method: 'POST',
            token,
            body: { amount, pixKey, password: securityPassword }
        });
        return { newBalance: data.balance, fee: data.fee, message: data.message };
    },

    // Histórico de Transações e Rede
    async getTransactions(token) {
        return await request('/transactions', { token });
    },

    async getReferrals(token) {
        return await request('/referrals', { token });
    },

    // Ranking Semanal da Corrida de Mineração
    async getWeeklyRanking(token) {
        return await request('/ranking/weekly', { token });
    },

    // Roleta Diária
    async spinRoulette(token, mode = 'random') {
        return await request('/roulette/spin', {
            method: 'POST',
            token,
            body: { mode }
        });
    },

    async getRouletteConfig() {
        return await request('/roulette/config');
    },

    async updateRouletteConfig(slices) {
        return await request('/admin/roulette/config', {
            method: 'POST',
            body: { slices }
        });
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

    // Painel de Gestão ERP (Admin)
    async getAdminUsers() {
        return await request('/admin/users');
    },

    async updateAdminUser(userId, updates) {
        return await request('/admin/user/update', {
            method: 'POST',
            body: { userId, updates }
        });
    },

    async getAdminTransactions() {
        return await request('/admin/transactions');
    },

    async getAdminDre() {
        return await request('/admin/dre');
    },

    async approveWithdrawal(txId) {
        return await request('/admin/withdraw/approve', {
            method: 'POST',
            body: { txId }
        });
    },

    async rejectWithdrawal(txId, lockAccount = false) {
        return await request('/admin/withdraw/reject', {
            method: 'POST',
            body: { txId, lockAccount }
        });
    },

    async fullReset() {
        return await request('/dev/reset', { method: 'POST' });
    }
};
