<script>
  import { authState } from '../../state/auth.svelte.js';
  import { toastState } from '../../state/toast.svelte.js';
  import { Modal, Button } from '@catbrasil/ui';

  let {
    isOpen = false,
    onclose
  } = $props();

  const user = $derived(authState.user);
  const hasExistingPin = $derived(Boolean(user?.hasSecurityPassword));
  const has2FA = $derived(Boolean(user?.twoFactorEnabled));

  let currentPin = $state('');
  let accountPassword = $state('');
  let pin = $state('');
  let confirmPin = $state('');
  let twoFactorCode = $state('');
  let isLoading = $state(false);

  async function handleSavePin(e) {
    e.preventDefault();
    if (hasExistingPin && (!currentPin || currentPin.length !== 6)) {
      toastState.warning('Informe sua senha financeira atual de 6 dígitos.');
      return;
    }

    if (!accountPassword) {
      toastState.warning('Informe a senha da sua conta.');
      return;
    }

    if (has2FA && (!twoFactorCode || twoFactorCode.length !== 6)) {
      toastState.warning('Informe o código 2FA do aplicativo autenticador.');
      return;
    }

    if (pin.length !== 6 || confirmPin.length !== 6) {
      toastState.warning('O novo PIN financeiro deve ter exatamente 6 dígitos numéricos.');
      return;
    }

    if (pin !== confirmPin) {
      toastState.error('A confirmação do PIN não confere.');
      return;
    }

    isLoading = true;
    const ok = await authState.saveSecurityPassword(pin, currentPin, accountPassword, twoFactorCode);
    isLoading = false;

    if (ok && onclose) {
      currentPin = '';
      accountPassword = '';
      pin = '';
      confirmPin = '';
      twoFactorCode = '';
      onclose();
    }
  }
</script>

<Modal {isOpen} title={hasExistingPin ? "ALTERAR SENHA FINANCEIRA (PIN)" : "CRIAR SENHA FINANCEIRA (PIN 6 DÍGITOS)"} {onclose} maxWidth="max-w-sm">
  <form onsubmit={handleSavePin} class="pin-form">
    <p class="pin-desc">
      Esta senha será solicitada a cada pedido de saque para garantir a segurança dos seus fundos.
    </p>

    {#if hasExistingPin}
      <div class="form-group">
        <label for="current-security-pin" class="form-label">PIN ATUAL (6 DÍGITOS):</label>
        <input
          id="current-security-pin"
          type="password"
          maxlength="6"
          class="input-cat font-mono text-center text-xl"
          placeholder="••••••"
          bind:value={currentPin}
          required
        />
      </div>
    {/if}

    <div class="form-group">
      <label for="account-password" class="form-label">SENHA DA CONTA:</label>
      <input id="account-password" type="password" autocomplete="current-password" class="input-cat" bind:value={accountPassword} required />
    </div>

    {#if has2FA}
      <div class="form-group">
        <label for="two-factor-code" class="form-label">CÓDIGO 2FA (AUTENTICADOR):</label>
        <input
          id="two-factor-code"
          type="text"
          maxlength="6"
          class="input-cat font-mono text-center text-xl text-yellow"
          placeholder="000000"
          bind:value={twoFactorCode}
          required
        />
      </div>
    {/if}

    <div class="form-group">
      <label for="new-security-pin" class="form-label">{hasExistingPin ? "NOVO PIN (6 DÍGITOS NUMÉRICOS):" : "DIGITE O PIN (6 DÍGITOS NUMÉRICOS):"}</label>
      <input
        id="new-security-pin"
        type="password"
        maxlength="6"
        class="input-cat font-mono text-center text-xl"
        placeholder="••••••"
        bind:value={pin}
        required
      />
    </div>

    <div class="form-group">
      <label for="confirm-security-pin" class="form-label">CONFIRME O NOVO PIN:</label>
      <input
        id="confirm-security-pin"
        type="password"
        maxlength="6"
        class="input-cat font-mono text-center text-xl"
        placeholder="••••••"
        bind:value={confirmPin}
        required
      />
    </div>

    <button
      type="submit"
      class="btn-cat btn-pin-submit"
      disabled={isLoading}
    >
      {#if isLoading}
        <span>⚙️ SALVANDO PIN...</span>
      {:else}
        <span>{hasExistingPin ? "ATUALIZAR SENHA FINANCEIRA 🔒" : "GRAVAR SENHA FINANCEIRA 🔒"}</span>
      {/if}
    </button>
  </form>
</Modal>

<style>
  .pin-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .pin-desc {
    font-size: 0.75rem;
    color: #999;
    line-height: 1.4;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .form-label {
    font-size: 0.65rem;
    font-weight: 700;
    color: #888;
  }

  .btn-pin-submit {
    width: 100%;
    padding: 12px;
    margin-top: 4px;
  }
</style>
