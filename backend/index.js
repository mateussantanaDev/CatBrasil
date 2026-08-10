require('dotenv').config();
const express = require('express');
const cors = require('cors');
const compression = require('compression');
const http = require('http');
const { Server } = require('socket.io');
const bcrypt = require('bcryptjs');
const jwt = require('jsonwebtoken');
const db = require('./database');
const authenticateToken = require('./auth');
const pixGateway = require('./pixGateway');

const app = express();
const server = http.createServer(app);
const io = new Server(server, {
    cors: {
        origin: '*',
        methods: ['GET', 'POST']
    }
});
const PORT = process.env.PORT || 5001;
const JWT_SECRET = process.env.JWT_SECRET || 'super_secret_cat_token_987654321_key';

// COMPRESSÃO BROTLI / GZIP PARA ATÉ 70% DE REDUÇÃO DE DADOS MÓVEIS
app.use(compression({
    level: 6,
    threshold: 512
}));

app.use(cors());
app.use(express.json());

// EVENTOS WEBSOCKET EM TEMPO REAL (SOCKET.IO)
io.on('connection', (socket) => {
    socket.on('join_room', (userId) => {
        if (userId) {
            socket.join(userId);
        }
    });
});

function notifyUserRealtime(userId, event, payload) {
    if (userId && io) {
        io.to(userId).emit(event, payload);
    }
}

// ================= MITIGAÇÃO DE SEGURANÇA E CABEÇALHOS HTTP =================
app.use((req, res, next) => {
    res.setHeader('X-Frame-Options', 'DENY');
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('X-XSS-Protection', '1; mode=block');
    res.setHeader('Referrer-Policy', 'strict-origin-when-cross-origin');
    next();
});

// LIMITADOR DE TAXA DE REQUISIÇÕES (RATE LIMITER ANTI-BRUTE FORCE / ANTI-DDOS)
const requestRateMap = new Map();

function createRateLimiter({ windowMs, maxRequests, message }) {
    return (req, res, next) => {
        const ip = req.ip || req.headers['x-forwarded-for'] || req.socket.remoteAddress || '127.0.0.1';
        const now = Date.now();
        const key = `${req.path}:${ip}`;

        const record = requestRateMap.get(key) || { count: 0, resetTime: now + windowMs };

        if (now > record.resetTime) {
            record.count = 1;
            record.resetTime = now + windowMs;
        } else {
            record.count++;
        }

        requestRateMap.set(key, record);

        if (record.count > maxRequests) {
            return res.status(429).json({
                error: message || "Muitas requisições. Aguarde um momento e tente novamente."
            });
        }

        next();
    };
}

const loginRateLimiter = createRateLimiter({
    windowMs: 60 * 1000,
    maxRequests: 10,
    message: "Muitas tentativas de autenticação. Por segurança, aguarde 1 minuto."
});

const apiRateLimiter = createRateLimiter({
    windowMs: 60 * 1000,
    maxRequests: 200,
    message: "Limite de requisições excedido. Reduza a frequência de chamadas à API."
});

app.use('/api/', apiRateLimiter);
app.use('/api/auth/login', loginRateLimiter);
app.use('/api/auth/register', loginRateLimiter);

// Helper para recalcular estados da garagem dinamicamente com base em timestamps e regras de 10 minutos
function getProcessedMachines(userId) {
    const machines = db.getMachines(userId);
    const now = Date.now();
    const cycle = 24 * 3600 * 1000; // 24 horas
    const grace = 10 * 60 * 1000;   // 10 minutos de tolerância

    return machines.map(m => {
        if (m.overrideState) {
            return {
                ...m,
                state: m.overrideState,
                secondsRemaining: m.overrideSecondsRemaining || 0
            };
        }

        const lastCollect = new Date(m.lastCollectedAt).getTime();
        const nextRelease = lastCollect + cycle;
        const limitTime = nextRelease + grace;
        
        let state = "aguardando";
        let secondsRemaining = 0;

        if (now < nextRelease) {
            state = "aguardando";
            secondsRemaining = Math.max(0, Math.floor((nextRelease - now) / 1000));
        } else if (now >= nextRelease && now <= limitTime) {
            state = "disponivel";
            secondsRemaining = Math.max(0, Math.floor((limitTime - now) / 1000));
        } else {
            state = "expirado";
            secondsRemaining = 0;
        }

        return {
            ...m,
            state,
            secondsRemaining
        };
    });
}

