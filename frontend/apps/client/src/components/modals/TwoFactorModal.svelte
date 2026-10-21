<script>
  import { api } from '../../services/api.js';
  import { toastState } from '../../state/toast.svelte.js';
  import { Modal } from '@catbrasil/ui';
  import * as QRCode from 'qrcode';

  let {
    isOpen = false,
    onclose,
    userOverride = null,
    tokenOverride = '',
    onEnabled = () => {}
  } = $props();

  const user = $derived(userOverride);
  const currentToken = $derived(tokenOverride);

  let qrCodeUrl = $state('');
  let secret = $state('');
  let code = $state('');
  let isLoading = $state(false);

  $effect(() => {
    if (isOpen && !user?.twoFactorEnabled) {
      loadSetup();
    }
  });

  async function loadSetup() {
    try {
      isLoading = true;
      const res = await api.setup2FA(currentToken);
      secret = res.secret;
      qrCodeUrl = res.otpauthUrl ? await QRCode.toDataURL(res.otpauthUrl, { width: 200, margin: 1 }) : '';
    } catch (err) {
      console.warn('[2FA] Erro ao carregar setup 2FA:', err);
      toastState.error(err.message || 'Erro ao gerar segredo 2FA.');
    } finally {
      isLoading = false;
    }
  }

  async function handleVerify2FA(e) {
    e.preventDefault();
    if (!code || code.length !== 6) {
      toastState.warning('Digite o código de 6 dígitos gerado no app autenticador.');
      return;
    }

    try {
      isLoading = true;
      const res = await api.verify2FA(currentToken, code.trim());
      toastState.success(res.message || 'Autenticação 2FA ativada com sucesso!');

      onEnabled();

      if (onclose) onclose();
    } catch (err) {
      toastState.error(err.message || 'Código 2FA incorreto ou expirado.');
    } finally {
      isLoading = false;
    }
  }
</script>

<Modal {isOpen} title="AUTENTICAÇÃO 2FA (GOOGLE AUTHENTICATOR)" {onclose} maxWidth="max-w-md">
  <div class="two-factor-body">
    {#if user?.twoFactorEnabled}
      <div class="enabled-box">
        <span class="enabled-icon">🛡️</span>
        <h4 class="enabled-title">AUTENTICAÇÃO 2FA ATIVA</h4>
        <p class="enabled-desc">Sua conta está protegida pelo Google Authenticator para saques e operações de alto valor.</p>
        <button class="btn-cat btn-close-2fa mt-4" onclick={onclose}>FECHAR</button>
      </div>
    {:else}
      <form onsubmit={handleVerify2FA} class="setup-form">
        <p class="setup-desc">
          1. Escaneie o QR Code abaixo no seu aplicativo autenticador (Google Authenticator ou Authy):
        </p>

        {#if qrCodeUrl}
          <div class="qr-box">
            <img src={qrCodeUrl} alt="QR Code 2FA" class="qr-img" />
          </div>
        {/if}

        {#if secret}
          <div class="secret-box font-mono">
            <span class="secret-lbl">SEGREDO MANUAL:</span>
            <strong class="secret-val text-yellow">{secret}</strong>
          </div>
        {/if}

        <p class="setup-desc">
          2. Digite o código de 6 dígitos gerado no aplicativo para confirmar a ativação:
        </p>

        <div class="form-group">
          <input
            type="text"
            maxlength="6"
            class="input-cat font-mono text-center text-xl text-yellow"
            placeholder="000000"
            bind:value={code}
            required
          />
        </div>

        <button
          type="submit"
          class="btn-cat btn-enable-2fa"
          disabled={isLoading}
        >
          {#if isLoading}
            <span>⚙️ VERIFICANDO CÓDIGO...</span>
          {:else}
            <span>ATIVAR PROTEÇÃO 2FA 🛡️</span>
          {/if}
        </button>
      </form>
    {/if}
  </div>
</Modal>

<style>
  .two-factor-body {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .setup-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .setup-desc {
    font-size: 0.75rem;
    color: #bbb;
    line-height: 1.4;
  }

  .qr-box {
    display: flex;
    justify-content: center;
    background: #fff;
    padding: 10px;
    border: 2px solid #000;
    max-width: 180px;
    margin: 0 auto;
  }

  .qr-img {
    width: 100%;
    height: auto;
  }

  .secret-box {
    background: #111;
    border: 1px solid #282828;
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }

  .secret-lbl {
    font-size: 0.6rem;
    color: #777;
  }

  .secret-val {
    font-size: 0.85rem;
    letter-spacing: 1px;
  }

  .btn-enable-2fa {
    width: 100%;
    padding: 12px;
  }

  .enabled-box {
    text-align: center;
    padding: 20px 10px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  .enabled-icon {
    font-size: 2.5rem;
  }

  .enabled-title {
    color: var(--cat-green);
    font-size: 1.1rem;
  }

  .enabled-desc {
    font-size: 0.75rem;
    color: #aaa;
  }

  .btn-close-2fa {
    width: 100%;
    background: #252525;
    color: #fff;
  }

  .text-yellow { color: var(--cat-yellow); }
</style>
