const Database = require('better-sqlite3');
const path = require('path');
const fs = require('fs');

const DB_FILE = path.join(__dirname, 'catbrasil.db');
const db = new Database(DB_FILE);

// Ativa WAL Mode (Write-Ahead Logging) para máxima concorrência e velocidade ACID
db.pragma('journal_mode = WAL');

// Inicializa Esquema Relacional de Tabelas
db.exec(`
  CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    balance REAL DEFAULT 0.0,
    xp INTEGER DEFAULT 0,
    nivel INTEGER DEFAULT 1,
    securityPassword TEXT DEFAULT '',
    promoUnlocked INTEGER DEFAULT 0,
    freeWithdrawActive INTEGER DEFAULT 0,
    coletasCompleted INTEGER DEFAULT 0,
    tratorRented INTEGER DEFAULT 0,
    alreadySpunToday INTEGER DEFAULT 0,
    withdrawLocked INTEGER DEFAULT 0,
    referralBonusSuspended INTEGER DEFAULT 0,
    multiAccountFlag INTEGER DEFAULT 0,
    requiredRollover REAL DEFAULT 0.0,
    currentRollover REAL DEFAULT 0.0,
    ipAddress TEXT DEFAULT '',
    deviceFingerprint TEXT DEFAULT '',
    twoFactorSecret TEXT DEFAULT '',
    twoFactorEnabled INTEGER DEFAULT 0,
    createdAt TEXT NOT NULL
  );

  CREATE TABLE IF NOT EXISTS machines (
    id TEXT PRIMARY KEY,
    userId TEXT NOT NULL,
    name TEXT NOT NULL,
    cost REAL NOT NULL,
    yieldAmount REAL NOT NULL,
    lastCollectedAt TEXT NOT NULL,
    state TEXT DEFAULT 'aguardando',
    overrideState TEXT,
    overrideSecondsRemaining INTEGER,
    FOREIGN KEY(userId) REFERENCES users(id) ON DELETE CASCADE
  );

  CREATE TABLE IF NOT EXISTS referrals (
    id TEXT PRIMARY KEY,
    referrerId TEXT NOT NULL,
    nome TEXT NOT NULL,
    status TEXT DEFAULT 'inativo',
    deposito REAL DEFAULT 0.0,
    data TEXT NOT NULL,
    FOREIGN KEY(referrerId) REFERENCES users(id) ON DELETE CASCADE
  );

  CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    userId TEXT NOT NULL,
    data TEXT NOT NULL,
    tipo TEXT NOT NULL,
    valor REAL NOT NULL,
    status TEXT DEFAULT 'pendente',
    externalId TEXT,
    pixKey TEXT,
    netAmount REAL,
    feeAmount REAL,
    valorEfetivoPago REAL,
    paidAt TEXT,
    createdAt TEXT NOT NULL,
    FOREIGN KEY(userId) REFERENCES users(id) ON DELETE CASCADE
  );

  CREATE TABLE IF NOT EXISTS roulette_config (
    id INTEGER PRIMARY KEY,
    label TEXT NOT NULL,
    amount REAL NOT NULL,
    probability REAL NOT NULL,
    color TEXT NOT NULL,
    textColor TEXT NOT NULL
  );

  CREATE TABLE IF NOT EXISTS usage_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    amount REAL DEFAULT 0.0,
    timestamp INTEGER NOT NULL
  );

  CREATE TABLE IF NOT EXISTS security_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    userId TEXT,
    event TEXT NOT NULL,
    ipAddress TEXT,
    fingerprint TEXT,
    timestamp INTEGER NOT NULL
  );

  CREATE TABLE IF NOT EXISTS mining_race_weekly (
    userId TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    weeklyTonnage REAL DEFAULT 0.0,
    updatedAt TEXT NOT NULL
  );
`);

// Migração segura de colunas para tabelas existentes
try { db.exec('ALTER TABLE users ADD COLUMN requiredRollover REAL DEFAULT 0.0'); } catch (e) {}
try { db.exec('ALTER TABLE users ADD COLUMN currentRollover REAL DEFAULT 0.0'); } catch (e) {}
try { db.exec('ALTER TABLE users ADD COLUMN yieldBalanceAvailable REAL DEFAULT 0.0'); } catch (e) {}
try { db.exec('ALTER TABLE users ADD COLUMN motorC15Count INTEGER DEFAULT 0'); } catch (e) {}
try { db.exec('ALTER TABLE users ADD COLUMN esteiraReforcadaCount INTEGER DEFAULT 0'); } catch (e) {}
try { db.exec('ALTER TABLE users ADD COLUMN filtroOleoCount INTEGER DEFAULT 0'); } catch (e) {}
try { db.exec('ALTER TABLE users ADD COLUMN turboGarrettCount INTEGER DEFAULT 0'); } catch (e) {}
try { db.exec('ALTER TABLE transactions ADD COLUMN paidAt TEXT'); } catch (e) {}

