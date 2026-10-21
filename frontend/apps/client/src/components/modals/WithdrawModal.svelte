<script>
  import { api } from '../../services/api.js';
  import { authState } from '../../state/auth.svelte.js';
  import { toastState } from '../../state/toast.svelte.js';
  import { Modal, Button } from '@catbrasil/ui';

  let {
    isOpen = false,
    onclose,
    onopensecuritypin
  } = $props();

  const user = $derived(authState.user);

  let amount = $state('');
  let pixKey = $state('');
  let password = $state('');
  let twoFactorCode = $state('');
  let isLoading = $state(false);

  const parsedAmount = $derived(() => {
    const a = parseFloat(amount);
    return isNaN(a) ? 0 : a;
  });

  const feeAmount = $derived(() => {
    if (user?.freeWithdrawActive) return 0;
    return parsedAmount() * 0.05;
  });

  const netAmount = $derived(() => {
    return Math.max(0, parsedAmount() - feeAmount());
  });

  const rolloverPercent = $derived(() => {
    if (!user || user.requiredRollover <= 0) return 100;
    return Math.min(100, Math.round((user.currentRollover / user.requiredRollover) * 100));
  });

  const hasMetRollover = $derived(() => {
    if (!user) return false;
    return user.currentRollover >= user.requiredRollover;
  });

  const needs2FA = $derived(() => {
    return parsedAmount() > 500 && user?.twoFactorEnabled;
  });

  async function handleWithdraw(e) {
    e.preventDefault();
    if (!parsedAmount() || parsedAmount() < 50) {
      toastState.warning('O valor mínimo para saque é de R$ 50,00.');
      return;
    }

    if (parsedAmount() > (user?.balance || 0)) {
      toastState.error('Saldo insuficiente em carteira.');
      return;
    }

    if (!hasMetRollover) {
      const missing = user.requiredRollover - user.currentRollover;
      toastState.error(`Rollover pendente! Faltam R$ ${missing.toFixed(2)} em aluguéis.`);
      return;
    }

    if (!password || password.length !== 6) {
      toastState.warning('Digite sua senha financeira de 6 dígitos.');
      return;
    }

    try {
      isLoading = true;
      const res = await api.requestWithdrawal(
        authState.token,
        parsedAmount(),
        pixKey.trim(),
        password.trim(),
        twoFactorCode.trim()
      );

      toastState.success(res.message || 'Saque PIX solicitado com sucesso!');

      if (res.newBalance !== undefined) {
        authState.updateUser({ balance: res.newBalance });
      }

      if (onclose) onclose();
    } catch (err) {
      toastState.error(err.message || 'Erro ao processar solicitação de saque.');
    } finally {
      isLoading = false;
    }
  }
</script>

