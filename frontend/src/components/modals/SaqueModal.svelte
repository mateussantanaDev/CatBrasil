<script>
    export let show = false;
    export let user = {};
    export let withdrawAmount = "";
    export let withdrawPixKey = "";
    export let withdrawSecurityPass = "";
    export let onClose = () => {};
    export let onSubmitWithdrawal = () => {};
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 380px;">
            <div class="modal-header">
                <h3>SOLICITAÇÃO DE SAQUE PIX</h3>
                <button class="modal-close" on:click={onClose}>✕</button>
            </div>
            <div class="modal-body">
                <div class="data-row" style="margin-bottom: 10px;">
                    <span>Saldo Disponível:</span>
                    <strong class="mono-text highlight-green">R$ {user.balance ? user.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</strong>
                </div>

                <div style="margin-bottom: 10px;">
                    <label class="input-field-label" for="wdr-amount">Valor do Saque (R$):</label>
                    <input id="wdr-amount" type="number" placeholder="0.00" class="input-field" bind:value={withdrawAmount}>
                </div>

                <div style="margin-bottom: 10px;">
                    <label class="input-field-label" for="wdr-pix">Chave PIX (CPF / Celular / Email / Aleatória):</label>
                    <input id="wdr-pix" type="text" placeholder="Digite sua chave PIX" class="input-field" bind:value={withdrawPixKey}>
                </div>

                <div style="margin-bottom: 12px;">
                    <label class="input-field-label" for="wdr-pass">Senha Financeira de 6 Dígitos:</label>
                    <input id="wdr-pass" type="password" maxlength="6" placeholder="******" class="input-field" bind:value={withdrawSecurityPass} style="letter-spacing: 4px; font-weight: 800; text-align: center;">
                </div>

                <button class="btn btn-primary" on:click={onSubmitWithdrawal}>
                    CONFIRMAR RETIRADA
                </button>
            </div>
        </div>
    </div>
{/if}
