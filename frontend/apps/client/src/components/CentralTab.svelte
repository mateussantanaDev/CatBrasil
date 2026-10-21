<script>
  import { onMount } from 'svelte';
  import { api } from '../services/api.js';
  import { authState } from '../state/auth.svelte.js';
  import { Button, Card, Badge } from '@catbrasil/ui';

  let {
    onopensecuritypin,
    onopen2fa
  } = $props();

  const user = $derived(authState.user);

  let transactions = $state([]);
  let filter = $state('TODOS');
  let isLoadingTx = $state(true);

  onMount(async () => {
    loadTransactions();
  });

  async function loadTransactions() {
    try {
      isLoadingTx = true;
      const res = await api.get('/user/transactions');
      transactions = res.transactions || res || [];
    } catch (err) {
      console.warn('[Central] Erro ao carregar transações:', err);
    } finally {
      isLoadingTx = false;
    }
  }

  const filteredTx = $derived(() => {
    if (filter === 'TODOS') return transactions;
    return transactions.filter((t) => t.tipo?.toUpperCase().includes(filter));
  });
</script>

<div class="central-tab">
  <!-- Perfil do Operador -->
  <div class="card-profile">
    <div class="profile-header">
      <div class="avatar-box">🚜</div>
      <div class="profile-meta">
        <h3 class="profile-username">@{user?.username || 'Operador'}</h3>
        <span class="profile-id font-mono">ID: {user?.id || 'CB-0000'} | TELEFONE: {user?.phone || 'N/A'}</span>
      </div>
      <button class="btn-logout" onclick={() => authState.logout()}>SAIR</button>
    </div>

    <!-- Segurança e Credenciais -->
    <div class="security-box">
      <h4 class="security-title">SEGURANÇA DA CONTA</h4>

      <div class="security-items">
        <div class="sec-item">
          <div class="sec-info">
            <span class="sec-name">SENHA FINANCEIRA (6 DÍGITOS)</span>
            <span class="sec-desc">Obrigatória para saques e alterações cadastrais.</span>
          </div>
          <button class="btn-sec" onclick={onopensecuritypin}>
            {user?.hasSecurityPassword ? 'ALTERAR PIN' : 'CRIAR PIN'}
          </button>
        </div>

        <div class="sec-item">
          <div class="sec-info">
            <span class="sec-name">AUTENTICAÇÃO 2FA (GOOGLE AUTHENTICATOR)</span>
            <span class="sec-desc">Proteção reforçada para saques acima de R$ 500.</span>
          </div>
          <button class="btn-sec {user?.twoFactorEnabled ? 'btn-sec-active' : ''}" onclick={onopen2fa}>
            {user?.twoFactorEnabled ? '✅ 2FA ATIVO' : 'CONFIGURAR 2FA'}
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- Histórico de Transações -->
  <div class="tx-section">
    <div class="tx-header">
      <h3 class="tx-title">EXTRATO DE MOVIMENTAÇÕES</h3>
      <div class="tx-filters">
        <button class="btn-tx-filter {filter === 'TODOS' ? 'active' : ''}" onclick={() => filter = 'TODOS'}>TODOS</button>
        <button class="btn-tx-filter {filter === 'DEPÓSITO' ? 'active' : ''}" onclick={() => filter = 'DEPÓSITO'}>DEPÓSITOS</button>
        <button class="btn-tx-filter {filter === 'SAQUE' ? 'active' : ''}" onclick={() => filter = 'SAQUE'}>SAQUES</button>
        <button class="btn-tx-filter {filter === 'RENDIMENTO' ? 'active' : ''}" onclick={() => filter = 'RENDIMENTO'}>RENDIMENTOS</button>
      </div>
    </div>

    {#if isLoadingTx}
      <div class="loading-box">
        <span class="animate-spin text-2xl">⚙️</span>
        <p>Carregando movimentações...</p>
      </div>
    {:else if filteredTx().length === 0}
      <div class="empty-box">
        <p>Nenhuma transação encontrada no período.</p>
      </div>
    {:else}
      <div class="tx-list">
        {#each filteredTx() as tx}
          <div class="tx-item">
            <div class="tx-left">
              <span class="tx-type font-bold">{tx.tipo}</span>
              <span class="tx-date font-mono">{new Date(tx.createdAt).toLocaleString('pt-BR')}</span>
            </div>

            <div class="tx-right">
              <span class="tx-amount font-mono {tx.valor >= 0 ? 'text-green' : 'text-red'}">
                {tx.valor >= 0 ? '+' : ''} R$ {Math.abs(tx.valor).toFixed(2)}
              </span>
              <span class="tx-status status-{tx.status} font-mono">{tx.status?.toUpperCase()}</span>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .central-tab {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding-bottom: 90px;
  }

  .card-profile {
    background: #181818;
    border: 2px solid var(--cat-black);
    box-shadow: 4px 4px 0px #000;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .profile-header {
    display: flex;
    align-items: center;
    gap: 12px;
    border-bottom: 1px solid #282828;
    padding-bottom: 14px;
  }

  .avatar-box {
    width: 46px;
    height: 46px;
    background: #252525;
    border: 2px solid var(--cat-yellow);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.5rem;
  }

  .profile-meta {
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .profile-username {
    color: var(--cat-yellow);
    font-size: 1.15rem;
  }

  .profile-id {
    font-size: 0.65rem;
    color: #888;
  }

  .btn-logout {
    background: #242424;
    color: #bbb;
    border: 1px solid #444;
    font-family: var(--font-heading);
    font-size: 0.75rem;
    padding: 6px 12px;
    cursor: pointer;
  }

  .btn-logout:hover {
    background: var(--cat-red);
    color: #fff;
    border-color: var(--cat-red);
  }

  .security-box {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .security-title {
    color: #fff;
    font-size: 0.85rem;
  }

  .security-items {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .sec-item {
    background: #121212;
    border: 1px solid #282828;
    padding: 10px 12px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
  }

  .sec-info {
    display: flex;
    flex-direction: column;
  }

  .sec-name {
    font-size: 0.75rem;
    font-weight: 700;
    color: #eee;
  }

  .sec-desc {
    font-size: 0.65rem;
    color: #777;
  }

  .btn-sec {
    background: #222;
    color: var(--cat-yellow);
    border: 1px solid var(--cat-yellow);
    font-family: var(--font-heading);
    font-size: 0.7rem;
    font-weight: 700;
    padding: 6px 10px;
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-sec-active {
    background: var(--cat-green);
    color: #000;
    border-color: #000;
  }

  .tx-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .tx-header {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .tx-title {
    color: #fff;
    font-size: 0.95rem;
  }

  .tx-filters {
    display: flex;
    gap: 4px;
    overflow-x: auto;
  }

  .btn-tx-filter {
    background: #181818;
    color: #777;
    border: 1px solid #282828;
    font-family: var(--font-heading);
    font-size: 0.65rem;
    padding: 4px 8px;
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-tx-filter.active {
    background: var(--cat-yellow);
    color: #000;
    border-color: #000;
  }

  .tx-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .tx-item {
    background: #161616;
    border: 1px solid #242424;
    padding: 10px 14px;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .tx-left {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .tx-type {
    color: #fff;
    font-size: 0.8rem;
  }

  .tx-date {
    font-size: 0.65rem;
    color: #666;
  }

  .tx-right {
    text-align: right;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .tx-amount {
    font-size: 0.9rem;
    font-weight: 700;
  }

  .tx-status {
    font-size: 0.6rem;
  }

  .status-sucesso { color: var(--cat-green); }
  .status-pendente { color: var(--cat-yellow); }
  .status-rejeitado { color: var(--cat-red); }

  .text-green { color: var(--cat-green); }
  .text-red { color: var(--cat-red); }

  .loading-box, .empty-box {
    background: #161616;
    border: 2px dashed #333;
    padding: 30px;
    text-align: center;
    color: #888;
  }
</style>
