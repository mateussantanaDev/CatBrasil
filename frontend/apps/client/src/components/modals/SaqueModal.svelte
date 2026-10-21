<script>
    export let show = false;
    export let user = {};
    export let withdrawAmount = "";
    export let withdrawPixKey = "";
    export let withdrawSecurityPass = "";
    export let withdrawTwoFactorCode = "";
    export let onClose = () => {};
    export let onSubmitWithdrawal = () => {};

    let isSubmitting = false;

    $: numAmount = parseFloat(withdrawAmount) || 0;
    $: isFreeWithdraw = user?.freeWithdrawActive;
    $: feePercent = isFreeWithdraw ? 0 : 0.05;
    $: feeValue = numAmount * feePercent;
    $: netAmount = Math.max(0, numAmount - feeValue);
    $: needs2FA = numAmount > 500 && Boolean(user?.twoFactorEnabled);

    async function handleConfirm() {
        if (isSubmitting) return;
        isSubmitting = true;
        try {
            await onSubmitWithdrawal();
        } finally {
            isSubmitting = false;
        }
    }
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 400px; background: #0f172a; color: #fff; border: 2px solid #ffb200; border-radius: 12px; overflow: hidden; box-shadow: 0 10px 30px rgba(0,0,0,0.6);">
            <!-- Modal Header -->
            <div style="background: #1e293b; padding: 12px 16px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #334155;">
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="font-size: 1.1rem;">💸</span>
                    <h3 style="font-family: monospace; font-size: 0.85rem; font-weight: 900; color: #ffb200; margin: 0; text-transform: uppercase; letter-spacing: 0.5px;">SOLICITAÇÃO DE SAQUE PIX</h3>
                </div>
                <button class="modal-close" on:click={onClose} style="background: transparent; border: none; color: #94a3b8; font-size: 1.1rem; cursor: pointer;">✕</button>
            </div>

            <div class="modal-body" style="padding: 16px;">
                <!-- Saldo do Usuário -->
                <div style="background: #1e293b; border: 1px solid #334155; padding: 10px 14px; border-radius: 8px; display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px;">
                    <span style="font-size: 0.72rem; color: #94a3b8; font-family: monospace;">SALDO DISPONÍVEL:</span>
                    <strong style="font-family: monospace; font-size: 0.95rem; color: #10b981; font-weight: 900;">
                        R$ {user.balance ? user.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
                    </strong>
                </div>

                <!-- Campo Valor -->
                <div style="margin-bottom: 12px;">
                    <label class="input-field-label" for="wdr-amount" style="font-size: 0.65rem; color: #94a3b8; font-family: monospace; display: block; margin-bottom: 4px;">
                        VALOR DO SAQUE (MÍNIMO R$ 50,00):
                    </label>
                    <input 
                        id="wdr-amount" 
                        type="number" 
                        step="10"
                        placeholder="Ex: 100.00" 
                        class="input-field" 
                        bind:value={withdrawAmount}
                        style="width: 100%; padding: 10px; background: #020617; border: 1px solid #334155; color: #fff; font-family: monospace; font-size: 0.9rem; font-weight: 700; border-radius: 6px; box-sizing: border-box;"
                    />
                </div>

                <!-- Campo Chave PIX -->
                <div style="margin-bottom: 12px;">
                    <label class="input-field-label" for="wdr-pix" style="font-size: 0.65rem; color: #94a3b8; font-family: monospace; display: block; margin-bottom: 4px;">
                        CHAVE PIX (CPF / TELEFONE / EMAIL / EVP):
                    </label>
                    <input 
                        id="wdr-pix" 
                        type="text" 
                        placeholder="Digite sua chave PIX cadastrada" 
                        class="input-field" 
                        bind:value={withdrawPixKey}
                        style="width: 100%; padding: 10px; background: #020617; border: 1px solid #334155; color: #fff; font-family: monospace; font-size: 0.82rem; border-radius: 6px; box-sizing: border-box;"
                    />
                </div>

                <!-- Campo Senha Financeira -->
                <div style="margin-bottom: 14px;">
                    <label class="input-field-label" for="wdr-pass" style="font-size: 0.65rem; color: #94a3b8; font-family: monospace; display: block; margin-bottom: 4px;">
                        SENHA FINANCEIRA (PIN DE 6 DÍGITOS):
                    </label>
                    <input 
                        id="wdr-pass" 
                        type="password" 
                        maxlength="6" 
                        placeholder="••••••" 
                        class="input-field" 
                        bind:value={withdrawSecurityPass} 
                        style="width: 100%; padding: 10px; background: #020617; border: 1px solid #334155; color: #ffb200; font-family: monospace; font-size: 1rem; font-weight: 900; letter-spacing: 6px; text-align: center; border-radius: 6px; box-sizing: border-box;"
                    />
                </div>

                <!-- Campo 2FA Google Authenticator (se valor > 500 e 2FA ativo) -->
                {#if needs2FA}
                    <div style="margin-bottom: 14px;">
                        <label class="input-field-label" for="wdr-2fa" style="font-size: 0.65rem; color: #fbbf24; font-family: monospace; display: block; margin-bottom: 4px; font-weight: 700;">
                            CÓDIGO 2FA GOOGLE AUTHENTICATOR (VALOR > R$ 500):
                        </label>
                        <input 
                            id="wdr-2fa" 
                            type="text" 
                            maxlength="6" 
                            placeholder="000000" 
                            class="input-field" 
                            bind:value={withdrawTwoFactorCode} 
                            style="width: 100%; padding: 10px; background: #020617; border: 1px solid #fbbf24; color: #fbbf24; font-family: monospace; font-size: 1rem; font-weight: 900; letter-spacing: 6px; text-align: center; border-radius: 6px; box-sizing: border-box;"
                        />
                    </div>
                {/if}

                <!-- Resumo Financeiro Dinâmico -->
                {#if numAmount > 0}
                    <div style="background: #020617; border: 1px dashed #475569; padding: 10px 12px; border-radius: 6px; margin-bottom: 14px; font-family: monospace; font-size: 0.68rem; display: flex; flex-direction: column; gap: 4px;">
                        <div style="display: flex; justify-content: space-between; color: #94a3b8;">
                            <span>Taxa de Processamento {isFreeWithdraw ? '(Primeiro Saque Grátis)' : '(5%)'}:</span>
                            <span style="color: {isFreeWithdraw ? '#10b981' : '#f87171'};">
                                {isFreeWithdraw ? 'R$ 0,00' : `- R$ ${feeValue.toFixed(2)}`}
                            </span>
                        </div>
                        <div style="display: flex; justify-content: space-between; color: #fff; font-weight: 700; border-top: 1px solid #1e293b; padding-top: 4px; margin-top: 2px;">
                            <span>LÍQUIDO A RECEBER:</span>
                            <strong style="color: #ffb200; font-size: 0.82rem;">R$ {netAmount.toFixed(2)}</strong>
                        </div>
                    </div>
                {/if}

                <!-- Botão de Confirmação -->
                <button 
                    class="btn btn-primary" 
                    on:click={handleConfirm}
                    disabled={isSubmitting || numAmount < 50}
                    style="width: 100%; background: {numAmount >= 50 && !isSubmitting ? '#ffb200' : '#475569'}; color: #000; font-family: monospace; font-weight: 900; font-size: 0.85rem; padding: 12px; border: 2px solid #000; border-radius: 6px; cursor: {numAmount >= 50 && !isSubmitting ? 'pointer' : 'not-allowed'}; text-transform: uppercase;"
                >
                    {isSubmitting ? '⏳ PROCESSANDO RETIRADA...' : 'SOLICITAR SAQUE PIX'}
                </button>
            </div>
        </div>
    </div>
{/if}