<Modal {isOpen} title="SOLICITAR SAQUE PIX" {onclose} maxWidth="max-w-md">
  <form onsubmit={handleWithdraw} class="withdraw-form">
    <!-- Barra de Progresso do Rollover -->
    <div class="rollover-box">
      <div class="rollover-header">
        <span class="rollover-lbl">STATUS DO ROLLOVER DE ALUGUÉIS:</span>
        <strong class="font-mono {hasMetRollover ? 'text-green' : 'text-yellow'}">
          {rolloverPercent()}% ({hasMetRollover ? 'LIBERADO ✅' : 'PENDENTE 🔒'})
        </strong>
      </div>
      <div class="rollover-bar-bg">
        <div class="rollover-bar-fill {hasMetRollover ? 'bar-green' : ''}" style="width: {rolloverPercent()}%;"></div>
      </div>
      <div class="rollover-footer font-mono">
        <span>GIRO: R$ {(user?.currentRollover || 0).toFixed(2)}</span>
        <span>EXIGIDO: R$ {(user?.requiredRollover || 0).toFixed(2)}</span>
      </div>
    </div>

    <!-- Saldo Disponível -->
    <div class="balance-badge font-mono">
      <span>SALDO DISPONÍVEL:</span>
      <strong class="text-yellow">R$ {(user?.balance || 0).toFixed(2)}</strong>
    </div>

    <!-- Campos do Formulário -->
    <div class="form-group">
      <label for="withdraw-amount" class="form-label">VALOR DO SAQUE (MÍNIMO R$ 50,00)</label>
      <input
        id="withdraw-amount"
        type="number"
        step="10"
        min="50"
        max={user?.balance || 0}
        class="input-cat font-mono"
        placeholder="ex: 150.00"
        bind:value={amount}
        required
      />
    </div>

    <div class="form-group">
      <label for="withdraw-pix-key" class="form-label">CHAVE PIX DE DESTINO (CPF / TELEFONE / EMAIL / CHAVE)</label>
      <input
        id="withdraw-pix-key"
        type="text"
        class="input-cat font-mono"
        placeholder="Digite sua chave PIX"
        bind:value={pixKey}
        required
      />
    </div>

    <div class="form-group">
      <label for="withdraw-pin" class="form-label">SENHA FINANCEIRA (6 DÍGITOS)</label>
      <input
        id="withdraw-pin"
        type="password"
        maxlength="6"
        class="input-cat font-mono"
        placeholder="••••••"
        bind:value={password}
        required
      />
    </div>

    {#if needs2FA}
      <div class="form-group">
        <label for="withdraw-2fa" class="form-label">CÓDIGO 2FA GOOGLE AUTHENTICATOR (VALOR > R$ 500)</label>
        <input
          id="withdraw-2fa"
          type="text"
          maxlength="6"
          class="input-cat font-mono text-yellow"
          placeholder="000000"
          bind:value={twoFactorCode}
          required
        />
      </div>
    {/if}

    <!-- Resumo de Taxas -->
    <div class="summary-box font-mono">
      <div class="summary-row">
        <span>VALOR SOLICITADO:</span>
        <span>R$ {parsedAmount().toFixed(2)}</span>
      </div>
      <div class="summary-row">
        <span>TAXA DE PROCESSAMENTO (5%):</span>
        <span class="{user?.freeWithdrawActive ? 'text-green' : 'text-red'}">
          {user?.freeWithdrawActive ? 'ISENTO (SAQUE GRÁTIS)' : `- R$ ${feeAmount().toFixed(2)}`}
        </span>
      </div>
      <div class="summary-row summary-total">
        <span>LÍQUIDO A RECEBER:</span>
        <strong class="text-green text-base">R$ {netAmount().toFixed(2)}</strong>
      </div>
    </div>

    <button
      type="submit"
      class="btn-cat btn-withdraw-submit"
      disabled={isLoading || !hasMetRollover}
    >
      {#if isLoading}
        <span>⚙️ PROCESSANDO SAQUE...</span>
      {:else if !hasMetRollover}
        <span>🔒 BLOQUEADO: CUMPRA O ROLLOVER EM FROTAS</span>
      {:else}
        <span>CONFIRMAR SOLICITAÇÃO DE SAQUE 💰</span>
      {/if}
    </button>
  </form>
</Modal>

<style>
  .withdraw-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .rollover-box {
    background: #111;
    border: 1.5px solid #282828;
    padding: 10px 12px;
  }

  .rollover-header {
    display: flex;
    justify-content: space-between;
    font-size: 0.65rem;
  }

  .rollover-lbl {
    color: #888;
  }

  .rollover-bar-bg {
    background: #222;
    height: 8px;
    margin: 6px 0;
    overflow: hidden;
  }

  .rollover-bar-fill {
    background: var(--cat-yellow);
    height: 100%;
    transition: width 0.3s;
  }

  .bar-green {
    background: var(--cat-green);
  }

  .rollover-footer {
    display: flex;
    justify-content: space-between;
    font-size: 0.6rem;
    color: #777;
  }

  .balance-badge {
    background: #161616;
    border: 1px solid #333;
    padding: 8px 12px;
    display: flex;
    justify-content: space-between;
    font-size: 0.8rem;
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
  }

  .summary-box {
    background: #101010;
    border: 1px solid #242424;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 0.75rem;
  }

  .summary-row {
    display: flex;
    justify-content: space-between;
    color: #aaa;
  }

  .summary-total {
    border-top: 1px solid #282828;
    padding-top: 6px;
    margin-top: 2px;
    color: #fff;
    font-weight: 700;
  }

  .btn-withdraw-submit {
    width: 100%;
    padding: 14px;
  }

  .text-yellow { color: var(--cat-yellow); }
  .text-green { color: var(--cat-green); }
  .text-red { color: var(--cat-red); }
</style>
