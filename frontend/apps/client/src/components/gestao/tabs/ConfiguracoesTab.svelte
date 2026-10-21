<script>
    export let gestaoConfig = {};
    export let gwDepositFee = 1.5;
    export let gwWithdrawFee = 1.00;
    export let gwPixExpMinutes = 15;
    export let suitpayClientId = "";
    export let suitpayClientSecret = "";
    export let suitpaySandbox = false;
    export let suitpayCallbackUrl = "";
    export let suitpayEnabled = false;

    export let onToggleGlobalLock = () => {};
    export let onSaveAutoApproveSettings = () => {};
    export let onSaveGatewaySettings = () => {};
</script>

<!-- ABA 8: CONFIGURAÇÕES GLOBAIS DA BANCA & KILL SWITCH -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- PanelHeader -->
    <div style="border-bottom: 1px solid #e2e8f0; background: linear-gradient(to right, #f8fafc, #ffffff); padding: 12px 16px; display: flex; align-items: center; justify-content: space-between;">
        <div style="display: flex; align-items: center; gap: 12px;">
            <span style="width: 24px; height: 24px; background: #b91c1c; color: #fff; font-family: monospace; font-size: 0.7rem; font-weight: 700; display: flex; align-items: center; justify-content: center;">⚙️</span>
            <div>
                <h3 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">CONFIGURAÇÕES GLOBAIS & TRAVAS DE SEGURANÇA</h3>
                <p style="font-size: 0.68rem; color: #64748b; margin: 2px 0 0 0;">Kill Switch de emergência, SuitPay Gateway PIX e limites de aprovação automática</p>
            </div>
        </div>
        <span style="font-family: monospace; font-size: 0.72rem; font-weight: 700; padding: 4px 10px; border: 1px solid {gestaoConfig.globalWithdrawalsLocked ? '#b91c1c' : '#047857'}; background: {gestaoConfig.globalWithdrawalsLocked ? '#fef2f2' : '#ecfdf5'}; color: {gestaoConfig.globalWithdrawalsLocked ? '#991b1b' : '#065f46'};">
            {gestaoConfig.globalWithdrawalsLocked ? 'KILL SWITCH ATIVO (SAQUES TRAVADOS)' : 'STATUS OPERACIONAL: NORMAL'}
        </span>
    </div>

    <!-- BLOCO 1: KILL SWITCH GLOBAL DE SAQUES -->
    <div style="background: #fff; border: 2px solid {gestaoConfig.globalWithdrawalsLocked ? '#b91c1c' : '#e2e8f0'}; padding: 16px;">
        <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #b91c1c; text-transform: uppercase; margin: 0 0 6px 0;">🚨 KILL SWITCH DE SAQUES (TRAVA DE EMERGÊNCIA GLOBAL)</h4>
        <p style="font-size: 0.72rem; color: #475569; margin: 0 0 14px 0;">Ao acionar o Kill Switch, todas as retiradas da plataforma serão suspensas imediatamente para prevenir drenagem de liquidez.</p>
        
        <button 
            on:click={onToggleGlobalLock}
            style="font-family: monospace; font-size: 0.8rem; font-weight: 900; text-transform: uppercase; padding: 14px 20px; border: 2px solid #000; background: {gestaoConfig.globalWithdrawalsLocked ? '#047857' : '#b91c1c'}; color: #fff; cursor: pointer; width: 100%;"
        >
            {gestaoConfig.globalWithdrawalsLocked ? '✅ LIBERAR SAQUES GLOBAIS (DESATIVAR KILL SWITCH)' : '🚫 ACIONAR KILL SWITCH (BLOQUEAR TODOS OS SAQUES)'}
        </button>
    </div>

    <!-- BLOCO 2: APROVAÇÃO AUTOMÁTICA DE SAQUES -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 6px 0;">⚡ REGRAS DE APROVAÇÃO AUTOMÁTICA DE SAQUES</h4>
        <p style="font-size: 0.72rem; color: #475569; margin: 0 0 12px 0;">Define se solicitações de saque de pequeno valor são autorizadas automaticamente pelo gateway sem necessidade de moderação humana manual.</p>

        <div style="background: #f8fafc; border: 1px solid #e2e8f0; padding: 14px; display: flex; flex-direction: column; gap: 12px;">
            <div style="display: flex; align-items: center; gap: 10px;">
                <input id="cfg-chk-auto" type="checkbox" bind:checked={gestaoConfig.autoApproveEnabled} style="width: 18px; height: 18px;" />
                <label for="cfg-chk-auto" style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; cursor: pointer;">
                    HABILITAR APROVAÇÃO AUTOMÁTICA VIA GATEWAY PIX
                </label>
            </div>

            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px; align-items: flex-end;">
                <div>
                    <label for="cfg-lim-inp" style="font-size: 0.6rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 4px;">
                        LIMITE MÁXIMO PARA APROVAÇÃO AUTOMÁTICA (R$):
                    </label>
                    <input id="cfg-lim-inp" type="number" step="10" bind:value={gestaoConfig.autoApproveWithdrawalsUnder} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.85rem; font-weight: 700; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
                <div>
                    <button on:click={onSaveAutoApproveSettings} style="width: 100%; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.75rem; font-weight: 700; text-transform: uppercase; padding: 9px; border: 1px solid #000; cursor: pointer;">
                        SALVAR LIMITE DE APROVAÇÃO
                    </button>
                </div>
            </div>
        </div>
    </div>

    <!-- BLOCO 3: INTEGRAÇÃO OFICIAL SUITPAY GATEWAY -->
    <div style="background: #fff; border: 2px solid {suitpayEnabled ? '#047857' : '#e2e8f0'}; padding: 16px;">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px;">
            <div>
                <h4 style="font-family: monospace; font-size: 0.8rem; font-weight: 900; color: #0f172a; text-transform: uppercase; margin: 0 0 4px 0;">💳 INTEGRAÇÃO OFICIAL SUITPAY (PIX CASH-IN & CASH-OUT)</h4>
                <p style="font-size: 0.72rem; color: #475569; margin: 0;">Conecte sua conta da SuitPay para geração de QR Codes dinâmicos e liquidação de saques automáticos.</p>
            </div>
            <span style="font-family: monospace; font-size: 0.68rem; font-weight: 700; padding: 4px 8px; border: 1px solid {suitpayEnabled && suitpayClientId ? '#047857' : '#d97706'}; background: {suitpayEnabled && suitpayClientId ? '#ecfdf5' : '#fffbeb'}; color: {suitpayEnabled && suitpayClientId ? '#065f46' : '#b45309'};">
                {suitpayEnabled && suitpayClientId ? '🟢 SUITPAY ATIVO' : '🟡 MODO SIMULADO / AGUARDANDO CHAVES'}
            </span>
        </div>

        <!-- Aviso Informativo -->
        <div style="background: #eff6ff; border-left: 4px solid #3b82f6; padding: 10px 14px; margin-bottom: 14px; font-size: 0.7rem; color: #1e40af; line-height: 1.4;">
            💡 <strong>Pronto para Ativação:</strong> Assim que criar sua conta na SuitPay, basta colar o <strong>Client ID (ci)</strong> e <strong>Client Secret (cs)</strong> abaixo e clicar em <em>Salvar</em>. O sistema mudará para a API ao vivo instantaneamente. Enquanto isso, depósitos e saques funcionam no gerador EMV nativo.
        </div>

        <div style="background: #f8fafc; border: 1px solid #e2e8f0; padding: 14px; display: flex; flex-direction: column; gap: 12px; margin-bottom: 14px;">
            <div style="display: flex; align-items: center; gap: 10px;">
                <input id="cfg-chk-suitpay" type="checkbox" bind:checked={suitpayEnabled} style="width: 18px; height: 18px;" />
                <label for="cfg-chk-suitpay" style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; cursor: pointer;">
                    HABILITAR SUITPAY COMO GATEWAY PRINCIPAL
                </label>
            </div>

            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
                <div>
                    <label for="cfg-sp-ci" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">CLIENT ID (ci):</label>
                    <input id="cfg-sp-ci" type="text" placeholder="Ex: suitpay_client_..." bind:value={suitpayClientId} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.78rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
                <div>
                    <label for="cfg-sp-cs" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">CLIENT SECRET (cs):</label>
                    <input id="cfg-sp-cs" type="password" placeholder="••••••••••••••••" bind:value={suitpayClientSecret} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.78rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
            </div>

            <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 12px; align-items: center;">
                <div>
                    <label for="cfg-sp-cb" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">URL DE CALLBACK DO WEBHOOK:</label>
                    <input id="cfg-sp-cb" type="text" placeholder="https://api.catbrasil.app/api/v1/webhook/suitpay" bind:value={suitpayCallbackUrl} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.78rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
                <div style="display: flex; align-items: center; gap: 8px; margin-top: 14px;">
                    <input id="cfg-chk-sandbox" type="checkbox" bind:checked={suitpaySandbox} style="width: 16px; height: 16px;" />
                    <label for="cfg-chk-sandbox" style="font-family: monospace; font-size: 0.7rem; font-weight: 700; color: #475569; cursor: pointer;">
                        MODO SANDBOX (TESTES)
                    </label>
                </div>
            </div>
        </div>

        <!-- Taxas e Expiração -->
        <h5 style="font-family: monospace; font-size: 0.7rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 8px 0;">TAXAS E EXPIRAÇÃO CONTRATUAL:</h5>
        <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; margin-bottom: 14px;">
            <div>
                <label for="cfg-gw-dep-fee" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">TAXA DEPÓSITO (%):</label>
                <input id="cfg-gw-dep-fee" type="number" step="0.1" bind:value={gwDepositFee} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.78rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>
            <div>
                <label for="cfg-gw-wdr-fee" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">TAXA SAQUE FIXA (R$):</label>
                <input id="cfg-gw-wdr-fee" type="number" step="0.5" bind:value={gwWithdrawFee} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.78rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>
            <div>
                <label for="cfg-gw-exp" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">EXPIRAÇÃO PIX (MINUTOS):</label>
                <input id="cfg-gw-exp" type="number" bind:value={gwPixExpMinutes} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.78rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>
        </div>

        <button on:click={onSaveGatewaySettings} style="width: 100%; background: #047857; color: #fff; font-family: monospace; font-size: 0.78rem; font-weight: 700; text-transform: uppercase; padding: 12px; border: 1px solid #000; cursor: pointer;">
            SALVAR PARÂMETROS SUITPAY & TAXAS NO BANCO
        </button>
    </div>

</div>
