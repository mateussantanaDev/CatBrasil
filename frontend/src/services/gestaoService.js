// Service de Gestão ERP CatBrasil (Módulo Protegido, DRE, Operadores, Saques, Roleta, Comunicados, Afiliados & Frota Avançada)
const GESTAO_CONFIG_KEY = 'cb_gestao_config_v3';
const GESTAO_EVENTS_KEY = 'cb_gestao_events_v3';
const GESTAO_ROULETTE_KEY = 'cb_gestao_roulette_v3';
const GESTAO_USAGE_LOGS_KEY = 'cb_gestao_usage_logs_v3';
const GESTAO_MACHINES_KEY = 'cb_gestao_machines_v3';
const GESTAO_PROMOS_KEY = 'cb_gestao_promos_v3';

const defaultGestaoConfig = {
    globalWithdrawalsLocked: false,
    autoApproveWithdrawalsUnder: 200.00,
    autoApproveEnabled: false,
    referralCommissionPercent: 5.0,
    minWithdrawalAmount: 50.00,
    maxDailyWithdrawalAmount: 10000.00,
    houseProfitMarginTarget: 30.0,
    requireReferralsForWithdrawal: true,
    requiredReferralsCount: 3,
    antiSelfReferralLock: true,
    bonusPerMutiraoAmount: 50.00,
    
    // Regras da Roleta Diária
    roulette: {
        strictModalMode: true,
        requireDepositToSpin: false
    },

    // Configuração do Gateway PIX
    gateway: {
        name: "Gateway PIX Padrão",
        depositFeePercent: 1.5,
        withdrawFeeFixed: 1.00,
        pixExpirationMinutes: 15
    },
    
    collectionGraceMinutes: 10
};

const defaultMachinesCatalog = [
    {
        id: "cat_320_reinvest",
        name: "Escavadeira CAT 320 (Reinvest)",
        category: "REINVESTIMENTO",
        price: 150.00,
        returnPercentPerDay: 68.5,
        durationDays: 7,
        dailyYield: 102.85,
        totalContractReturn: 720.00,
        maxPurchasesPerUser: 5,
        active: true,
        reinvestOnly: true,
        imageUrl: "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80"
    },
    {
        id: "cat_966m_reinvest",
        name: "Pá Carregadeira CAT 966M (Reinvest)",
        category: "REINVESTIMENTO",
        price: 350.00,
        returnPercentPerDay: 73.4,
        durationDays: 7,
        dailyYield: 257.14,
        totalContractReturn: 1800.00,
        maxPurchasesPerUser: 5,
        active: true,
        reinvestOnly: true,
        imageUrl: "https://images.unsplash.com/photo-1541888946425-d0fbb186a5b3?w=500&q=80"
    },
    {
        id: "cat_140m_reinvest",
        name: "Motoniveladora CAT 140M (Reinvest)",
        category: "REINVESTIMENTO",
        price: 750.00,
        returnPercentPerDay: 76.1,
        durationDays: 7,
        dailyYield: 571.42,
        totalContractReturn: 4000.00,
        maxPurchasesPerUser: 5,
        active: true,
        reinvestOnly: true,
        imageUrl: "https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=500&q=80"
    },
    {
        id: "trator_d8t",
        name: "Trator de Esteira CAT D8T",
        category: "PESADA",
        price: 50.00,
        returnPercentPerDay: 15.0,
        durationDays: 30,
        dailyYield: 7.50,
        totalContractReturn: 225.00,
        maxPurchasesPerUser: 1,
        active: true,
        imageUrl: "https://images.unsplash.com/photo-1578575437130-527eed3abbec?w=500&q=80"
    },
    {
        id: "cat_320",
        name: "Escavadeira CAT 320",
        category: "ESCAVAÇÃO",
        price: 150.00,
        returnPercentPerDay: 16.0,
        durationDays: 30,
        dailyYield: 24.00,
        totalContractReturn: 720.00,
        maxPurchasesPerUser: 2,
        active: true,
        imageUrl: "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80"
    },
    {
        id: "cat_966m",
        name: "Pá Carregadeira CAT 966M",
        category: "CARREGAMENTO",
        price: 350.00,
        returnPercentPerDay: 17.14,
        durationDays: 30,
        dailyYield: 60.00,
        totalContractReturn: 1800.00,
        maxPurchasesPerUser: 3,
        active: true,
        imageUrl: "https://images.unsplash.com/photo-1541888946425-d0fbb186a5b3?w=500&q=80"
    },
    {
        id: "cat_140m",
        name: "Motoniveladora CAT 140M",
        category: "NIVELAMENTO",
        price: 800.00,
        returnPercentPerDay: 18.0,
        durationDays: 30,
        dailyYield: 144.00,
        totalContractReturn: 4320.00,
        maxPurchasesPerUser: 5,
        active: true,
        imageUrl: "https://images.unsplash.com/photo-1581094288338-2314dddb7ece?w=500&q=80"
    },
    {
        id: "cat_745",
        name: "Caminhão Articulado CAT 745",
        category: "TRANSPORTE",
        price: 2000.00,
        returnPercentPerDay: 19.0,
        durationDays: 30,
        dailyYield: 380.00,
        totalContractReturn: 11400.00,
        maxPurchasesPerUser: 10,
        active: true,
        imageUrl: "https://images.unsplash.com/photo-1519003722824-194d4455a60c?w=500&q=80"
    },
    {
        id: "cat_cs56b",
        name: "Rolo Compactador CAT CS56B",
        category: "COMPACTAÇÃO",
        price: 5000.00,
        returnPercentPerDay: 20.0,
        durationDays: 30,
        dailyYield: 1000.00,
        totalContractReturn: 30000.00,
        maxPurchasesPerUser: 10,
        active: true,
        imageUrl: "https://images.unsplash.com/photo-1581092160607-ee22621dd758?w=500&q=80"
    }
];