// ================= ROTAS DE AUTENTICAÇÃO REAL =================
app.post('/api/auth/register', async (req, res) => {
    const { username, password, inviteCode, fingerprint } = req.body;
    const ipAddress = req.ip || req.headers['x-forwarded-for'] || req.socket.remoteAddress || '127.0.0.1';
    
    if (!username || !password) {
        return res.status(400).json({ error: "Preencha os campos de usuário e senha." });
    }

    if (username.length < 3 || password.length < 5) {
        return res.status(400).json({ error: "Usuário deve ter 3+ caracteres e Senha 5+ caracteres." });
    }

    const existingUser = db.getUserByUsername(username);
    if (existingUser) {
        return res.status(400).json({ error: "Este usuário já está cadastrado." });
    }

    // Detecção Anti-Fraude de Multi-Contas no mesmo dispositivo/IP
    const existingRelatedUsers = db.getUsersByFingerprintOrIp(fingerprint, ipAddress);
    const isMultiAccount = existingRelatedUsers.length >= 2;

    // Criptografa senha
    const salt = await bcrypt.genSalt(10);
    const hashedPassword = await bcrypt.hash(password, salt);

    // Cria o usuário
    const newUser = db.addUser({
        username,
        password: hashedPassword,
        ipAddress,
        deviceFingerprint: fingerprint || '',
        multiAccountFlag: isMultiAccount
    });

    if (isMultiAccount) {
        db.logSecurityEvent(newUser.id, 'MULTI_ACCOUNT_DETECTED', ipAddress, fingerprint);
    } else {
        db.logSecurityEvent(newUser.id, 'USER_REGISTERED', ipAddress, fingerprint);
    }

    // Se informou convite, vincula ao referrer (se não for conta duplicada suspeita)
    if (inviteCode && !isMultiAccount) {
        const referrer = db.getUserById(inviteCode.toUpperCase());
        if (referrer) {
            db.addReferral({
                referrerId: referrer.id,
                nome: username,
                status: "inativo",
                deposito: 0.00
            });
        }
    }

    const token = jwt.sign({ userId: newUser.id }, JWT_SECRET, { expiresIn: '7d' });

    res.status(201).json({
        message: "Operador cadastrado com sucesso na CatBrasil!",
        token,
        userId: newUser.id,
        multiAccountFlag: isMultiAccount
    });
});

app.post('/api/auth/login', async (req, res) => {
    const { username, password, fingerprint } = req.body;
    const ipAddress = req.ip || req.headers['x-forwarded-for'] || req.socket.remoteAddress || '127.0.0.1';

    if (!username || !password) {
        return res.status(400).json({ error: "Preencha usuário e senha." });
    }

    const user = db.getUserByUsername(username);

    if (!user) {
        return res.status(400).json({ error: "Usuário ou senha incorretos." });
    }

    const isMatch = await bcrypt.compare(password, user.password);
    if (!isMatch && password !== user.password) {
        return res.status(400).json({ error: "Usuário ou senha incorretos." });
    }

    // Atualiza endereço IP e Fingerprint de Acesso
    db.updateUser(user.id, {
        ipAddress,
        deviceFingerprint: fingerprint || user.deviceFingerprint || ''
    });

    db.logSecurityEvent(user.id, 'USER_LOGGED_IN', ipAddress, fingerprint);

    const token = jwt.sign({ userId: user.id }, JWT_SECRET, { expiresIn: '7d' });

    res.json({
        message: "Bem-vindo de volta à CatBrasil!",
        token,
        userId: user.id
    });
});

// ================= ROTAS PROTEGIDAS =================

