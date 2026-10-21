<script>
  import * as QRCode from 'qrcode';

  let { isOpen = false, onclose = () => {}, setup, verify, onEnabled = () => {} } = $props();
  let secret = $state('');
  let qr = $state('');
  let code = $state('');
  let error = $state('');
  let loading = $state(false);
  let requested = $state(false);

  $effect(() => {
    if (isOpen && !requested) {
      requested = true;
      void loadSetup();
    }
  });

  async function loadSetup() {
    loading = true;
    error = '';
    try {
      const result = await setup();
      secret = result.secret;
      qr = result.otpauthUrl ? await QRCode.toDataURL(result.otpauthUrl, { width: 200, margin: 1 }) : '';
    } catch (cause) {
      error = cause.message || 'Não foi possível iniciar o 2FA.';
    } finally {
      loading = false;
    }
  }

  async function confirm() {
    if (!/^\d{6}$/.test(code.trim())) {
      error = 'Informe o código de 6 dígitos do autenticador.';
      return;
    }
    loading = true;
    error = '';
    try {
      await verify(code.trim());
      onEnabled();
      close();
    } catch (cause) {
      error = cause.message || 'Código 2FA inválido.';
    } finally {
      loading = false;
    }
  }

  function close() {
    secret = '';
    qr = '';
    code = '';
    error = '';
    requested = false;
    onclose();
  }
</script>

{#if isOpen}
  <div class="overlay" role="presentation">
    <div class="dialog" role="dialog" aria-modal="true" aria-label="Ativar autenticação de dois fatores">
      <h2>ATIVAR 2FA DO ERP</h2>
      <p>Escaneie o QR no aplicativo autenticador ou cadastre a chave manualmente.</p>
      {#if qr}<img src={qr} alt="QR para cadastrar autenticador" width="200" height="200" />{/if}
      {#if secret}<p class="secret">Chave manual: <strong>{secret}</strong></p>{/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      {#if error && !secret}<button type="button" onclick={() => requested = false}>Tentar novamente</button>{/if}
      <label for="erp-totp-code">Código do autenticador</label>
      <input id="erp-totp-code" type="text" inputmode="numeric" autocomplete="one-time-code" maxlength="6" bind:value={code} placeholder="000000" />
      <div class="actions">
        <button type="button" onclick={close}>Cancelar</button>
        <button type="button" onclick={confirm} disabled={loading || !secret}>Ativar 2FA</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; z-index: 1000002; display: grid; place-items: center; background: #020617cc; padding: 16px; }
  .dialog { width: min(100%, 420px); padding: 22px; background: #fff; color: #0f172a; border: 3px solid #ffb200; border-radius: 8px; box-shadow: 0 12px 35px #0008; font-family: system-ui, sans-serif; }
  h2 { margin: 0 0 10px; font-size: 1.15rem; }
  p { font-size: .85rem; }
  img { display: block; margin: 12px auto; }
  .secret { overflow-wrap: anywhere; font-family: monospace; }
  .error { color: #b91c1c; font-weight: 700; }
  label { display: block; font-size: .8rem; font-weight: 700; margin: 12px 0 4px; }
  input { width: 100%; box-sizing: border-box; padding: 10px; font-size: 1.2rem; letter-spacing: 6px; text-align: center; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px; }
  button { padding: 10px 14px; border: 1px solid #0f172a; cursor: pointer; font-weight: 800; }
  button:last-child { background: #ffb200; }
</style>
