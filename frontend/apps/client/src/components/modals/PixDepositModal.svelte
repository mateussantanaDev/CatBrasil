<script>
  import { api } from '../../services/api.js';
  import { authState } from '../../state/auth.svelte.js';
  import { toastState } from '../../state/toast.svelte.js';
  import { Modal, Button } from '@catbrasil/ui';
  import * as QRCode from 'qrcode';

  let {
    isOpen = false,
    onclose
  } = $props();

  const presets = [50, 100, 250, 500, 1000];
  let selectedAmount = $state(100);
  let customAmount = $state('');
  let isLoading = $state(false);
  let pixData = $state(null); // { txId, amount, pixCode, qrCodeUrl }

  function handleSelectPreset(val) {
    selectedAmount = val;
    customAmount = '';
  }

  function handleCustomInput(e) {
    const val = parseFloat(e.target.value);
    if (!isNaN(val) && val > 0) {
      selectedAmount = val;
    }
  }

  async function generatePix() {
    const amount = customAmount ? parseFloat(customAmount) : selectedAmount;
    if (!amount || amount <= 0) {
      toastState.warning('Selecione ou digite um valor válido para depósito.');
      return;
    }

    try {
      isLoading = true;
      const res = await api.post('/financial/deposit/pix', {
        userId: authState.user?.id,
        amount: amount,
      });

      pixData = { ...res, qrCodeUrl: await QRCode.toDataURL(res.pixCode, { width: 240, margin: 1 }) };
      toastState.success('QR Code PIX gerado! Pague no seu banco.');
    } catch (err) {
      toastState.error(err.message || 'Erro ao gerar PIX.');
    } finally {
      isLoading = false;
    }
  }

  function copyPixCode() {
    if (pixData?.pixCode) {
      navigator.clipboard.writeText(pixData.pixCode);
      toastState.success('Código PIX Copia e Cola copiado!');
    }
  }

  function handleCloseModal() {
    pixData = null;
    if (onclose) onclose();
  }
</script>

<Modal {isOpen} title="DEPOSITAR SALDO VIA PIX" onclose={handleCloseModal} maxWidth="max-w-md">
  <div class="pix-modal-body">
    {#if !pixData}
      <div class="step-form">
        <span class="step-lbl">1. SELECIONE O VALOR DO DEPÓSITO:</span>

        <div class="presets-grid">
          {#each presets as val}
            <button
              class="btn-preset {selectedAmount === val && !customAmount ? 'active' : ''}"
              onclick={() => handleSelectPreset(val)}
            >
              R$ {val}
            </button>
          {/each}
        </div>

        <div class="custom-val-group">
          <label for="custom-pix-amount" class="custom-lbl">OU DIGITE OUTRO VALOR (R$):</label>
          <input
            id="custom-pix-amount"
            type="number"
            step="10"
            min="10"
            class="input-cat font-mono"
            placeholder="ex: 350.00"
            bind:value={customAmount}
            oninput={handleCustomInput}
          />
        </div>

        <div class="deposit-notice font-mono">
          <span>⚡ Liberação Imediata: Processamento bancário em tempo real via PIX 24/7.</span>
        </div>

        <button
          class="btn-cat btn-generate-pix"
          disabled={isLoading}
          onclick={generatePix}
        >
          {#if isLoading}
            <span>⚙️ GERANDO QR CODE PIX...</span>
          {:else}
            <span>GERAR PAGAMENTO PIX (R$ {customAmount ? customAmount : selectedAmount}) ⚡</span>
          {/if}
        </button>
      </div>
    {:else}
      <div class="step-qr">
        <div class="qr-box">
          <img src={pixData.qrCodeUrl} alt="QR Code PIX" class="qr-image" />
        </div>

        <div class="pix-info font-mono">
          <span class="pix-amount-label">VALOR A PAGAR:</span>
          <strong class="pix-amount-val text-yellow">R$ {pixData.amount.toFixed(2)}</strong>
          <span class="pix-timer">{pixData.expirationMinutes > 0 ? `Expira em ${pixData.expirationMinutes} minutos` : 'Confirme o prazo no aplicativo do seu banco'}</span>
        </div>

        <div class="copy-box">
          <label for="pix-emv-code" class="copy-lbl">PIX COPIA E COLA:</label>
          <div class="copy-input-row font-mono">
            <input
              id="pix-emv-code"
              type="text"
              class="input-cat input-emv text-truncate"
              readonly
              value={pixData.pixCode}
            />
            <button class="btn-copy-pix" onclick={copyPixCode}>COPIAR</button>
          </div>
        </div>

        <div class="payment-instructions">
          <p>1. Abra o aplicativo do seu banco</p>
          <p>2. Escolha <strong>Pagar com PIX</strong> e selecione <strong>QR Code</strong> ou <strong>Copia e Cola</strong></p>
          <p>3. Seu saldo será creditado automaticamente assim que o pagamento for concluído!</p>
        </div>

        <button class="btn-cat btn-done" onclick={handleCloseModal}>
          CONCLUIR / FECHAR
        </button>
      </div>
    {/if}
  </div>
</Modal>

<style>
  .pix-modal-body {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .step-form, .step-qr {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .step-lbl {
    font-size: 0.7rem;
    font-weight: 700;
    color: #bbb;
    letter-spacing: 0.5px;
  }

  .presets-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }

  .btn-preset {
    background: #202020;
    color: #fff;
    border: 1.5px solid #333;
    padding: 10px;
    font-family: var(--font-heading);
    font-size: 0.95rem;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.1s ease;
  }

  .btn-preset.active {
    background: var(--cat-yellow);
    color: #000;
    border-color: #000;
    box-shadow: 2px 2px 0px #000;
  }

  .custom-val-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .custom-lbl {
    font-size: 0.65rem;
    color: #888;
    font-weight: 700;
  }

  .deposit-notice {
    background: #111;
    border: 1px solid #282828;
    padding: 8px 10px;
    font-size: 0.65rem;
    color: #aaa;
  }

  .btn-generate-pix {
    width: 100%;
    padding: 14px;
  }

  .qr-box {
    display: flex;
    justify-content: center;
    background: #fff;
    padding: 14px;
    border: 2px solid #000;
    max-width: 220px;
    margin: 0 auto;
  }

  .qr-image {
    width: 100%;
    height: auto;
  }

  .pix-info {
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .pix-amount-label {
    font-size: 0.65rem;
    color: #888;
  }

  .pix-amount-val {
    font-size: 1.4rem;
  }

  .pix-timer {
    font-size: 0.7rem;
    color: #ff9100;
  }

  .copy-box {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .copy-lbl {
    font-size: 0.65rem;
    color: #888;
    font-weight: 700;
  }

  .copy-input-row {
    display: flex;
    gap: 6px;
  }

  .input-emv {
    flex: 1;
    font-size: 0.75rem;
    background: #0d0d0d;
  }

  .btn-copy-pix {
    background: var(--cat-yellow);
    color: #000;
    border: 1px solid #000;
    font-family: var(--font-heading);
    font-size: 0.8rem;
    font-weight: 700;
    padding: 0 14px;
    cursor: pointer;
  }

  .payment-instructions {
    background: #121212;
    border: 1px solid #282828;
    padding: 10px 12px;
    font-size: 0.7rem;
    color: #999;
    line-height: 1.5;
  }

  .btn-done {
    width: 100%;
    background: #252525;
    color: #fff;
    border-color: #383838;
  }

  .btn-done:hover {
    background: var(--cat-yellow);
    color: #000;
  }

  .text-truncate {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .text-yellow { color: var(--cat-yellow); }
</style>
