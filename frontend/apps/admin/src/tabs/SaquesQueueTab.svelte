<script>
  import { onMount } from 'svelte';
  import { adminDashboardState } from '../state/admin.svelte.js';
  import { Button, Card, Badge } from '@catbrasil/ui';

  const withdrawals = $derived(adminDashboardState.withdrawals);

  onMount(() => {
    adminDashboardState.loadWithdrawals();
  });

  async function handleApprove(txId) {
    if (confirm(`Confirmar aprovação e liberação imediata do saque #${txId}?`)) {
      await adminDashboardState.approveWithdrawal(txId);
    }
  }

  async function handleReject(txId) {
    const reason = prompt('Informe o motivo da rejeição (o saldo será estornado automaticamente ao operador):', 'Dados PIX divergentes');
    if (reason !== null) {
      await adminDashboardState.rejectWithdrawal(txId, reason, false);
    }
  }

  async function handleRejectAndLock(txId) {
    const reason = prompt('ATENÇÃO: Rejeitar saque, estornar saldo e BLOQUEAR saques futuros deste operador por suspeita de fraude?', 'Atividade multi-conta suspeita');
    if (reason !== null) {
      await adminDashboardState.rejectWithdrawal(txId, reason, true);
    }
  }
</script>

<div class="saques-tab">
  <div class="tab-top-bar">
    <div>
      <span class="tab-subtitle font-mono">MODERAÇÃO FINANCEIRA & PAYOUTS</span>
      <h2 class="tab-title">FILA DE SAQUES PIX PENDENTES ({withdrawals.length}) 💰</h2>
    </div>
    <button class="btn-refresh font-mono" onclick={() => adminDashboardState.loadWithdrawals()}>
      🔄 ATUALIZAR FILA
    </button>
  </div>

  {#if adminDashboardState.isLoading && withdrawals.length === 0}
    <div class="loading-box">
      <span class="animate-spin text-2xl">⚙️</span>
      <p>Carregando fila de solicitações de saque...</p>
    </div>
  {:else if withdrawals.length === 0}
    <div class="empty-box">
      <span class="empty-icon">✅</span>
      <h4>Fila Limpa!</h4>
      <p>Nenhuma solicitação de saque pendente no momento.</p>
    </div>
  {:else}
    <div class="withdrawals-table-wrapper font-mono">
      <table class="withdrawals-table">
        <thead>
          <tr>
            <th>TRANSAÇÃO</th>
            <th>OPERADOR</th>
            <th>CHAVE PIX</th>
            <th>SOLICITADO</th>
            <th>TAXA</th>
            <th>LÍQUIDO A PAGAR</th>
            <th>DATA</th>
            <th>AÇÕES DE MODERAÇÃO</th>
          </tr>
        </thead>
        <tbody>
          {#each withdrawals as tx (tx.id)}
            <tr>
              <td>
                <span class="tx-id-badge">#{tx.id}</span>
              </td>
              <td>
                <strong class="user-name">@{tx.userId}</strong>
              </td>
              <td>
                <span class="pix-key-val">{tx.pixKey || 'N/A'}</span>
              </td>
              <td>
                <strong>R$ {Math.abs(tx.valor || 0).toFixed(2)}</strong>
              </td>
              <td>
                <span class="text-red">R$ {(tx.feeAmount || 0).toFixed(2)}</span>
              </td>
              <td>
                <strong class="text-green text-base">R$ {(tx.netAmount || Math.abs(tx.valor || 0)).toFixed(2)}</strong>
              </td>
              <td>
                <span class="tx-date">{new Date(tx.createdAt).toLocaleString('pt-BR')}</span>
              </td>
              <td>
                <div class="action-buttons">
                  <button
                    class="btn-mod btn-approve"
                    onclick={() => handleApprove(tx.id)}
                    title="Aprovar e Liberar Saque"
                  >
                    ⚡ APROVAR
                  </button>
                  <button
                    class="btn-mod btn-reject"
                    onclick={() => handleReject(tx.id)}
                    title="Rejeitar e Estornar Saldo"
                  >
                    🔄 ESTORNAR
                  </button>
                  <button
                    class="btn-mod btn-lock"
                    onclick={() => handleRejectAndLock(tx.id)}
                    title="Rejeitar e Bloquear Conta"
                  >
                    🚫 FRAUDE
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .saques-tab {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .tab-top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: #141414;
    border: 2px solid var(--cat-black);
    box-shadow: 3px 3px 0px #000;
    padding: 14px 18px;
  }

  .tab-subtitle {
    font-size: 0.65rem;
    color: #888;
  }

  .tab-title {
    color: var(--cat-yellow);
    font-size: 1.25rem;
    margin-top: 2px;
  }

  .btn-refresh {
    background: #222;
    color: var(--cat-yellow);
    border: 1.5px solid var(--cat-yellow);
    padding: 8px 14px;
    font-size: 0.75rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-refresh:hover {
    background: var(--cat-yellow);
    color: #000;
  }

  .withdrawals-table-wrapper {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    overflow-x: auto;
  }

  .withdrawals-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.75rem;
  }

  .withdrawals-table th {
    background: #111;
    color: #888;
    padding: 12px 14px;
    border-bottom: 2px solid #282828;
    font-size: 0.65rem;
    font-weight: 700;
  }

  .withdrawals-table td {
    padding: 12px 14px;
    border-bottom: 1px solid #222;
    color: #ddd;
  }

  .tx-id-badge {
    background: #222;
    color: #aaa;
    padding: 2px 6px;
    font-size: 0.65rem;
  }

  .user-name {
    color: var(--cat-yellow);
  }

  .pix-key-val {
    color: #fff;
  }

  .tx-date {
    font-size: 0.65rem;
    color: #777;
  }

  .action-buttons {
    display: flex;
    gap: 6px;
  }

  .btn-mod {
    padding: 6px 10px;
    font-family: var(--font-heading);
    font-size: 0.7rem;
    font-weight: 700;
    border: 1px solid #000;
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-approve {
    background: var(--cat-green);
    color: #000;
  }

  .btn-reject {
    background: #424242;
    color: #fff;
  }

  .btn-lock {
    background: var(--cat-red);
    color: #fff;
  }

  .text-green { color: var(--cat-green); }
  .text-red { color: var(--cat-red); }

  .loading-box, .empty-box {
    background: #161616;
    border: 2px dashed #333;
    padding: 40px;
    text-align: center;
    color: #888;
  }

  .empty-icon {
    font-size: 2.5rem;
    margin-bottom: 8px;
    display: block;
  }
</style>