// 1. DADOS DO OPERADOR
app.get('/api/user', authenticateToken, (req, res) => {
    const user = db.getUserById(req.userId);
    if (!user) return res.status(404).json({ error: "Operador não encontrado" });

    const referrals = db.getReferrals(req.userId);
    const activeCount = referrals.filter(r => r.status === 'ativo').length;
    const isSuspended = Boolean(user.referralBonusSuspended) || (referrals.length > 0 && referrals.some(r => r.status === 'inativo'));

    const reqRoll = user.requiredRollover || 0;
    const currRoll = user.currentRollover || 0;
    const rollPercent = reqRoll > 0 ? Math.min(100, parseFloat(((currRoll / reqRoll) * 100).toFixed(1))) : 100;

    res.json({
        id: user.id,
        username: user.username,
        balance: user.balance,
        xp: user.xp,
        nivel: user.nivel,
        securityPassword: user.securityPassword ? "CADASTRADA" : "",
        promoUnlocked: user.promoUnlocked,
        freeWithdrawActive: user.freeWithdrawActive,
        coletasCompleted: user.coletasCompleted,
        tratorRented: user.tratorRented,
        alreadySpunToday: user.alreadySpunToday,
        requiredRollover: reqRoll,
        currentRollover: currRoll,
        rolloverProgressPercent: rollPercent,
        yieldBalanceAvailable: user.yieldBalanceAvailable || 0,
        partsInventory: {
            motorC15: user.motorC15Count || 0,
            esteiraReforcada: user.esteiraReforcadaCount || 0,
            filtroOleo: user.filtroOleoCount || 0,
            turboGarrett: user.turboGarrettCount || 0
        },
        referralsCount: referrals.length,
        activeReferralsCount: activeCount,
        referralBonusSuspended: isSuspended
    });
});

app.post('/api/user/security-password', authenticateToken, (req, res) => {
    const { password } = req.body;
    if (!password || password.length !== 6) {
        return res.status(400).json({ error: "A senha deve conter exatamente 6 dígitos" });
    }
    
    db.updateUser(req.userId, { securityPassword: password });
    res.json({ message: "Senha de segurança salva com sucesso" });
});

app.post('/api/user/level-up', authenticateToken, (req, res) => {
    const user = db.getUserById(req.userId);
    const referrals = db.getReferrals(req.userId);
    const activeGuests = referrals.filter(r => r.status === 'ativo').length;

    if (user.coletasCompleted < 5 || !user.tratorRented || activeGuests < 3) {
        return res.status(400).json({ error: "Requisitos de promoção ainda pendentes" });
    }

    const novoNivel = user.nivel + 1;
    db.updateUser(req.userId, {
        nivel: novoNivel,
        coletasCompleted: 0,
        tratorRented: false
    });

    db.addTransaction({
        userId: req.userId,
        tipo: "Promoção Nível",
        valor: 0.00,
        status: "sucesso"
    });

    res.json({
        message: `Parabéns! Subiu para o Nível ${novoNivel}`,
        nivel: novoNivel
    });
});

// 2. GARAGEM (COLETAS E SIMULADOR)
app.get('/api/garage', authenticateToken, (req, res) => {
    const processed = getProcessedMachines(req.userId);
    res.json(processed);
});

app.post('/api/garage/collect', authenticateToken, (req, res) => {
    const { machineId } = req.body;
    const processed = getProcessedMachines(req.userId);
    const machine = processed.find(m => m.id === machineId);

    if (!machine) {
        return res.status(404).json({ error: "Máquina não encontrada" });
    }

    if (machine.state === 'expirado') {
        db.logUsageEvent('collect_expired', machine.yieldAmount);
        db.updateMachine(machineId, {
            lastCollectedAt: new Date().toISOString(),
            overrideState: null,
            overrideSecondsRemaining: null
        });
        return res.status(400).json({ error: "Janela de coleta de 10min expirada! Rendimento retido para a banca." });
    }

    if (machine.state !== 'disponivel') {
        return res.status(400).json({ error: `Coleta indisponível (Status: ${machine.state})` });
    }

    const user = db.getUserById(req.userId);
    const newBalance = user.balance + machine.yieldAmount;
    const newYieldBalanceAvailable = (user.yieldBalanceAvailable || 0) + machine.yieldAmount;
    const novasColetas = Math.min(5, user.coletasCompleted + 1);

    db.updateUser(req.userId, {
        balance: newBalance,
        yieldBalanceAvailable: newYieldBalanceAvailable,
        coletasCompleted: novasColetas
    });

    db.updateMachine(machineId, {
        lastCollectedAt: new Date().toISOString(),
        overrideState: null,
        overrideSecondsRemaining: null
    });

    db.addTransaction({
        userId: req.userId,
        tipo: `Rendimento ${machine.name}`,
        valor: machine.yieldAmount,
        status: "sucesso"
    });

    db.logUsageEvent('collect_success', machine.yieldAmount);

    // Atualiza a Pontuação Semanal na Corrida de Mineração
    const addedTonnage = Math.round(machine.yieldAmount * 10);
    db.addOrUpdateWeeklyTonnage(user.id, user.username, addedTonnage);

    // Se o usuário foi convidado por alguém, ativa ele no referrer na primeira coleta!
    const allRefs = db.getAllReferrals();
    const foundRef = allRefs.find(r => r.nome && r.nome.toLowerCase() === user.username.toLowerCase());
    if (foundRef && foundRef.status !== 'ativo') {
        db.updateReferral(foundRef.id, {
            status: 'ativo',
            deposito: 350.00
        });
    }

    res.json({
        message: "Coleta de rendimentos efetuada com sucesso!",
        balance: newBalance,
        coletasCompleted: novasColetas
    });
});