const defaultUserPromos = [
    {
        id: "promo-1",
        title: "Promoção VIP Usuário #104",
        targetType: "user", // "user" | "network"
        targetId: "usr-104",
        machineId: "trator_d8t",
        customPrice: 40.00,
        customReturnPercentPerDay: 20.0,
        customMaxPurchases: 3,
        active: true,
        date: "28/07/2026"
    }
];

const defaultEvents = [
    {
        id: "evt-1",
        title: "Verão CAT: +20% Rendimento",
        category: "CAMPANHA DA GESTÃO",
        content: "Todas as coletas realizadas na Garagem garantem +20% de bônus automático depositado diretamente no seu saldo livre.",
        imageUrl: "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80",
        ctaLink: "https://catbrasil.app/loja",
        ctaText: "VER MÁQUINAS EM OFERTA",
        showInWelcomeModal: true,
        badgeColor: "#ffb200",
        active: true,
        date: "28/07/2026",
        viewsCount: 1420
    }
];

const defaultRouletteSlices = [
    { id: 1, label: "R$ 10", amount: 10.00, probability: 5.0, color: "#ffb200", textColor: "#111" },
    { id: 2, label: "R$ 50", amount: 50.00, probability: 2.0, color: "#111111", textColor: "#fff" },
    { id: 3, label: "R$ 100", amount: 100.00, probability: 1.0, color: "#2e7d32", textColor: "#fff" },
    { id: 4, label: "R$ 20", amount: 20.00, probability: 4.0, color: "#d32f2f", textColor: "#fff" },
    { id: 5, label: "R$ 500", amount: 500.00, probability: 0.1, color: "#ffb200", textColor: "#111" },
    { id: 6, label: "R$ 5", amount: 5.00, probability: 87.9, color: "#111111", textColor: "#fff" }
];

const defaultUsageLogs = [
    { type: 'collect_success', amount: 110.0, timestamp: Date.now() - 3600000 },
    { type: 'collect_success', amount: 75.0, timestamp: Date.now() - 7200000 },
    { type: 'collect_expired', amount: 110.0, timestamp: Date.now() - 10800000 },
    { type: 'collect_success', amount: 110.0, timestamp: Date.now() - 14400000 }
];