// Popula dados padrão se o banco estiver vazio
const userCount = db.prepare('SELECT COUNT(*) as count FROM users').get().count;
if (userCount === 0) {
  console.log('📦 Populando banco de dados SQLite ACID inicial...');
  
  const insertUser = db.prepare(`
    INSERT INTO users (id, username, password, balance, xp, nivel, securityPassword, promoUnlocked, freeWithdrawActive, coletasCompleted, tratorRented, alreadySpunToday, createdAt)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `);
  insertUser.run("CAT-458F", "mateus", "admin", 2095.00, 247, 8, "123456", 0, 0, 4, 0, 1, new Date().toISOString());

  const insertMachine = db.prepare(`
    INSERT INTO machines (id, userId, name, cost, yieldAmount, lastCollectedAt, state)
    VALUES (?, ?, ?, ?, ?, ?, ?)
  `);
  insertMachine.run("mach-1", "CAT-458F", "Escavadeira CAT 320", 2500, 110.00, new Date(Date.now() - 23.9 * 3600 * 1000).toISOString(), "aguardando");

  const insertRef = db.prepare(`
    INSERT INTO referrals (id, referrerId, nome, status, deposito, data)
    VALUES (?, ?, ?, ?, ?, ?)
  `);
  insertRef.run("ref-1", "CAT-458F", "João Silva", "ativo", 500.00, "18/07/2026");
  insertRef.run("ref-2", "CAT-458F", "Maria Oliveira", "ativo", 350.00, "19/07/2026");
  insertRef.run("ref-3", "CAT-458F", "Carlos Pereira", "ativo", 1200.00, "20/07/2026");

  const insertTx = db.prepare(`
    INSERT INTO transactions (id, userId, data, tipo, valor, status, createdAt)
    VALUES (?, ?, ?, ?, ?, ?, ?)
  `);
  insertTx.run("tx-1", "CAT-458F", "20/07/2026", "Depósito", 300.00, "sucesso", new Date().toISOString());
  insertTx.run("tx-2", "CAT-458F", "18/07/2026", "Saque", -150.00, "sucesso", new Date().toISOString());
  insertTx.run("tx-3", "CAT-458F", "15/07/2026", "Depósito", 1000.00, "sucesso", new Date().toISOString());

  const insertLeaderboard = db.prepare('INSERT INTO mining_race_weekly (userId, username, weeklyTonnage, updatedAt) VALUES (?, ?, ?, ?)');
  const nowStr = new Date().toISOString();
  insertLeaderboard.run("CAT-458F", "mateus", 8450, nowStr);
  insertLeaderboard.run("CB-9901", "carlos_operador", 6200, nowStr);
  insertLeaderboard.run("CB-9902", "minerador_sp", 4890, nowStr);
  insertLeaderboard.run("CB-9903", "frota_brasil", 3750, nowStr);
  insertLeaderboard.run("CB-9904", "joao_escavadeira", 2900, nowStr);
  insertLeaderboard.run("CB-9905", "ana_cat", 2100, nowStr);
  insertLeaderboard.run("CB-9906", "pedro_br", 1850, nowStr);
  insertLeaderboard.run("CB-9907", "lucas_cat", 1400, nowStr);

  const insertRoulette = db.prepare(`
    INSERT INTO roulette_config (id, label, amount, probability, color, textColor)
    VALUES (?, ?, ?, ?, ?, ?)
  `);
  const defaultSlices = [
    { id: 1, label: "🧩 Motor C15", amount: 0, probability: 20.0, color: "#ffb200", textColor: "#111" },
    { id: 2, label: "🧩 Esteira CAT", amount: 0, probability: 20.0, color: "#111111", textColor: "#fff" },
    { id: 3, label: "🧩 Filtro Óleo", amount: 0, probability: 25.0, color: "#ffb200", textColor: "#111" },
    { id: 4, label: "🧩 Turbo Garrett", amount: 0, probability: 10.0, color: "#111111", textColor: "#fff" },
    { id: 5, label: "📦 Caixa CAT", amount: 0, probability: 15.0, color: "#ffb200", textColor: "#111" },
    { id: 6, label: "💰 R$ 5,00", amount: 5.00, probability: 10.0, color: "#2e7d32", textColor: "#fff" }
  ];
  defaultSlices.forEach(s => insertRoulette.run(s.id, s.label, s.amount, s.probability, s.color, s.textColor));
}