app.post('/api/garage/simulate', authenticateToken, (req, res) => {
    const { state } = req.body;
    const machines = db.getMachines(req.userId);
    if (machines.length === 0) return res.status(404).json({ error: "Nenhuma máquina na garagem" });
    
    const machineId = machines[0].id;
    let updates = {};

    if (state === 'aguardando') {
        updates = {
            overrideState: 'aguardando',
            overrideSecondsRemaining: 15,
            lastCollectedAt: new Date(Date.now() - 23.996 * 3600 * 1000).toISOString()
        };
    } else if (state === 'disponivel') {
        updates = {
            overrideState: 'disponivel',
            overrideSecondsRemaining: 600,
            lastCollectedAt: new Date(Date.now() - 24.0 * 3600 * 1000).toISOString()
        };
    } else if (state === 'expirado') {
        updates = {
            overrideState: 'expirado',
            overrideSecondsRemaining: 0,
            lastCollectedAt: new Date(Date.now() - 24.5 * 3600 * 1000).toISOString()
        };
    } else {
        updates = {
            overrideState: null,
            overrideSecondsRemaining: null,
            lastCollectedAt: new Date().toISOString()
        };
    }

    db.updateMachine(machineId, updates);
    if (state === 'disponivel') {
        db.updateUser(req.userId, { coletasCompleted: 4 });
    }

    res.json({ message: `Estado da garagem simulado para: ${state}` });
});

// Catalogo oficial de máquinas validado 100% no servidor backend (Single Source of Truth)
const OFFICIAL_MACHINES_CATALOG = {
    'trator_d8t': { name: 'Trator de Esteira CAT D8T', cost: 1500, yieldAmount: 75 },
    'cat_320': { name: 'Escavadeira CAT 320', cost: 2500, yieldAmount: 110 },
    'cat_966m': { name: 'Pá Carregadeira CAT 966M', cost: 350, yieldAmount: 60 },
    'cat_140m': { name: 'Motoniveladora CAT 140M', cost: 800, yieldAmount: 144 },
    'cat_745': { name: 'Caminhão Articulado CAT 745', cost: 2000, yieldAmount: 380 },
    'cat_cs56b': { name: 'Rolo Compactador CAT CS56B', cost: 5000, yieldAmount: 1000 },

    // LINHA REINVEST (EXCLUSIVA COM SALDO GERADO DE COLETAS)
    'cat_320_reinvest': { name: 'Escavadeira CAT 320 (Reinvest)', cost: 150, yieldAmount: 102.85, reinvestOnly: true, durationDays: 7 },
    'cat_966m_reinvest': { name: 'Pá Carregadeira CAT 966M (Reinvest)', cost: 350, yieldAmount: 257.14, reinvestOnly: true, durationDays: 7 },
    'cat_140m_reinvest': { name: 'Motoniveladora CAT 140M (Reinvest)', cost: 800, yieldAmount: 617.14, reinvestOnly: true, durationDays: 7 }
};

