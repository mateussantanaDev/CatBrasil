// Service de Gestão ERP CatBrasil (Módulo Protegido, DRE, Operadores, Saques, Roleta, Comunicados, Afiliados & Frota)
import { api } from './api.js';

const ADMIN_ROLES = ['admin', 'gestor', 'diretoria'];

export const gestaoService = {
    async restoreErpSession() {
        if (!api.getToken()) return null;
        try {
            const response = await api.get('/admin/profile');
            const profile = response?.user || response;
            if (!profile || !ADMIN_ROLES.includes(profile.role) || profile.twoFactorEnabled !== true) {
                throw new Error('Sessão administrativa inválida.');
            }
            return profile;
        } catch {
            api.setToken('');
            return null;
        }
    },

    async loginErp(username, password, pin, twoFactorCode = '') {
        api.setToken('');
        try {
            const response = await api.post('/admin/auth/login', {
                username: (username || '').trim(),
                password,
                pin: (pin || '').trim(),
                twoFactorCode: (twoFactorCode || '').trim()
            });
            const token = response?.data?.token;
            const role = response?.data?.gestor?.role;
            if (!token || !ADMIN_ROLES.includes(role)) {
                return { success: false, error: 'Acesso de gestão não autorizado.' };
            }
            api.setToken(token);
            return { success: true, user: response.data.gestor };
        } catch (error) {
            return { success: false, error: error.message || 'Credenciais de gestão inválidas.' };
        }
    },

    logoutErp() {
        api.post('/admin/auth/logout').catch(() => {});
        api.setToken('');
    },

    // Métricas Matemáticas e Contábeis (Executadas sobre dados reais da API)
    getRouletteMathematicalMetrics(slices = []) {
        let totalProb = 0;
        let expectedValueEV = 0;

        slices.forEach(s => {
            const prob = parseFloat(s.probability) || 0;
            const amt = parseFloat(s.amount) || 0;
            totalProb += prob;
            expectedValueEV += amt * (prob / 100);
        });

        const isProbValid = Math.abs(totalProb - 100.0) < 0.1;

        return {
            totalProb: parseFloat(totalProb.toFixed(2)),
            expectedValueEV: parseFloat(expectedValueEV.toFixed(2)),
            isProbValid
        };
    },

    getUserFinancialStats(userId, transactions = []) {
        const userTxs = transactions.filter(t => t.userId === userId || t.user_id === userId);
        let totalDeposited = 0;
        let totalWithdrawn = 0;

        userTxs.forEach(t => {
            const tipo = (t.tipo || '').toLowerCase();
            const val = Math.abs(t.valor || t.amount || 0);
            if (tipo.includes('depósito') || tipo.includes('deposito')) {
                totalDeposited += val;
            } else if (tipo.includes('saque')) {
                totalWithdrawn += val;
            }
        });

        const netLtvProfit = totalDeposited - totalWithdrawn;
        return { totalDeposited, totalWithdrawn, netLtvProfit, txCount: userTxs.length };
    },

    getWithdrawalQueueMetrics(transactions = []) {
        const withdrawTxs = transactions.filter(t => (t.tipo || '').toLowerCase().includes('saque'));
        const pending = withdrawTxs.filter(t => t.status === 'pendente');
        const approved = withdrawTxs.filter(t => t.status === 'sucesso' || t.status === 'aprovado');
        const rejected = withdrawTxs.filter(t => t.status === 'rejeitado' || t.status === 'recusado');

        return {
            pendingCount: pending.length,
            pendingTotalVal: pending.reduce((acc, t) => acc + Math.abs(t.valor || 0), 0),
            approvedCount: approved.length,
            approvedTotalVal: approved.reduce((acc, t) => acc + Math.abs(t.valor || 0), 0),
            rejectedCount: rejected.length,
            rejectedTotalVal: rejected.reduce((acc, t) => acc + Math.abs(t.valor || 0), 0)
        };
    },

    getBankMetrics(users = [], transactions = []) {
        let totalDeposits = 0;
        let totalWithdrawals = 0;
        let totalPassiveCustody = 0;

        transactions.forEach(t => {
            const tipo = (t.tipo || '').toLowerCase();
            const val = Math.abs(t.valor || t.amount || 0);
            const status = (t.status || '').toLowerCase();

            if (status === 'sucesso' || status === 'aprovado' || status === 'paid') {
                if (tipo.includes('depósito') || tipo.includes('deposito')) {
                    totalDeposits += val;
                } else if (tipo.includes('saque')) {
                    totalWithdrawals += val;
                }
            }
        });

        users.forEach(u => {
            totalPassiveCustody += parseFloat(u.balance || 0);
        });

        const grossProfit = totalDeposits - totalWithdrawals;
        const gatewayFees = totalDeposits * 0.015;
        const netProfit = grossProfit - gatewayFees;
        const profitMargin = totalDeposits > 0 ? ((grossProfit / totalDeposits) * 100) : 100;
        const liquidityCoverage = totalPassiveCustody > 0 ? ((grossProfit / totalPassiveCustody) * 100) : 100;

        return {
            totalDeposits,
            totalWithdrawals,
            totalPassiveCustody,
            grossProfit,
            gatewayFees,
            netProfit,
            profitMargin: parseFloat(profitMargin.toFixed(1)),
            liquidityCoverage: parseFloat(liquidityCoverage.toFixed(1)),
            totalActiveUsers: users.length,
            totalTxs: transactions.length
        };
    },

    getAffiliateNetworkMetrics(users = [], transactions = []) {
        let totalCommissionsPaid = 0;

        transactions.forEach(t => {
            const tipo = (t.tipo || '').toLowerCase();
            if (tipo.includes('comissão') || tipo.includes('comissao') || tipo.includes('afiliado')) {
                totalCommissionsPaid += Math.abs(t.valor || t.amount || 0);
            }
        });

        const suspendedCount = users.filter(u => u.referralBonusSuspended).length;
        const leadersList = users.map(u => {
            const userTxs = transactions.filter(t => t.userId === u.id || t.user_id === u.id);
            let userCommissions = 0;
            userTxs.forEach(t => {
                const tipo = (t.tipo || '').toLowerCase();
                if (tipo.includes('comissão') || tipo.includes('comissao') || tipo.includes('afiliado')) {
                    userCommissions += Math.abs(t.valor || t.amount || 0);
                }
            });

            return {
                id: u.id,
                username: u.username,
                nivel: u.nivel || 1,
                referralBonusSuspended: u.referralBonusSuspended || false,
                userCommissions
            };
        });

        return {
            totalCommissionsPaid,
            suspendedCount,
            leadersList
        };
    }
};

export default gestaoService;