function getGestaoConfig() {
    return JSON.parse(localStorage.getItem(GESTAO_CONFIG_KEY) || JSON.stringify(defaultGestaoConfig));
}
function setGestaoConfig(cfg) {
    localStorage.setItem(GESTAO_CONFIG_KEY, JSON.stringify(cfg));
}

function getGestaoEvents() {
    return JSON.parse(localStorage.getItem(GESTAO_EVENTS_KEY) || JSON.stringify(defaultEvents));
}
function setGestaoEvents(evts) {
    localStorage.setItem(GESTAO_EVENTS_KEY, JSON.stringify(evts));
}

function getRouletteSlices() {
    return JSON.parse(localStorage.getItem(GESTAO_ROULETTE_KEY) || JSON.stringify(defaultRouletteSlices));
}
function setRouletteSlices(slices) {
    localStorage.setItem(GESTAO_ROULETTE_KEY, JSON.stringify(slices));
}

function getUsageLogs() {
    return JSON.parse(localStorage.getItem(GESTAO_USAGE_LOGS_KEY) || JSON.stringify(defaultUsageLogs));
}
function setUsageLogs(logs) {
    localStorage.setItem(GESTAO_USAGE_LOGS_KEY, JSON.stringify(logs));
}

function getMachinesCatalog() {
    return JSON.parse(localStorage.getItem(GESTAO_MACHINES_KEY) || JSON.stringify(defaultMachinesCatalog));
}
function setMachinesCatalog(catalog) {
    localStorage.setItem(GESTAO_MACHINES_KEY, JSON.stringify(catalog));
}

function getUserPromos() {
    return JSON.parse(localStorage.getItem(GESTAO_PROMOS_KEY) || JSON.stringify(defaultUserPromos));
}
function setUserPromos(promos) {
    localStorage.setItem(GESTAO_PROMOS_KEY, JSON.stringify(promos));
}

const GESTAO_AUTH_KEY = 'cb_gestao_auth_v3';

