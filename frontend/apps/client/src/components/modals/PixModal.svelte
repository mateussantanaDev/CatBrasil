<script>
    import * as QRCode from 'qrcode';
    export let show = false;
    export let currentPixValue = 0;
    export let pixCodeString = "";
    export let pixTimerSeconds = 900;
    export let onClose = () => {};
    export let onCopyCode = () => {};

    let copied = false;
    let qrCodeUrl = '';
    $: if (show && pixCodeString) {
        const currentCode = pixCodeString;
        QRCode.toDataURL(currentCode, { width: 180, margin: 1 }).then((url) => {
            if (pixCodeString === currentCode) qrCodeUrl = url;
        });
    }

    function handleCopy() {
        onCopyCode();
        copied = true;
        setTimeout(() => copied = false, 2500);
    }

    function formatTimer(sec) {
        const m = Math.floor(sec / 60);
        const s = sec % 60;
        return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    }
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 390px; background: #0f172a; color: #fff; border: 2px solid #ffb200; border-radius: 12px; overflow: hidden; box-shadow: 0 10px 30px rgba(0,0,0,0.6);">
            <!-- Header Modal -->
            <div style="background: #1e293b; padding: 12px 16px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #334155;">
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="font-size: 1.1rem;">⚡</span>
                    <h3 style="font-family: monospace; font-size: 0.85rem; font-weight: 900; color: #ffb200; margin: 0; text-transform: uppercase;">PAGAMENTO PIX INSTANTÂNEO</h3>
                </div>
                <button class="modal-close" on:click={onClose} style="background: transparent; border: none; color: #94a3b8; font-size: 1.1rem; cursor: pointer;">✕</button>
            </div>

            <div class="modal-body" style="padding: 16px; text-align: center;">
                <!-- Contador de Expiração -->
                {#if pixTimerSeconds > 0}<div style="display: inline-block; background: #1e293b; border: 1px solid #334155; padding: 4px 12px; border-radius: 20px; font-family: monospace; font-size: 0.72rem; color: #94a3b8; margin-bottom: 12px;">
                    EXPIRA EM: <strong style="color: #ffb200; font-size: 0.82rem;">{formatTimer(pixTimerSeconds)}</strong>
                </div>{:else}<p>Confirme o prazo da cobrança no aplicativo do seu banco.</p>{/if}

                <div style="margin-bottom: 12px;">
                    <span style="font-size: 0.7rem; color: #94a3b8; font-family: monospace; display: block;">VALOR DO DEPÓSITO:</span>
                    <strong style="font-family: monospace; font-size: 1.4rem; color: #10b981; font-weight: 900;">
                        R$ {currentPixValue.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                    </strong>
                </div>

                <!-- QR CODE Image -->
                <div style="background: #fff; padding: 12px; border-radius: 8px; display: inline-block; border: 2px solid #ffb200; box-shadow: 0 4px 12px rgba(0,0,0,0.3); margin-bottom: 10px;">
                    <img 
                        src={qrCodeUrl}
                        alt="QR Code PIX" 
                        style="width: 160px; height: 160px; display: block;"
                    />
                </div>

                <p style="font-size: 0.68rem; color: #94a3b8; font-family: monospace; margin: 4px 0 10px 0;">
                    Escaneie pelo aplicativo do seu banco ou use a chave Copia e Cola:
                </p>

                <!-- Input Copia e Cola -->
                <div style="display: flex; flex-direction: column; gap: 8px;">
                    <textarea 
                        readonly 
                        class="pix-code-area" 
                        value={pixCodeString}
                        style="width: 100%; height: 50px; background: #020617; border: 1px solid #334155; color: #cbd5e1; font-family: monospace; font-size: 0.62rem; padding: 8px; border-radius: 6px; resize: none; box-sizing: border-box;"
                    ></textarea>

                    <button 
                        class="btn btn-primary" 
                        on:click={handleCopy} 
                        style="background: {copied ? '#10b981' : '#ffb200'}; color: #000; font-family: monospace; font-size: 0.8rem; font-weight: 900; padding: 10px; border: 2px solid #000; border-radius: 6px; cursor: pointer; text-transform: uppercase;"
                    >
                        {copied ? '✅ CÓDIGO PIX COPIADO!' : 'COPIAR CÓDIGO PIX'}
                    </button>
                </div>

                <!-- Live status -->
                <div style="margin-top: 12px; display: flex; align-items: center; justify-content: center; gap: 6px; font-size: 0.65rem; color: #64748b; font-family: monospace;">
                    <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #10b981; animation: pulse 1.5s infinite;"></span>
                    Aguardando confirmação automática do gateway...
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    @keyframes pulse {
        0%, 100% { opacity: 1; transform: scale(1); }
        50% { opacity: 0.4; transform: scale(0.85); }
    }
</style>
