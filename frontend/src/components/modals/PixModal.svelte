<script>
    export let show = false;
    export let currentPixValue = 0;
    export let pixCodeString = "";
    export let pixTimerSeconds = 900;
    export let onClose = () => {};
    export let onCopyCode = () => {};

    function formatTimer(sec) {
        const m = Math.floor(sec / 60);
        const s = sec % 60;
        return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    }
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 380px;">
            <div class="modal-header">
                <h3>PAGAMENTO PIX INSTANTÂNEO</h3>
                <button class="modal-close" on:click={onClose}>✕</button>
            </div>
            <div class="modal-body" style="text-align: center;">
                <div class="pix-timer-badge">
                    EXPIRA EM: <strong>{formatTimer(pixTimerSeconds)}</strong>
                </div>

                <p style="font-size: 0.75rem; color: #555; margin-bottom: 8px;">
                    Valor do Depósito: <strong class="mono-text highlight-green">R$ {currentPixValue.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong>
                </p>

                <!-- QR CODE Placeholder -->
                <div class="qr-container">
                    <img 
                        src="https://api.qrserver.com/v1/create-qr-code/?size=180x180&data={encodeURIComponent(pixCodeString)}" 
                        alt="QR Code PIX" 
                        style="width: 160px; height: 160px; border: 2px solid var(--cat-black);"
                    />
                </div>

                <p style="font-size: 0.65rem; color: #666; margin-top: 6px;">Escaneie o QR Code acima no app do seu banco ou use a chave Copia e Cola:</p>

                <!-- Input Copia e Cola -->
                <div style="margin-top: 10px;">
                    <textarea readonly class="pix-code-area" value={pixCodeString}></textarea>
                    <button class="btn btn-primary" on:click={onCopyCode} style="margin-top: 6px;">
                        COPIAR CÓDIGO PIX
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}
