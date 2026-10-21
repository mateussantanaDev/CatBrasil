<script>
  import { onMount } from 'svelte';
  import { authState } from '../../state/auth.svelte.js';
  import { toastState } from '../../state/toast.svelte.js';

  let activeTab = $state('login'); // 'login' | 'register'
  let username = $state('');
  let password = $state('');
  let phone = $state('');
  let inviteCode = $state('');
  let fingerprint = $state('');

  onMount(() => {
    // Detecta código de convite na URL (?ref=CB-1234)
    const urlParams = new URLSearchParams(window.location.search);
    const ref = urlParams.get('ref');
    if (ref) {
      inviteCode = ref.toUpperCase();
      activeTab = 'register';
    }

    // Gera fingerprint simples de navegador
    fingerprint = `${navigator.userAgent}-${screen.width}x${screen.height}`;
  });

  let twoFactorCode = $state('');
  let show2FAField = $state(false);

  async function handleLogin(e) {
    e.preventDefault();
    if (!username.trim() || !password) {
      toastState.warning('Preencha seu usuário e senha');
      return;
    }
    const ok = await authState.login(username, password, twoFactorCode, fingerprint);
    if (!ok && !show2FAField) {
      show2FAField = true;
    }
  }

  async function handleRegister(e) {
    e.preventDefault();
    if (!username.trim() || !password || !phone.trim()) {
      toastState.warning('Preencha todos os campos obrigatórios');
      return;
    }
    await authState.register(username, password, phone, inviteCode, fingerprint);
  }
</script>

<div class="login-screen">
  <div class="login-modal-box">
    <!-- Faixa de Perigo Industrial -->
    <div class="hazard-stripes modal-hazard-bar"></div>

    <div class="login-header">
      <div class="logo-box">
        <span class="logo-icon">🚜</span>
        <h1 class="logo-title">CATBRASIL</h1>
      </div>
      <p class="logo-sub">TERMINAL INDUSTRIAL DE MINERAÇÃO</p>
    </div>

    <!-- Abas de Alternância -->
    <div class="auth-tabs">
      <button
        class="auth-tab-btn {activeTab === 'login' ? 'active' : ''}"
        onclick={() => activeTab = 'login'}
      >
        ENTRAR NO SISTEMA
      </button>
      <button
        class="auth-tab-btn {activeTab === 'register' ? 'active' : ''}"
        onclick={() => activeTab = 'register'}
      >
        CRIAR CONTA OPERADOR
      </button>
    </div>

    <div class="login-form-container">
      {#if activeTab === 'login'}
        <form onsubmit={handleLogin} class="auth-form">
          <div class="form-group">
            <label class="form-label" for="login-username">NOME DE OPERADOR (USUÁRIO)</label>
            <input
              id="login-username"
              type="text"
              class="input-cat"
              placeholder="ex: operador_cat"
              bind:value={username}
              autocomplete="username"
              required
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="login-password">SENHA DE ACESSO</label>
            <input
              id="login-password"
              type="password"
              class="input-cat"
              placeholder="••••••••"
              bind:value={password}
              autocomplete="current-password"
              required
            />
          </div>

          {#if show2FAField}
            <div class="form-group">
              <label class="form-label" for="login-2fa" style="color: #fbbf24;">CÓDIGO 2FA (GOOGLE AUTHENTICATOR)</label>
              <input
                id="login-2fa"
                type="text"
                maxlength="6"
                class="input-cat font-mono text-center text-yellow"
                placeholder="000000"
                bind:value={twoFactorCode}
              />
            </div>
          {/if}

          <button
            type="submit"
            class="btn-cat btn-auth-submit"
            disabled={authState.isLoading}
          >
            {#if authState.isLoading}
              <span>⚙️ AUTENTICANDO...</span>
            {:else}
              <span>CONECTAR À PLATAFORMA ⚡</span>
            {/if}
          </button>
        </form>
      {:else}
        <form onsubmit={handleRegister} class="auth-form">
          <div class="form-group">
            <label class="form-label" for="reg-username">NOME DE OPERADOR (MÍN. 3 LETRAS)</label>
            <input
              id="reg-username"
              type="text"
              class="input-cat"
              placeholder="ex: minerador_pro"
              bind:value={username}
              autocomplete="username"
              required
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="reg-phone">TELEFONE / WHATSAPP</label>
            <input
              id="reg-phone"
              type="tel"
              class="input-cat"
              placeholder="(11) 99999-9999"
              bind:value={phone}
              autocomplete="tel"
              required
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="reg-password">SENHA DE ACESSO</label>
            <input
              id="reg-password"
              type="password"
              class="input-cat"
              placeholder="Mínimo de 5 caracteres"
              bind:value={password}
              autocomplete="new-password"
              required
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="reg-invite">CÓDIGO DE CONVITE (OPCIONAL)</label>
            <input
              id="reg-invite"
              type="text"
              class="input-cat input-invite font-mono"
              placeholder="ex: CB-1001"
              bind:value={inviteCode}
            />
          </div>

          <div class="welcome-gift-notice">
            <span>🎁 <strong>Bônus de Boas-Vindas:</strong> Cadastro garante 1x Escavadeira CAT 320 com primeira coleta liberada!</span>
          </div>

          <button
            type="submit"
            class="btn-cat btn-auth-submit"
            disabled={authState.isLoading}
          >
            {#if authState.isLoading}
              <span>⚙️ REGISTRANDO...</span>
            {:else}
              <span>CADASTRAR E ATIVAR FROTA 🚜</span>
            {/if}
          </button>
        </form>
      {/if}
    </div>
  </div>
</div>

<style>
  .login-screen {
    min-height: 100vh;
    background: #0a0a0a;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;
  }

  .login-modal-box {
    width: 100%;
    max-width: 440px;
    background: #141414;
    border: 3px solid var(--cat-black);
    box-shadow: var(--modal-shadow);
    overflow: hidden;
  }

  .modal-hazard-bar {
    height: 8px;
    width: 100%;
  }

  .login-header {
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
    font-size: 1.8rem;
    color: var(--cat-yellow);
    letter-spacing: 2px;
  }

  .logo-sub {
    font-size: 0.65rem;
    color: #888;
    letter-spacing: 1px;
    font-weight: 700;
    margin-top: 2px;
  }

  .auth-tabs {
    display: flex;
    background: #0c0c0c;
    border-bottom: 2px solid #222;
  }

  .auth-tab-btn {
    flex: 1;
    padding: 12px 6px;
    font-family: var(--font-heading);
    font-size: 0.8rem;
    font-weight: 700;
    color: #666;
    background: transparent;
    border: none;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .auth-tab-btn.active {
    color: var(--cat-yellow);
    background: #181818;
    border-bottom: 3px solid var(--cat-yellow);
  }

  .login-form-container {
    padding: 20px;
  }

  .auth-form {
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
    font-weight: 700;
    color: #999;
    letter-spacing: 0.5px;
  }

  .input-invite {
    color: var(--cat-yellow);
  }

  .welcome-gift-notice {
    background: rgba(255, 178, 0, 0.08);
    border: 1px dashed var(--cat-yellow);
    padding: 8px 10px;
    font-size: 0.7rem;
    color: #eee;
  }

  .btn-auth-submit {
    width: 100%;
    padding: 14px;
    font-size: 1rem;
    margin-top: 6px;
  }
</style>