// 3. LOJA (ALUGAR) - VALIDAÇÃO E CÁLCULO EXCLUSIVOS NO BACKEND
app.post('/api/shop/rent', authenticateToken, (req, res) => {
    const { machineId, machineModel } = req.body;
    const modelKey = machineId || machineModel;
    const catalogItem = OFFICIAL_MACHINES_CATALOG[modelKey];

    if (!catalogItem) {
        return res.status(400).json({ error: "Modelo de máquina inválido ou indisponível" });
    }

    const user = db.getUserById(req.userId);
    if (!user) return res.status(404).json({ error: "Usuário não encontrado" });

    // Se for máquina exclusiva de Reinvestimento Turbinado, exige saldo gerado de coletas na plataforma
    if (catalogItem.reinvestOnly) {
        const yieldAvail = user.yieldBalanceAvailable || 0;
        if (yieldAvail < catalogItem.cost) {
            return res.status(400).json({
                error: `Esta máquina é EXCLUSIVA da linha Reinvest Turbinado! É necessário ter pelo menos R$ ${catalogItem.cost.toFixed(2)} em rendimentos acumulados de coletas (Saldo de coletas disponível: R$ ${yieldAvail.toFixed(2)}).`
            });
        }
    }

    if (user.balance < catalogItem.cost) {
        return res.status(400).json({ error: `Saldo insuficiente. Custo de aluguel: R$ ${catalogItem.cost.toFixed(2)}` });
    }

    const newBalance = user.balance - catalogItem.cost;
    const newCurrentRollover = (user.currentRollover || 0) + catalogItem.cost;
    const newYieldBalanceAvailable = catalogItem.reinvestOnly ? Math.max(0, (user.yieldBalanceAvailable || 0) - catalogItem.cost) : (user.yieldBalanceAvailable || 0);
    const xpGained = Math.floor(catalogItem.cost * 0.02);
    let tratorAtivo = user.tratorRented;

    if (modelKey === 'trator_d8t') {
        tratorAtivo = true;
    }

    // Cria a máquina na garagem do operador
    const newMachine = db.addMachine({
        id: `mach-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
        userId: req.userId,
        name: catalogItem.name,
        cost: catalogItem.cost,
        yieldAmount: catalogItem.yieldAmount,
        lastCollectedAt: new Date().toISOString(),
        state: "aguardando"
    });

    db.updateUser(req.userId, {
        balance: newBalance,
        currentRollover: newCurrentRollover,
        yieldBalanceAvailable: newYieldBalanceAvailable,
        xp: Math.min(1000, user.xp + xpGained),
        tratorRented: tratorAtivo
    });

    db.addTransaction({
        userId: req.userId,
        tipo: `Aluguel ${catalogItem.name}`,
        valor: -catalogItem.cost,
        status: "sucesso"
    });

    res.json({
        message: `${catalogItem.name} alugado com sucesso!`,
        newMachine,
        balance: newBalance,
        xp: user.xp + xpGained,
        tratorRented: tratorAtivo
    });
});

// GERAÇÃO DE PIX PARA DEPÓSITO VIA GATEWAY (RETORNA QR CODE + SALDO PERMANECE PENDENTE)
app.post('/api/pix/generate', authenticateToken, async (req, res) => {
    try {
        const { amount } = req.body;
        const result = await pixGateway.createPixDeposit({
            userId: req.userId,
            amount
        });
        res.json(result);
    } catch (err) {
        res.status(400).json({ error: err.message });
    }
});

// WEBHOOK OFICIAL DO GATEWAY PIX (NOTIFICAÇÃO DE PAGAMENTO CONFIRMADO)
app.post('/api/webhooks/pix', async (req, res) => {
    try {
        console.log("📥 [Webhook PIX Recebido]:", req.body);
        const result = await pixGateway.processWebhookNotification(req.body, req.headers);
        if (!result.success) {
            return res.status(400).json(result);
        }
        notifyUserRealtime(result.userId, 'pix_confirmed', result);
        res.json({ success: true, message: result.message, txId: result.txId });
    } catch (err) {
        console.error("❌ [Erro Webhook PIX]:", err.message);
        res.status(500).json({ error: "Erro interno ao processar webhook de pagamento" });
    }
});

// SIMULADOR DE WEBHOOK PARA TESTES DE QA E DESENVOLVIMENTO
app.post('/api/dev/simulate-webhook', async (req, res) => {
    try {
        const { txId, paidAmount } = req.body;
        if (!txId) {
            return res.status(400).json({ error: "Informe o txId da transação pendente" });
        }
        const targetTx = db.getTransactionById(txId);
        if (!targetTx) {
            return res.status(404).json({ error: `Transação #${txId} não encontrada` });
        }

        const actualPaid = paidAmount !== undefined ? parseFloat(paidAmount) : Math.abs(targetTx.valor);
        const webhookPayload = {
            txId: targetTx.id,
            status: "paid",
            paidAmount: actualPaid,
            timestamp: new Date().toISOString()
        };

        const result = await pixGateway.processWebhookNotification(webhookPayload);
        res.json({
            message: "Simulação de Webhook PIX executada!",
            result
        });
    } catch (err) {
        res.status(500).json({ error: err.message });
    }
});

// 4. CONVIDADOS (SIMULADOR E LISTA)
app.get('/api/referrals', authenticateToken, (req, res) => {
    const list = db.getReferrals(req.userId);
    res.json(list);
});