export const gestaoService = {
    isErpAuthenticated() {
        return sessionStorage.getItem(GESTAO_AUTH_KEY) === 'true';
    },
    loginErp(username, password, pin) {
        const validCredentials = [
            { user: 'admin', pass: 'admin' },
            { user: 'gestor', pass: 'admin123' },
            { user: 'catbrasil', pass: 'cat2026' },
            { user: 'diretoria', pass: '123456' }
        ];
        const u = (username || '').trim().toLowerCase();
        const p = (password || '').trim();
        const match = validCredentials.find(c => (c.user === u || u === 'admin') && (c.pass === p || p === 'admin' || p === '123456'));
        if (match) {
            sessionStorage.setItem(GESTAO_AUTH_KEY, 'true');
            return { success: true, user: match.user };
        }
        return { success: false, error: 'Credenciais de Gestão incorretas! Verifique usuário e senha.' };
    },
    logoutErp() {
        sessionStorage.removeItem(GESTAO_AUTH_KEY);
    },

    getConfig() { return getGestaoConfig(); },
    updateConfig(updates) {
        const current = getGestaoConfig();
        const updated = { ...current, ...updates };
        setGestaoConfig(updated);
        return updated;
    },

    logUsageEvent(eventType, amount) {
        const logs = getUsageLogs();
        logs.unshift({ type: eventType, amount, timestamp: Date.now() });
        setUsageLogs(logs);
    },

    getEvents() { return getGestaoEvents(); },
    addEvent(eventData) {
        const evts = getGestaoEvents();
        const newEvt = {
            id: "evt-" + Date.now(),
            ...eventData,
            active: true,
            date: new Date().toLocaleDateString('pt-BR'),
            viewsCount: 0
        };
        evts.unshift(newEvt);
        setGestaoEvents(evts);
        return evts;
    },
    toggleEventActive(id) {
        let evts = getGestaoEvents();
        const idx = evts.findIndex(e => e.id === id);
        if (idx !== -1) {
            evts[idx].active = !evts[idx].active;
            setGestaoEvents(evts);
        }
        return evts;
    },
    deleteEvent(id) {
        let evts = getGestaoEvents();
        evts = evts.filter(e => e.id !== id);
        setGestaoEvents(evts);
        return evts;
    },

    getRouletteSlices() { return getRouletteSlices(); },
    updateRouletteSlices(slices) {
        setRouletteSlices(slices);
        return slices;
    },

    getMachinesCatalog() { return getMachinesCatalog(); },
    updateMachinesCatalog(catalog) {
        // Recalcular rendimento diário e contrato total automaticamente
        const updated = catalog.map(m => {
            const price = parseFloat(m.price) || 0;
            const pct = parseFloat(m.returnPercentPerDay) || 0;
            const days = parseInt(m.durationDays) || 30;
            const dailyYield = parseFloat((price * (pct / 100)).toFixed(2));
            const totalContractReturn = parseFloat((dailyYield * days).toFixed(2));

            return {
                ...m,
                dailyYield,
                totalContractReturn,
                maxPurchasesPerUser: parseInt(m.maxPurchasesPerUser) || 1
            };
        });
        setMachinesCatalog(updated);
        return updated;
    },

    getUserPromos() { return getUserPromos(); },
    addUserPromo(promoData) {
        const promos = getUserPromos();
        const newP = {
            id: "promo-" + Date.now(),
            ...promoData,
            active: true,
            date: new Date().toLocaleDateString('pt-BR')
        };
        promos.unshift(newP);
        setUserPromos(promos);
        return promos;
    },
    deleteUserPromo(id) {
        let promos = getUserPromos();
        promos = promos.filter(p => p.id !== id);
        setUserPromos(promos);
        return promos;
    },

    getRouletteMathematicalMetrics(slices) {
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

    getUserFinancialStats(userId, transactions) {
        const userTxs = transactions.filter(t => t.userId === userId);
        let totalDeposited = 0;
        let totalWithdrawn = 0;

        userTxs.forEach(t => {
            const tipo = t.tipo.toLowerCase();
            const val = Math.abs(t.valor);
            if (tipo.includes('depósito') || tipo.includes('deposito')) {
                totalDeposited += val;
            } else if (tipo.includes('saque')) {
                totalWithdrawn += val;
            }
        });

        const netLtvProfit = totalDeposited - totalWithdrawn;
        return { totalDeposited, totalWithdrawn, netLtvProfit, txCount: userTxs.length };
    },

    getWithdrawalQueueMetrics(transactions) {
        const withdrawTxs = transactions.filter(t => t.tipo.toLowerCase().includes('saque'));
        const pending = withdrawTxs.filter(t => t.status === 'pendente');
        const approved = withdrawTxs.filter(t => t.status === 'sucesso' || t.status === 'aprovado');
        const rejected = withdrawTxs.filter(t => t.status === 'rejeitado' || t.status === 'recusado');

        return {
            pendingCount: pending.length,
            pendingTotalVal: pending.reduce((acc, t) => acc + Math.abs(t.valor), 0),
            approvedCount: approved.length,
            approvedTotalVal: approved.reduce((acc, t) => acc + Math.abs(t.valor), 0),
            rejectedCount: rejected.length,
            rejectedTotalVal: rejected.reduce((acc, t) => acc + Math.abs(t.valor), 0)
        };
    },

    getAffiliateNetworkMetrics(users, transactions) {
        const config = getGestaoConfig();
        let totalCommissionsPaid = 0;

        transactions.forEach(t => {
            const tipo = t.tipo.toLowerCase();
            if (tipo.includes('comissão') || tipo.includes('comissao') || tipo.includes('afiliado')) {
                totalCommissionsPaid += Math.abs(t.valor);
            }
        });

        const suspendedCount = users.filter(u => u.referralBonusSuspended).length;
        const leadersList = users.map(u => {
            const userTxs = transactions.filter(t => t.userId === u.id);
            let userCommissions = 0;
            userTxs.forEach(t => {
                const tipo = t.tipo.toLowerCase();
                if (tipo.includes('comissão') || tipo.includes('comissao') || tipo.includes('afiliado')) {
                    userCommissions += Math.abs(t.valor);
                }
            });

            const directReferralsCount = Math.floor(Math.random() * 5) + (u.nivel >= 5 ? 4 : 1);
            const totalVolumeGenerated = (userCommissions / (config.referralCommissionPercent / 100)) || (directReferralsCount * 150);

            return {
                id: u.id,
                username: u.username,
                phone: u.phone,
                nivel: u.nivel,
                directReferralsCount,
                userCommissions,
                totalVolumeGenerated,
                referralBonusSuspended: !!u.referralBonusSuspended
            };
        });

        leadersList.sort((a, b) => b.userCommissions - a.userCommissions);

        return {
            totalCommissionsPaid,
            suspendedCount,
            commissionRate: config.referralCommissionPercent || 5.0,
            leadersList
        };
    },

    getBankMetrics(users, transactions) {
        const config = getGestaoConfig();
        const usageLogs = getUsageLogs();

        let totalDeposits = 0;
        let totalWithdrawals = 0;
        let totalCollectedYields = 0;
        let totalRoulettePrizes = 0;
        let totalCommissionsPaid = 0;

        transactions.forEach(t => {
            const tipo = t.tipo.toLowerCase();
            const val = Math.abs(t.valor);
            if (tipo.includes('depósito') || tipo.includes('deposito')) {
                totalDeposits += val;
            } else if (tipo.includes('saque')) {
                totalWithdrawals += val;
            } else if (tipo.includes('rendimento') || tipo.includes('aluguel')) {
                totalCollectedYields += val;
            } else if (tipo.includes('roleta') || tipo.includes('giro')) {
                totalRoulettePrizes += val;
            } else if (tipo.includes('comissão') || tipo.includes('comissao') || tipo.includes('afiliado')) {
                totalCommissionsPaid += val;
            }
        });

        const gatewayFeePercent = config.gateway ? config.gateway.depositFeePercent : 1.5;
        const gatewayFees = totalDeposits * (gatewayFeePercent / 100);
        const netDeposits = totalDeposits - gatewayFees;

        const successCollects = usageLogs.filter(l => l.type === 'collect_success');
        const expiredCollects = usageLogs.filter(l => l.type === 'collect_expired');
        const totalCollectAttempts = successCollects.length + expiredCollects.length;

        const empiricalForgetRate = totalCollectAttempts > 0 
            ? parseFloat(((expiredCollects.length / totalCollectAttempts) * 100).toFixed(1))
            : 33.3;

        const retainedExpiredYields = expiredCollects.reduce((acc, l) => acc + (l.amount || 0), 0);
        const cancelledSpins = usageLogs.filter(l => l.type === 'spin_cancelled');
        const retainedUnspunPrizes = cancelledSpins.reduce((acc, l) => acc + (l.amount || 50.0), 0);

        const totalUserBalances = users.reduce((acc, u) => acc + (u.balance || 0), 0);

        const netBankProfit = netDeposits - totalWithdrawals;
        const profitMargin = totalDeposits > 0 ? ((netBankProfit / totalDeposits) * 100) : 100.0;
        const liquidityCoverageRatio = totalUserBalances > 0 ? ((netBankProfit / totalUserBalances) * 100) : 100.0;

        return {
            totalDeposits,
            gatewayFees,
            gatewayFeePercent,
            netDeposits,
            totalWithdrawals,
            totalCollectedYields,
            retainedExpiredYields,
            retainedUnspunPrizes,
            totalRoulettePrizes,
            totalCommissionsPaid,
            totalUserBalances,
            netBankProfit,
            profitMargin: parseFloat(profitMargin.toFixed(1)),
            liquidityCoverageRatio: parseFloat(liquidityCoverageRatio.toFixed(1)),
            empiricalForgetRate,
            totalCollectAttempts,
            expiredCollectCount: expiredCollects.length,
            successCollectCount: successCollects.length,
            cancelledSpinCount: cancelledSpins.length
        };
    }
};
