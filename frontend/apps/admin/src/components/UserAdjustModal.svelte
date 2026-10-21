<script>
  import { adminDashboardState } from '../state/admin.svelte.js';
  import { toastState } from '../state/toast.svelte.js';
  import { Modal, Button } from '@catbrasil/ui';

  let {
    isOpen = false,
    user = null,
    onclose
  } = $props();

  let deltaAmount = $state(0);
  let actionType = $state('add'); // 'add' | 'sub'
  let reason = $state('');

  async function handleAdjust(e) {
    e.preventDefault();
    if (!deltaAmount || deltaAmount <= 0) {
      toastState.warning('Informe um valor de ajuste válido.');
      return;
    }

    const finalDelta = actionType === 'add' ? deltaAmount : -deltaAmount;
    const ok = await adminDashboardState.adjustUserBalance(user.id, finalDelta);

    if (ok && onclose) {
      deltaAmount = 0;
      reason = '';
      onclose();
    }
  }
</script>

<Modal {isOpen} title="AJUSTAR SALDO MANUALMENTE" {onclose} maxWidth="max-w-sm">
  <form onsubmit={handleAdjust} class="adjust-form font-mono">
    <div class="user-summary">
      <span class="user-lbl">OPERADOR SELECIONADO:</span>
      <strong class="user-val text-yellow">@{user?.username || 'N/A'} (ID: {user?.id || 'N/A'})</strong>
      <div class="current-bal">
        <span>SALDO ATUAL:</span>
        <strong class="text-green">R$ {(user?.balance || 0).toFixed(2)}</strong>
      </div>
    </div>

    <div class="action-type-toggle">
      <button
        type="button"
        class="btn-toggle {actionType === 'add' ? 'toggle-add' : ''}"
        onclick={() => actionType = 'add'}
      >
        + ADICIONAR CRÉDITO
      </button>
      <button
        type="button"
        class="btn-toggle {actionType === 'sub' ? 'toggle-sub' : ''}"
        onclick={() => actionType = 'sub'}
      >
        - DEBITAR SALDO
      </button>
    </div>

    <div class="form-group">
      <label for="adjust-amt" class="form-label">VALOR DO AJUSTE (R$)</label>
      <input
        id="adjust-amt"
        type="number"
        step="10"
        min="1"
        class="input-cat font-bold {actionType === 'add' ? 'text-green' : 'text-red'}"
        placeholder="ex: 100.00"
        bind:value={deltaAmount}
        required
      />
    </div>

    <div class="form-group">
      <label for="adjust-reason" class="form-label">MOTIVO / JUSTIFICATIVA DO AJUSTE</label>
      <input
        id="adjust-reason"
        type="text"
        class="input-cat"
        placeholder="ex: Bônus de campanha promocional"
        bind:value={reason}
      />
    </div>

    <button
      type="submit"
      class="btn-cat btn-submit-adjust"
      disabled={adminDashboardState.isLoading}
    >
      {#if adminDashboardState.isLoading}
        <span>⚙️ PROCESSANDO AJUSTE...</span>
      {:else}
        <span>CONFIRMAR AJUSTE DE SALDO 💾</span>
      {/if}
    </button>
  </form>
</Modal>

<style>
  .adjust-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
    font-size: 0.75rem;
  }

  .user-summary {
    background: #111;
    border: 1px solid #282828;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .user-lbl {
    font-size: 0.6rem;
    color: #888;
  }

  .user-val {
    font-size: 0.9rem;
  }

  .current-bal {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    margin-top: 4px;
    border-top: 1px dashed #282828;
    padding-top: 4px;
  }

  .action-type-toggle {
    display: flex;
    gap: 6px;
  }

  .btn-toggle {
    flex: 1;
    background: #202020;
    color: #888;
    border: 1px solid #333;
    padding: 8px;
    font-family: var(--font-heading);
    font-size: 0.75rem;
    font-weight: 700;
    cursor: pointer;
  }

  .toggle-add {
    background: rgba(0, 200, 83, 0.2);
    color: var(--cat-green);
    border-color: var(--cat-green);
  }

  .toggle-sub {
    background: rgba(213, 0, 0, 0.2);
    color: var(--cat-red);
    border-color: var(--cat-red);
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

  .btn-submit-adjust {
    width: 100%;
    padding: 12px;
    margin-top: 4px;
  }

  .text-yellow { color: var(--cat-yellow); }
  .text-green { color: var(--cat-green); }
  .text-red { color: var(--cat-red); }
</style>