app.post('/api/referrals/simulate', authenticateToken, (req, res) => {
    const { active } = req.body;
    const list = db.getReferrals(req.userId);

    if (list.length > 0) {
        db.updateReferral(list[0].id, {
            status: active ? "ativo" : "inativo",
            deposito: active ? 350.00 : 0.00
        });
    } else {
        db.addReferral({
            referrerId: req.userId,
            nome: "Maria Oliveira (Simulado)",
            status: active ? "ativo" : "inativo",
            deposito: active ? 350.00 : 0.00
        });
    }

    res.json({ message: "Status de convidados de rede atualizado no simulador" });
});

// 5. TRANSAÇÕES E RETIRADAS
app.get('/api/transactions', authenticateToken, (req, res) => {
    const list = db.getTransactions(req.userId);
    res.json(list);
});

app.post('/api/transactions/withdraw', authenticateToken, async (req, res) => {
    try {
        const { amount, pixKey, password, twoFactorCode } = req.body;
        const result = await pixGateway.executePixPayout({
            userId: req.userId,
            amount,
            pixKey,
            securityPassword: password,
            twoFactorCode: twoFactorCode || password
        });
        res.json(result);
    } catch (err) {
        res.status(400).json({ error: err.message });
    }
});

// 5B. RANKING SEMANAL DA CORRIDA DE MINERAÇÃO DA CATBRASIL
app.get('/api/ranking/weekly', authenticateToken, (req, res) => {
    const list = db.getWeeklyLeaderboard();
    const prizes = [
        { prize: "Escavadeira CAT 320 TURBO (7 Dias)", badge: "👑 LENDÁRIO" },
        { prize: "Pá Carregadeira CAT 966M (7 Dias)", badge: "🥇 OURO" },
        { prize: "Motoniveladora CAT 140M (7 Dias)", badge: "🥈 PRATA" },
        { prize: "Bônus +20% XP", badge: "🥉 BRONZE" },
        { prize: "Bônus +15% XP", badge: "⭐ TOP 5" },
        { prize: "Selo VIP Ouro", badge: "⭐ TOP 10" },
        { prize: "Selo VIP Prata", badge: "⭐ TOP 10" },
        { prize: "Selo VIP Prata", badge: "⭐ TOP 10" },
        { prize: "Selo VIP Bronze", badge: "⭐ TOP 10" },
        { prize: "Selo VIP Bronze", badge: "⭐ TOP 10" }
    ];

    const leaderboard = list.map((item, index) => ({
        rank: index + 1,
        userId: item.userId,
        username: item.username,
        weeklyTonnage: item.weeklyTonnage,
        prize: prizes[index] ? prizes[index].prize : "Selo VIP Participante",
        badge: prizes[index] ? prizes[index].badge : "⭐ TOP 10"
    }));

    const userEntryIndex = leaderboard.findIndex(item => item.userId === req.userId);
    const userRank = userEntryIndex >= 0 ? leaderboard[userEntryIndex] : {
        rank: 99,
        weeklyTonnage: 0,
        badge: "MINERADOR INICIANTE"
    };

    res.json({
        leaderboard,
        userRank,
        seasonEndsInHours: 48
    });
});

// 6. ROLETA DA SORTE DIÁRIA (UNIFICADA COM PROBABILIDADES DO ERP E SERVIDOREXPRESS)
app.get('/api/roulette/config', (req, res) => {
    res.json(db.getRouletteConfig());
});

app.post('/api/admin/roulette/config', (req, res) => {
    const { slices } = req.body;
    if (!Array.isArray(slices)) {
        return res.status(400).json({ error: "O parâmetro 'slices' deve ser um array" });
    }
    const updated = db.updateRouletteConfig(slices);
    res.json({ message: "Matriz de probabilidades da roleta salva no servidor!", slices: updated });
});

