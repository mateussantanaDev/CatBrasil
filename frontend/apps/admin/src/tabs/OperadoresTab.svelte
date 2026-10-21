<script>
  import { onMount } from 'svelte';
  import { adminDashboardState } from '../state/admin.svelte.js';
  import UserAdjustModal from '../components/UserAdjustModal.svelte';
  import { Button, Card, Badge } from '@catbrasil/ui';

  const users = $derived(adminDashboardState.users);

  let searchQuery = $state('');
  let statusFilter = $state('');
  let isAdjustModalOpen = $state(false);
  let selectedUser = $state(null);

  onMount(() => {
    adminDashboardState.loadUsers();
  });

  function handleSearch(e) {
    e.preventDefault();
    adminDashboardState.loadUsers(searchQuery, statusFilter);
  }

  function openAdjust(user) {
    selectedUser = user;
    isAdjustModalOpen = true;
  }

  async function toggleLock(user) {
    const isLocked = user.isWithdrawalLocked;
    const msg = isLocked
      ? `Desbloquear saques do operador @${user.username}?`
      : `ATENÇÃO: Bloquear saques do operador @${user.username}?`;

    if (confirm(msg)) {
      await adminDashboardState.updateUserStatus(user.id, {
        isWithdrawalLocked: !isLocked,
      });
    }
  }
</script>

<div class="operadores-tab">
  <div class="tab-top-bar">
    <div>
      <span class="tab-subtitle font-mono">BASE DE DADOS DE OPERADORES & CARTEIRAS</span>
      <h2 class="tab-title">AUDITORIA DE OPERADORES ({users.length}) 👥</h2>
    </div>
    <form onsubmit={handleSearch} class="search-form font-mono">
      <input
        type="text"
        class="input-search"
        placeholder="Buscar por nome, telefone ou ID..."
        bind:value={searchQuery}
      />
      <button type="submit" class="btn-search">BUSCAR</button>
    </form>
  </div>

  {#if adminDashboardState.isLoading && users.length === 0}
    <div class="loading-box">
      <span class="animate-spin text-2xl">⚙️</span>
      <p>Consultando base de dados dos operadores...</p>
    </div>
  {:else}
    <div class="users-table-wrapper font-mono">
      <table class="users-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>OPERADOR</th>
            <th>TELEFONE</th>
            <th>SALDO (R$)</th>
            <th>ROLLOVER</th>
            <th>NÍVEL</th>
            <th>CADASTRO</th>
            <th>SAQUES</th>
            <th>AÇÕES</th>
          </tr>
        </thead>
        <tbody>
          {#each users as u (u.id)}
            <tr>
              <td>
                <span class="id-tag">{u.id}</span>
              </td>
              <td>
                <strong class="user-name">@{u.username}</strong>
                {#if u.role === 'admin' || u.role === 'master'}
                  <span class="badge-role">{u.role?.toUpperCase()}</span>
                {/if}
              </td>
              <td>
                <span class="user-phone">{u.phone || 'N/A'}</span>
              </td>
              <td>
                <strong class="text-green">R$ {(u.balance || 0).toFixed(2)}</strong>
              </td>
              <td>
                <span class="rollover-stat">
                  R$ {(u.currentRollover || 0).toFixed(2)} / {(u.requiredRollover || 0).toFixed(2)}
                </span>
              </td>
              <td>
                <span class="badge-level">LVL {u.nivel || 1}</span>
              </td>
              <td>
                <span class="user-date">{new Date(u.createdAt).toLocaleDateString('pt-BR')}</span>
              </td>
              <td>
                {#if u.isWithdrawalLocked}
                  <span class="status-locked">🔒 BLOQUEADO</span>
                {:else}
                  <span class="status-unlocked">✅ LIBERADO</span>
                {/if}
              </td>
              <td>
                <div class="action-btns">
                  <button class="btn-action-bal" onclick={() => openAdjust(u)} title="Ajustar Saldo">
                    💰 SALDO
                  </button>
                  <button
                    class="btn-action-lock {u.isWithdrawalLocked ? 'btn-unlock' : ''}"
                    onclick={() => toggleLock(u)}
                    title="Alternar Bloqueio de Saques"
                  >
                    {u.isWithdrawalLocked ? '🔓 DESTRAVAR' : '🔒 TRAVAR'}
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <UserAdjustModal
    isOpen={isAdjustModalOpen}
    user={selectedUser}
    onclose={() => isAdjustModalOpen = false}
  />
</div>

<style>
  .operadores-tab {
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
    gap: 12px;
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

  .search-form {
    display: flex;
    gap: 6px;
  }

  .input-search {
    background: #0f0f0f;
    border: 1px solid #333;
    color: #fff;
    padding: 8px 12px;
    font-size: 0.75rem;
    width: 250px;
  }

  .btn-search {
    background: var(--cat-yellow);
    color: #000;
    border: 1px solid #000;
    padding: 8px 12px;
    font-weight: 700;
    cursor: pointer;
  }

  .users-table-wrapper {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    overflow-x: auto;
  }

  .users-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.75rem;
  }

  .users-table th {
    background: #111;
    color: #888;
    padding: 12px 14px;
    border-bottom: 2px solid #282828;
    font-size: 0.65rem;
    font-weight: 700;
  }

  .users-table td {
    padding: 10px 14px;
    border-bottom: 1px solid #222;
  }

  .id-tag {
    background: #222;
    color: #888;
    padding: 2px 6px;
    font-size: 0.65rem;
  }

  .user-name {
    color: #fff;
  }

  .badge-role {
    background: var(--cat-yellow);
    color: #000;
    font-size: 0.55rem;
    font-weight: 800;
    padding: 1px 4px;
    margin-left: 4px;
  }

  .user-phone {
    color: #aaa;
    font-size: 0.7rem;
  }

  .rollover-stat {
    font-size: 0.7rem;
    color: #aaa;
  }

  .badge-level {
    background: #222;
    color: var(--cat-yellow);
    border: 1px solid var(--cat-yellow);
    padding: 2px 6px;
    font-size: 0.65rem;
    font-weight: 700;
  }

  .user-date {
    font-size: 0.65rem;
    color: #777;
  }

  .status-locked { color: var(--cat-red); font-weight: 700; font-size: 0.65rem; }
  .status-unlocked { color: var(--cat-green); font-size: 0.65rem; }

  .action-btns {
    display: flex;
    gap: 4px;
  }

  .btn-action-bal {
    background: #252525;
    color: var(--cat-yellow);
    border: 1px solid #444;
    padding: 4px 8px;
    font-size: 0.65rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-action-lock {
    background: #333;
    color: #ff5252;
    border: 1px solid #444;
    padding: 4px 8px;
    font-size: 0.65rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-unlock {
    color: var(--cat-green);
  }

  .text-green { color: var(--cat-green); }

  .loading-box {
    background: #161616;
    border: 2px dashed #333;
    padding: 40px;
    text-align: center;
    color: #888;
  }
</style>
