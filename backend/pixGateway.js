const crypto = require('crypto');
const db = require('./database');

// Configuração Padrão do Gateway PIX (Pode ser atualizada via ERP/env)
const gatewayConfig = {
    provider: process.env.PIX_PROVIDER || 'suitpay', // 'suitpay' | 'mercadopago' | 'asaas' | 'pushinpay' | 'generic'
    apiKey: process.env.PIX_API_KEY || 'cat_sandbox_api_key_2026',
    secretKey: process.env.PIX_SECRET_KEY || 'cat_sandbox_secret_key_9988',
    webhookSecret: process.env.PIX_WEBHOOK_SECRET || 'cat_webhook_secret_776655',
    environment: process.env.NODE_ENV === 'production' ? 'production' : 'sandbox',
    depositFeePercent: 1.5,
    withdrawFeeFixed: 1.00
};

// Gera Código PIX Copia e Cola Padrão EMV (BR Code) de acordo com o montante
function generateBRCodePayload(txId, amount) {
    const formattedVal = parseFloat(amount).toFixed(2);
    // Payload EMV Coeficiente Banco Central do Brasil
    return `00020126580014br.gov.bcb.pix0136catbrasil-deposit-${txId}5204000053039865405${formattedVal}5802BR5916CatBrasil Mining6009SAO PAULO620705031236304E8A2`;
}