app.post('/api/roulette/spin', authenticateToken, (req, res) => {
    const user = db.getUserById(req.userId);

    if (!user) return res.status(404).json({ error: "Operador não encontrado" });

    if (user.alreadySpunToday) {
        return res.status(400).json({ error: "Você já realizou seu giro diário hoje!" });
    }

    const slices = db.getRouletteConfig();
    const roll = Math.random() * 100;
    let cumulative = 0;
    let sectorIdx = 0;

    for (let i = 0; i < slices.length; i++) {
        cumulative += parseFloat(slices[i].probability) || 0;
        if (roll <= cumulative) {
            sectorIdx = i;
            break;
        }
    }

    const winSlice = slices[sectorIdx] || slices[0];
    const prizeAmount = parseFloat(winSlice.amount) || 0;
    const newBalance = user.balance + prizeAmount;

    let updates = {
        balance: newBalance,
        alreadySpunToday: true
    };

    const labelLower = (winSlice.label || '').toLowerCase();
    let wonPartName = null;

    if (labelLower.includes('motor')) {
        updates.motorC15Count = (user.motorC15Count || 0) + 1;
        wonPartName = "Motor C15 Heavy Duty";
    } else if (labelLower.includes('esteira')) {
        updates.esteiraReforcadaCount = (user.esteiraReforcadaCount || 0) + 1;
        wonPartName = "Esteira Reforçada CAT";
    } else if (labelLower.includes('filtro')) {
        updates.filtroOleoCount = (user.filtroOleoCount || 0) + 1;
        wonPartName = "Filtro de Óleo Alta Vazão";
    } else if (labelLower.includes('turbo')) {
        updates.turboGarrettCount = (user.turboGarrettCount || 0) + 1;
        wonPartName = "Turbo Garrett T66";
    } else if (labelLower.includes('caixa')) {
        const partsKeys = ['motorC15Count', 'esteiraReforcadaCount', 'filtroOleoCount', 'turboGarrettCount'];
        const randomKey = partsKeys[Math.floor(Math.random() * partsKeys.length)];
        updates[randomKey] = (user[randomKey] || 0) + 1;
        wonPartName = "Peça Surpresa da Caixa Misteriosa CAT";
    }

    db.updateUser(req.userId, updates);

    if (prizeAmount > 0 || wonPartName) {
        db.addTransaction({
            userId: req.userId,
            tipo: `Giro Roleta: ${winSlice.label}`,
            valor: prizeAmount,
            status: "sucesso"
        });
        db.logUsageEvent('roulette_win', prizeAmount);
    }

    res.json({
        sectorIdx,
        win: winSlice,
        wonPartName,
        user: db.getUserById(req.userId)
    });
});

