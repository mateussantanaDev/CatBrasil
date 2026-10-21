<script>
  import { adminAuthState } from '../state/auth.svelte.js';
  import { toastState } from '../state/toast.svelte.js';

  let username = $state('');
  let password = $state('');
  let pin = $state('');
  let twoFactorCode = $state('');

  async function handleLogin(e) {
    e.preventDefault();
    if (!username.trim() || !password || !pin.trim() || !/^\d{6}$/.test(twoFactorCode.trim())) {
      toastState.warning('Informe usuário, senha, PIN e código 2FA de 6 dígitos.');
      return;
    }
    await adminAuthState.login(username, password, pin, twoFactorCode);
  }
</script>

<div class="erp-login-screen">
  <div class="erp-login-box">
    <div class="hazard-stripes modal-hazard-bar"></div>

    <div class="erp-login-header">
      <div class="logo-box">
        <span class="logo-icon">🚜</span>
        <h1 class="logo-title">CATBRASIL ERP</h1>
      </div>
      <p class="logo-sub font-mono">PAINEL DE GESTÃO & CONTROLE DE LIQUIDEZ</p>
    </div>

    <form onsubmit={handleLogin} class="erp-form font-mono">
      <div class="form-group">
        <label for="admin-user" class="form-label">USUÁRIO ADMINISTRADOR</label>
        <input
          id="admin-user"
          type="text"
          class="input-cat"
          placeholder="Usuário administrativo"
          bind:value={username}
          autocomplete="username"
          required
        />
      </div>

      <div class="form-group">
        <label for="admin-pass" class="form-label">SENHA DE ACESSO</label>
        <input
          id="admin-pass"
          type="password"
          class="input-cat"
          placeholder="••••••••••••"
          bind:value={password}
          autocomplete="current-password"
          required
        />
      </div>

      <div class="form-group">
        <label for="admin-pin" class="form-label">PIN ADMINISTRATIVO</label>
        <input
          id="admin-pin"
          type="password"
          maxlength="64"
          class="input-cat text-yellow"
          placeholder="••••••"
          bind:value={pin}
        />
      </div>

        <div class="form-group">
          <label for="admin-2fa" class="form-label">CÓDIGO 2FA / AUTHENTICATOR (6 DÍGITOS)</label>
          <input
            id="admin-2fa"
            type="text"
            maxlength="6"
            class="input-cat text-yellow font-mono text-center tracking-widest text-lg"
            placeholder="000000"
            bind:value={twoFactorCode}
            autocomplete="one-time-code"
          />
        </div>

      <div class="security-warning">
        <span>🔒 Acesso restrito a diretores e auditores autorizados da CatBrasil. Todas as ações e IPs são registrados no security_logs.</span>
      </div>

      <button
        type="submit"
        class="btn-cat btn-auth-erp"
        disabled={adminAuthState.isLoading}
      >
        {#if adminAuthState.isLoading}
          <span>⚙️ AUTENTICANDO ERP...</span>
        {:else}
          <span>ACESSAR TERMINAL ADMINISTRATIVO ⚡</span>
        {/if}
      </button>
    </form>
  </div>
</div>

<style>
  .erp-login-screen {
    min-height: 100vh;
    background: #080808;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;
  }

  .erp-login-box {
    width: 100%;
    max-width: 440px;
    background: #141414;
    border: 3px solid var(--cat-black);
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.9);
    overflow: hidden;
  }

  .modal-hazard-bar {
    height: 8px;
    width: 100%;
  }

  .erp-login-header {
    padding: 24px 20px 14px;
    text-align: center;
    border-bottom: 2px solid #222;
  }

  .logo-box {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .logo-icon {
    font-size: 2rem;
  }

  .logo-title {
    font-size: 1.7rem;
    color: var(--cat-yellow);
    letter-spacing: 2px;
  }

  .logo-sub {
    font-size: 0.65rem;
    color: #888;
    letter-spacing: 1px;
    margin-top: 4px;
  }

  .erp-form {
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .form-label {
    font-size: 0.65rem;
    color: #888;
    font-weight: 700;
  }

  .security-warning {
    background: #0d0d0d;
    border: 1px dashed #333;
    padding: 8px 10px;
    font-size: 0.65rem;
    color: #777;
    line-height: 1.4;
  }

  .btn-auth-erp {
    width: 100%;
    padding: 14px;
    font-size: 0.95rem;
    margin-top: 4px;
  }

  .text-yellow { color: var(--cat-yellow); }
</style>