// Wrapper DB com Suporte Completo a Transações ACID Nativas
const sqliteDb = {
    // Execução de bloco em Transação ACID Atômica (Garantia contra Race Condition)
    transaction(fn) {
        return db.transaction(fn)();
    },

    getUsers() {
        const rows = db.prepare('SELECT * FROM users').all();
        return rows.map(u => ({
            ...u,
            promoUnlocked: Boolean(u.promoUnlocked),
            freeWithdrawActive: Boolean(u.freeWithdrawActive),
            tratorRented: Boolean(u.tratorRented),
            alreadySpunToday: Boolean(u.alreadySpunToday),
            withdrawLocked: Boolean(u.withdrawLocked),
            referralBonusSuspended: Boolean(u.referralBonusSuspended),
            multiAccountFlag: Boolean(u.multiAccountFlag),
            twoFactorEnabled: Boolean(u.twoFactorEnabled)
        }));
    },

    getUserById(id) {
        const u = db.prepare('SELECT * FROM users WHERE id = ?').get(id);
        if (!u) return null;
        return {
            ...u,
            promoUnlocked: Boolean(u.promoUnlocked),
            freeWithdrawActive: Boolean(u.freeWithdrawActive),
            tratorRented: Boolean(u.tratorRented),
            alreadySpunToday: Boolean(u.alreadySpunToday),
            withdrawLocked: Boolean(u.withdrawLocked),
            referralBonusSuspended: Boolean(u.referralBonusSuspended),
            multiAccountFlag: Boolean(u.multiAccountFlag),
            twoFactorEnabled: Boolean(u.twoFactorEnabled)
        };
    },

    getUserByUsername(username) {
        const u = db.prepare('SELECT * FROM users WHERE LOWER(username) = LOWER(?)').get(username);
        if (!u) return null;
        return {
            ...u,
            promoUnlocked: Boolean(u.promoUnlocked),
            freeWithdrawActive: Boolean(u.freeWithdrawActive),
            tratorRented: Boolean(u.tratorRented),
            alreadySpunToday: Boolean(u.alreadySpunToday),
            withdrawLocked: Boolean(u.withdrawLocked),
            referralBonusSuspended: Boolean(u.referralBonusSuspended),
            multiAccountFlag: Boolean(u.multiAccountFlag),
            twoFactorEnabled: Boolean(u.twoFactorEnabled)
        };
    },

    getUsersByFingerprintOrIp(fingerprint, ip) {
        const fp = fingerprint || 'FP-NONE';
        const clientIp = ip || 'IP-NONE';
        return db.prepare("SELECT * FROM users WHERE (deviceFingerprint = ? AND deviceFingerprint != '') OR (ipAddress = ? AND ipAddress != '')").all(fp, clientIp);
    },

    addUser(user) {
        const id = user.id || `CB-${Math.floor(1000 + Math.random() * 9000)}`;
        const createdAt = new Date().toISOString();

        db.prepare(`
            INSERT INTO users (id, username, password, balance, xp, nivel, securityPassword, promoUnlocked, freeWithdrawActive, coletasCompleted, tratorRented, alreadySpunToday, withdrawLocked, referralBonusSuspended, multiAccountFlag, ipAddress, deviceFingerprint, createdAt)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            id,
            user.username,
            user.password,
            user.balance || 0.0,
            user.xp || 0,
            user.nivel || 1,
            user.securityPassword || '',
            user.promoUnlocked ? 1 : 0,
            user.freeWithdrawActive ? 1 : 0,
            user.coletasCompleted || 0,
            user.tratorRented ? 1 : 0,
            user.alreadySpunToday ? 1 : 0,
            user.withdrawLocked ? 1 : 0,
            user.referralBonusSuspended ? 1 : 0,
            user.multiAccountFlag ? 1 : 0,
            user.ipAddress || '',
            user.deviceFingerprint || '',
            createdAt
        );

        // Máquina gratuita inicial
        db.prepare(`
            INSERT INTO machines (id, userId, name, cost, yieldAmount, lastCollectedAt, state)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        `).run(
            `mach-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
            id,
            "Escavadeira CAT 320",
            2500,
            110.00,
            new Date(Date.now() - 24 * 3600 * 1000).toISOString(),
            "disponivel"
        );

        return this.getUserById(id);
    },

    updateUser(id, updates) {
        const user = this.getUserById(id);
        if (!user) return null;

        const fields = [];
        const values = [];

        Object.keys(updates).forEach(key => {
            fields.push(`${key} = ?`);
            let val = updates[key];
            if (typeof val === 'boolean') val = val ? 1 : 0;
            values.push(val);
        });

        if (fields.length === 0) return user;

        values.push(id);
        db.prepare(`UPDATE users SET ${fields.join(', ')} WHERE id = ?`).run(...values);
        return this.getUserById(id);
    },

    getMachines(userId) {
        return db.prepare('SELECT * FROM machines WHERE userId = ?').all(userId);
    },

    getMachineById(id) {
        return db.prepare('SELECT * FROM machines WHERE id = ?').get(id);
    },

    addMachine(machine) {
        db.prepare(`
            INSERT INTO machines (id, userId, name, cost, yieldAmount, lastCollectedAt, state, overrideState, overrideSecondsRemaining)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            machine.id,
            machine.userId,
            machine.name,
            machine.cost,
            machine.yieldAmount,
            machine.lastCollectedAt,
            machine.state || 'aguardando',
            machine.overrideState || null,
            machine.overrideSecondsRemaining || null
        );
        return machine;
    },

    updateMachine(id, updates) {
        const fields = [];
        const values = [];
        Object.keys(updates).forEach(key => {
            fields.push(`${key} = ?`);
            values.push(updates[key]);
        });
        if (fields.length === 0) return this.getMachineById(id);
        values.push(id);
        db.prepare(`UPDATE machines SET ${fields.join(', ')} WHERE id = ?`).run(...values);
        return this.getMachineById(id);
    },

    getReferrals(userId) {
        return db.prepare('SELECT * FROM referrals WHERE referrerId = ?').all(userId);
    },

    getAllReferrals() {
        return db.prepare('SELECT * FROM referrals').all();
    },

    addReferral(ref) {
        const id = ref.id || `ref-${Date.now()}-${Math.floor(Math.random() * 1000)}`;
        const data = ref.data || new Date().toLocaleDateString('pt-BR');
        db.prepare(`
            INSERT INTO referrals (id, referrerId, nome, status, deposito, data)
            VALUES (?, ?, ?, ?, ?, ?)
        `).run(id, ref.referrerId, ref.nome, ref.status || 'inativo', ref.deposito || 0.0, data);
        return { id, ...ref, data };
    },

    updateReferral(id, updates) {
        const fields = [];
        const values = [];
        Object.keys(updates).forEach(key => {
            fields.push(`${key} = ?`);
            values.push(updates[key]);
        });
        if (fields.length === 0) return null;
        values.push(id);
        db.prepare(`UPDATE referrals SET ${fields.join(', ')} WHERE id = ?`).run(...values);
        return db.prepare('SELECT * FROM referrals WHERE id = ?').get(id);
    },

    getTransactions(userId) {
        return db.prepare('SELECT * FROM transactions WHERE userId = ? ORDER BY id DESC').all(userId);
    },

    getAllTransactions() {
        return db.prepare('SELECT * FROM transactions ORDER BY id DESC').all();
    },

    getTransactionById(id) {
        return db.prepare('SELECT * FROM transactions WHERE id = ?').get(id);
    },

    addTransaction(tx) {
        const id = tx.id || `tx-${Date.now()}-${Math.floor(Math.random() * 1000)}`;
        const data = tx.data || new Date().toLocaleDateString('pt-BR');
        const createdAt = tx.createdAt || new Date().toISOString();

        db.prepare(`
            INSERT INTO transactions (id, userId, data, tipo, valor, status, externalId, pixKey, netAmount, feeAmount, valorEfetivoPago, createdAt)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            id,
            tx.userId,
            data,
            tx.tipo,
            tx.valor,
            tx.status || 'pendente',
            tx.externalId || null,
            tx.pixKey || null,
            tx.netAmount || null,
            tx.feeAmount || null,
            tx.valorEfetivoPago || null,
            createdAt
        );
        return { id, data, ...tx };
    },

    updateTransaction(id, updates) {
        const fields = [];
        const values = [];
        Object.keys(updates).forEach(key => {
            fields.push(`${key} = ?`);
            values.push(updates[key]);
        });
        if (fields.length === 0) return this.getTransactionById(id);
        values.push(id);
        db.prepare(`UPDATE transactions SET ${fields.join(', ')} WHERE id = ?`).run(...values);
        return this.getTransactionById(id);
    },

    getRouletteConfig() {
        return db.prepare('SELECT * FROM roulette_config ORDER BY id ASC').all();
    },

    updateRouletteConfig(slices) {
        db.prepare('DELETE FROM roulette_config').run();
        const stmt = db.prepare(`
            INSERT INTO roulette_config (id, label, amount, probability, color, textColor)
            VALUES (?, ?, ?, ?, ?, ?)
        `);
        slices.forEach(s => stmt.run(s.id, s.label, s.amount, s.probability, s.color, s.textColor));
        return this.getRouletteConfig();
    },

    getUsageLogs() {
        return db.prepare('SELECT * FROM usage_logs ORDER BY id DESC').all();
    },

    logUsageEvent(type, amount) {
        db.prepare('INSERT INTO usage_logs (type, amount, timestamp) VALUES (?, ?, ?)').run(type, amount || 0, Date.now());
    },

    logSecurityEvent(userId, event, ipAddress, fingerprint) {
        db.prepare('INSERT INTO security_logs (userId, event, ipAddress, fingerprint, timestamp) VALUES (?, ?, ?, ?, ?)').run(userId || null, event, ipAddress || '', fingerprint || '', Date.now());
    },

    addOrUpdateWeeklyTonnage(userId, username, tonnage) {
        const existing = db.prepare('SELECT * FROM mining_race_weekly WHERE userId = ?').get(userId);
        if (existing) {
            const newTonnage = existing.weeklyTonnage + tonnage;
            db.prepare('UPDATE mining_race_weekly SET weeklyTonnage = ?, updatedAt = ? WHERE userId = ?')
                .run(newTonnage, new Date().toISOString(), userId);
        } else {
            db.prepare('INSERT INTO mining_race_weekly (userId, username, weeklyTonnage, updatedAt) VALUES (?, ?, ?, ?)')
                .run(userId, username, tonnage, new Date().toISOString());
        }
    },

    getWeeklyLeaderboard() {
        const rows = db.prepare('SELECT * FROM mining_race_weekly ORDER BY weeklyTonnage DESC LIMIT 10').all();
        if (rows.length === 0) {
            const nowStr = new Date().toISOString();
            const stmt = db.prepare('INSERT INTO mining_race_weekly (userId, username, weeklyTonnage, updatedAt) VALUES (?, ?, ?, ?)');
            stmt.run("CAT-458F", "mateus", 8450, nowStr);
            stmt.run("CB-9901", "carlos_operador", 6200, nowStr);
            stmt.run("CB-9902", "minerador_sp", 4890, nowStr);
            stmt.run("CB-9903", "frota_brasil", 3750, nowStr);
            stmt.run("CB-9904", "joao_escavadeira", 2900, nowStr);
            stmt.run("CB-9905", "ana_cat", 2100, nowStr);
            stmt.run("CB-9906", "pedro_br", 1850, nowStr);
            stmt.run("CB-9907", "lucas_cat", 1400, nowStr);
            return db.prepare('SELECT * FROM mining_race_weekly ORDER BY weeklyTonnage DESC LIMIT 10').all();
        }
        return rows;
    },

    resetAll() {
        db.exec('DELETE FROM users; DELETE FROM machines; DELETE FROM referrals; DELETE FROM transactions; DELETE FROM usage_logs; DELETE FROM security_logs;');
        // Re-executa seed inicial
        const insertUser = db.prepare(`
            INSERT INTO users (id, username, password, balance, xp, nivel, securityPassword, promoUnlocked, freeWithdrawActive, coletasCompleted, tratorRented, alreadySpunToday, createdAt)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `);
        insertUser.run("CAT-458F", "mateus", "admin", 2095.00, 247, 8, "123456", 0, 0, 4, 0, 1, new Date().toISOString());

        const insertMachine = db.prepare(`
            INSERT INTO machines (id, userId, name, cost, yieldAmount, lastCollectedAt, state)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        `);
        insertMachine.run("mach-1", "CAT-458F", "Escavadeira CAT 320", 2500, 110.00, new Date(Date.now() - 23.9 * 3600 * 1000).toISOString(), "aguardando");
        return { message: "Banco de dados SQLite resetado com sucesso!" };
    }
};

module.exports = sqliteDb;