// MONTAGEM DE MÁQUINA A PARTIR DO INVENTÁRIO DE PEÇAS CATERPILLAR
app.post('/api/parts/assemble', authenticateToken, (req, res) => {
    const user = db.getUserById(req.userId);
    if (!user) return res.status(404).json({ error: "Operador não encontrado" });

    const motor = user.motorC15Count || 0;
    const esteira = user.esteiraReforcadaCount || 0;
    const filtro = user.filtroOleoCount || 0;
    const turbo = user.turboGarrettCount || 0;

    if (motor < 1 || esteira < 1 || filtro < 1 || turbo < 1) {
        return res.status(400).json({
            error: "Para montar a Escavadeira CAT 320, é necessário possuir as 4 peças (Motor C15, Esteira, Filtro e Turbo).",
            parts: { motor, esteira, filtro, turbo }
        });
    }

    // Debita 1 unidade de cada peça
    db.updateUser(req.userId, {
        motorC15Count: motor - 1,
        esteiraReforcadaCount: esteira - 1,
        filtroOleoCount: filtro - 1,
        turboGarrettCount: turbo - 1
    });

    // Cria a máquina Escavadeira CAT 320 na garagem do operador a CUSTO R$ 0,00 DE CAIXA!
    const newMachine = db.addMachine({
        id: `mach-parts-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
        userId: req.userId,
        name: "Escavadeira CAT 320 (Oficina de Peças) 🧩",
        cost: 2500,
        yieldAmount: 110.00,
        lastCollectedAt: new Date().toISOString(),
        state: "aguardando"
    });

    db.logSecurityEvent(req.userId, 'MACHINE_ASSEMBLED_FROM_PARTS', '', '');

    res.json({
        success: true,
        message: "🎉 PARABÉNS! Você montou 1x Escavadeira CAT 320 completa com suas 4 Peças Caterpillar!",
        newMachine,
        user: db.getUserById(req.userId)
    });
});

app.post('/api/roulette/reset', authenticateToken, (req, res) => {
    db.updateUser(req.userId, {
        alreadySpunToday: false,
        promoUnlocked: false,
        freeWithdrawActive: false
    });
    res.json({ message: "Giro diário resetado com sucesso para testes" });
});

// ================= ROTAS DE GESTÃO ERP (ADMIN) =================
app.get('/api/admin/users', (req, res) => {
    const users = db.getUsers().map(u => {
        const { password, ...safeUser } = u;
        return safeUser;
    });
    res.json(users);
});

app.post('/api/admin/user/update', (req, res) => {
    const { userId, updates } = req.body;
    if (!userId || !updates) {
        return res.status(400).json({ error: "Informe userId e os campos de atualização" });
    }
    const updated = db.updateUser(userId, updates);
    if (!updated) return res.status(404).json({ error: "Operador não encontrado" });
    res.json({ message: "Operador atualizado no servidor com sucesso", user: updated });
});

app.get('/api/admin/transactions', (req, res) => {
    const txs = db.getAllTransactions();
    res.json(txs);
});

app.post('/api/admin/withdraw/approve', (req, res) => {
    const { txId } = req.body;
    const tx = db.updateTransaction(txId, { status: "sucesso" });
    if (!tx) return res.status(404).json({ error: "Transação de saque não encontrada" });
    res.json({ message: `Saque #${txId} aprovado no servidor!`, tx });
});

app.post('/api/admin/withdraw/reject', (req, res) => {
    const { txId, lockAccount } = req.body;
    const allTxs = db.getAllTransactions();
    const targetTx = allTxs.find(t => t.id === txId);

    if (!targetTx) return res.status(404).json({ error: "Transação de saque não encontrada" });

    // Atualiza status da transação
    db.updateTransaction(txId, { status: "rejeitado" });

    // Devolve o saldo ao operador no banco de dados do servidor
    const user = db.getUserById(targetTx.userId);
    if (user) {
        const refundedBalance = user.balance + Math.abs(targetTx.valor);
        let userUpdates = { balance: refundedBalance };
        if (lockAccount) {
            userUpdates.withdrawLocked = true;
        }
        db.updateUser(user.id, userUpdates);
    }

    res.json({ message: `Saque #${txId} rejeitado e saldo estornado no servidor.`, txId });
});

app.get('/api/admin/dre', (req, res) => {
    const users = db.getUsers();
    const txs = db.getAllTransactions();
    const usageLogs = db.getUsageLogs();

    let totalDeposits = 0;
    let totalWithdrawals = 0;
    let totalRoulettePrizes = 0;

    txs.forEach(t => {
        if (t.status === 'sucesso') {
            const tipo = (t.tipo || '').toLowerCase();
            const val = Math.abs(t.valor);
            if (tipo.includes('depósito') || tipo.includes('deposito')) {
                totalDeposits += val;
            } else if (tipo.includes('saque')) {
                totalWithdrawals += val;
            } else if (tipo.includes('roleta') || tipo.includes('giro')) {
                totalRoulettePrizes += val;
            }
        }
    });

    const gatewayFees = totalDeposits * 0.015;
    const netDeposits = totalDeposits - gatewayFees;

    const successCollects = usageLogs.filter(l => l.type === 'collect_success');
    const expiredCollects = usageLogs.filter(l => l.type === 'collect_expired');
    const totalCollects = successCollects.length + expiredCollects.length;
    const retainedExpiredYields = expiredCollects.reduce((acc, l) => acc + (l.amount || 0), 0);

    const empiricalForgetRate = totalCollects > 0 ? parseFloat(((expiredCollects.length / totalCollects) * 100).toFixed(1)) : 33.3;

    const totalUserBalances = users.reduce((acc, u) => acc + (u.balance || 0), 0);
    const netBankProfit = netDeposits - totalWithdrawals;
    const profitMargin = totalDeposits > 0 ? parseFloat(((netBankProfit / totalDeposits) * 100).toFixed(1)) : 100.0;
    const liquidityCoverageRatio = totalUserBalances > 0 ? parseFloat(((netBankProfit / totalUserBalances) * 100).toFixed(1)) : 100.0;

    res.json({
        totalDeposits,
        gatewayFees,
        netDeposits,
        totalWithdrawals,
        retainedExpiredYields,
        totalRoulettePrizes,
        totalUserBalances,
        netBankProfit,
        profitMargin,
        liquidityCoverageRatio,
        empiricalForgetRate,
        expiredCollectCount: expiredCollects.length,
        successCollectCount: successCollects.length
    });
});

app.post('/api/dev/reset', (req, res) => {
    db.resetAll();
    res.json({ message: "Dados gerais resetados para testes de QA" });
});

server.listen(PORT, () => {
    console.log(`🚀 CatBrasil Backend & WebSockets rodando na porta ${PORT}`);
});