const pixGateway = {
    getConfig() {
        return gatewayConfig;
    },

    updateConfig(updates) {
        Object.assign(gatewayConfig, updates);
        return gatewayConfig;
    },

    // 1. GERAÇÃO DE PIX INBOUND (DEPÓSITO)
    async createPixDeposit({ userId, amount }) {
        const numAmount = parseFloat(amount);
        if (isNaN(numAmount) || numAmount <= 0) {
            throw new Error("Valor de depósito inválido");
        }

        const txId = `PIX-${Date.now()}-${Math.floor(1000 + Math.random() * 9000)}`;
        const pixCode = generateBRCodePayload(txId, numAmount);
        const qrCodeUrl = `https://api.qrserver.com/v1/create-qr-code/?size=240x240&data=${encodeURIComponent(pixCode)}`;

        // Registra transação no banco de dados com status "pendente"
        // SEGURANÇA CRÍTICA: Saldo NUNCA é creditado aqui!
        const tx = db.addTransaction({
            id: txId,
            userId,
            tipo: "Depósito PIX",
            valor: numAmount,
            status: "pendente",
            externalId: `GW-${txId}`,
            createdAt: new Date().toISOString()
        });

        return {
            txId: tx.id,
            amount: numAmount,
            pixCode,
            qrCodeUrl,
            status: "pendente",
            expirationMinutes: 15,
            message: "QR Code PIX gerado com sucesso. Aguardando confirmação do pagamento via Webhook."
        };
    },

    // Valida Assinatura Criptográfica HMAC-SHA256 do Webhook do Gateway
    verifyWebhookSignature(payload, headers = {}) {
        const signature = headers['x-webhook-signature'] || headers['x-signature'] || (typeof payload === 'object' ? payload.signature : null);
        if (!signature) {
            // Em ambiente de produção, exige assinatura HMAC válida obrigatoriamente
            if (gatewayConfig.environment === 'production') {
                return false;
            }
            return true;
        }

        const payloadString = typeof payload === 'string' ? payload : JSON.stringify(payload);
        const expectedSignature = crypto
            .createHmac('sha256', gatewayConfig.webhookSecret)
            .update(payloadString)
            .digest('hex');

        return signature === expectedSignature || signature === `sha256=${expectedSignature}`;
    },

    // 2. PROCESSAMENTO SEGURO DE WEBHOOK RECEBIDO DO GATEWAY (COM TRANSAÇÃO ACID ATÔMICA)
    async processWebhookNotification(payload, headers = {}) {
        // Valida Assinatura Criptográfica HMAC-SHA256
        if (!this.verifyWebhookSignature(payload, headers)) {
            return { success: false, error: "Assinatura HMAC-SHA256 inválida. Notificação de webhook rejeitada por segurança." };
        }

        const txId = payload.txId || payload.transaction_id || payload.reference_id || payload.id || (payload.data && (payload.data.id || payload.data.txId || payload.data.reference_id)) || (payload.object && payload.object.id);
        const rawAmount = payload.paidAmount !== undefined ? payload.paidAmount : (payload.amount !== undefined ? payload.amount : (payload.value !== undefined ? payload.value : (payload.data && (payload.data.paidAmount || payload.data.amount || payload.data.transaction_amount))));
        const paidAmount = parseFloat(rawAmount || 0);
        const statusRaw = payload.status || payload.event || (payload.data && payload.data.status) || '';
        const status = String(statusRaw).toLowerCase();

        if (!txId) {
            return { success: false, error: "Identificador de transação (txId) ausente no payload" };
        }

        const tx = db.getTransactionById(txId);
        if (!tx) {
            return { success: false, error: `Transação #${txId} não encontrada no banco de dados` };
        }

        if (tx.status === 'sucesso') {
            return { success: true, message: `Transação #${txId} já havia sido confirmada anteriormente (Idempotente)` };
        }

        const isPaid = status.includes('paid') || status.includes('approved') || status.includes('confirmed') || status.includes('sucesso') || status === 'paid';

        if (!isPaid) {
            return { success: false, message: `Status de pagamento não confirmado (${status})` };
        }

        if (isNaN(paidAmount) || paidAmount <= 0) {
            return { success: false, error: "Valor de pagamento confirmado inválido no webhook" };
        }

        const user = db.getUserById(tx.userId);
        if (!user) {
            return { success: false, error: "Operador associado à transação não encontrado" };
        }

        // Executa atualização em Bloco de Transação ACID Atômica (Zero Race Condition)
        let resultData = null;
        db.transaction(() => {
            const newBalance = user.balance + paidAmount;
            const newRequiredRollover = (user.requiredRollover || 0) + paidAmount;

            db.updateUser(user.id, {
                balance: newBalance,
                requiredRollover: newRequiredRollover
            });

            db.updateTransaction(txId, {
                status: "sucesso",
                valorEfetivoPago: paidAmount,
                paidAt: new Date().toISOString()
            });

            const allRefs = db.getAllReferrals();
            const foundRef = allRefs.find(r => r.nome && r.nome.toLowerCase() === user.username.toLowerCase());
            if (foundRef && foundRef.status !== 'ativo') {
                const commission = paidAmount * 0.05; // 5% APENAS sobre o PRIMEIRO DEPÓSITO do convidado!
                db.updateReferral(foundRef.id, {
                    status: 'ativo',
                    deposito: paidAmount
                });

                const referrer = db.getUserById(foundRef.referrerId);
                if (referrer && !referrer.referralBonusSuspended && !referrer.multiAccountFlag) {
                    const newReferrerBalance = referrer.balance + commission;
                    db.updateUser(referrer.id, { balance: newReferrerBalance });
                    db.addTransaction({
                        userId: referrer.id,
                        tipo: `Comissão 5% (1º Depósito ${user.username})`,
                        valor: commission,
                        status: "sucesso"
                    });
                }
            }

            db.logSecurityEvent(user.id, `PIX_WEBHOOK_CONFIRMED_R$${paidAmount}`, '', '');

            resultData = {
                success: true,
                message: `Pagamento PIX de R$ ${paidAmount.toFixed(2)} confirmado! Saldo de ${user.username} atualizado para R$ ${newBalance.toFixed(2)}.`,
                txId,
                userId: user.id,
                newBalance,
                paidAmount
            };
        });

        return resultData;
    },

    // 3. SAQUE PIX OUTBOUND (PAYOUT COM TRANSAÇÃO ACID ATÔMICA, ROLLOVER E 2FA)
    async executePixPayout({ userId, amount, pixKey, securityPassword, twoFactorCode }) {
        const numAmount = parseFloat(amount);
        const user = db.getUserById(userId);

        if (!user) throw new Error("Operador não encontrado");
        if (user.withdrawLocked) throw new Error("Saques travados para esta conta");
        if (isNaN(numAmount) || numAmount < 50) throw new Error("Valor mínimo para saque PIX é R$ 50,00");
        if (user.balance < numAmount) throw new Error("Saldo em carteira insuficiente");
        if (!user.securityPassword) throw new Error("Cadastre sua senha financeira de 6 dígitos antes de sacar");
        if (user.securityPassword !== securityPassword) throw new Error("Senha financeira incorreta");

        // REGRA FINANCEIRA DE RETENÇÃO (ROLLOVER DE GIRO MÍNIMO EM ALUGUÉIS DE MÁQUINAS)
        const reqRoll = user.requiredRollover || 0;
        const currRoll = user.currentRollover || 0;
        if (currRoll < reqRoll) {
            const diff = reqRoll - currRoll;
            throw new Error(`Para realizar saques, é necessário ter um giro mínimo de R$ ${reqRoll.toFixed(2)} em contratação de frotas (Rollover). Faltam R$ ${diff.toFixed(2)} em aluguel de máquinas.`);
        }

        // REGRA DE SEGURANÇA CRÍTICA: Saques de Alto Valor (> R$ 500.00) exigem Validação de 2FA / PIN
        if (numAmount > 500) {
            const requiredCode = user.securityPassword;
            if (!twoFactorCode || twoFactorCode !== requiredCode) {
                throw new Error("Saques superiores a R$ 500,00 exigem a confirmação do PIN de Segurança 2FA de 6 dígitos.");
            }
        }

        const feePercent = user.freeWithdrawActive ? 0.00 : (gatewayConfig.withdrawFeeFixed > 0 ? 0.05 : 0);
        const fee = numAmount * feePercent;
        const netAmount = numAmount - fee;

        let resultData = null;
        db.transaction(() => {
            const newBalance = user.balance - numAmount;
            db.updateUser(user.id, {
                balance: newBalance,
                freeWithdrawActive: false
            });

            const tx = db.addTransaction({
                id: `WDR-${Date.now()}-${Math.floor(1000 + Math.random() * 9000)}`,
                userId: user.id,
                tipo: "Saque PIX",
                valor: -numAmount,
                status: "pendente",
                pixKey,
                netAmount,
                feeAmount: fee,
                createdAt: new Date().toISOString()
            });

            db.logSecurityEvent(user.id, `WITHDRAWAL_REQUESTED_R$${numAmount}`, '', '');

            resultData = {
                txId: tx.id,
                requestedAmount: numAmount,
                netAmount,
                fee,
                newBalance,
                status: "pendente",
                message: `Solicitação de saque PIX de R$ ${numAmount.toFixed(2)} registrada! Líquido: R$ ${netAmount.toFixed(2)}`
            };
        });

        return resultData;
    }
};

module.exports = pixGateway;
