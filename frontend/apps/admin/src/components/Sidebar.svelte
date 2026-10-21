<script>
  import { adminAuthState } from '../state/auth.svelte.js';
  import { adminDashboardState } from '../state/admin.svelte.js';

  let {
    activeTab = 'dre',
    ontabchange
  } = $props();

  const user = $derived(adminAuthState.user);
  const pendingWithdrawalsCount = $derived(adminDashboardState.withdrawals.length);

  const menuItems = $derived([
    { id: 'dre', label: 'DRE CONTÁBIL', icon: '📊' },
    { id: 'saques', label: 'FILA DE SAQUES', icon: '💰', badge: pendingWithdrawalsCount },
    { id: 'roleta', label: 'MATRIZ ROLETA', icon: '🎰' },
    { id: 'frota', label: 'GESTÃO FROTAS', icon: '🚜' },
    { id: 'operadores', label: 'OPERADORES', icon: '👥' },
    { id: 'comunicados', label: 'COMUNICADOS', icon: '📢' },
    { id: 'config', label: 'CONFIGURAÇÕES', icon: '⚙️' },
  ]);
</script>

<aside class="erp-sidebar">
  <div class="sidebar-header">
    <div class="logo-box">
      <span class="logo-icon">🚜</span>
      <h1 class="logo-title">CATBRASIL</h1>
    </div>
    <span class="logo-badge font-mono">ERP & GESTÃO v3.0</span>
  </div>

  <!-- Identificação do Administrador -->
  <div class="admin-user-card font-mono">
    <span class="admin-user-lbl">LOGADO COMO:</span>
    <strong class="admin-user-name">@{user?.username || 'Administrador'}</strong>
    <span class="admin-user-role">PERFIL: {user?.role?.toUpperCase() || 'ADMIN'}</span>
  </div>

  <!-- Menu de Navegação -->
  <nav class="sidebar-nav">
    {#each menuItems as item}
      <button
        class="nav-btn {activeTab === item.id ? 'active' : ''}"
        onclick={() => ontabchange && ontabchange(item.id)}
      >
        <span class="nav-icon">{item.icon}</span>
        <span class="nav-label">{item.label}</span>
        {#if item.badge && item.badge > 0}
          <span class="nav-badge font-mono">{item.badge}</span>
        {/if}
      </button>
    {/each}
  </nav>

  <!-- Rodapé com Logout -->
  <div class="sidebar-footer">
    <button class="btn-logout-erp font-mono" onclick={() => adminAuthState.logout()}>
      🔒 ENCERRAR SESSÃO ERP
    </button>
  </div>
</aside>

<style>
  .erp-sidebar {
    width: 240px;
    background: #111111;
    border-right: 3px solid var(--cat-yellow);
    display: flex;
    flex-direction: column;
    height: 100vh;
    position: sticky;
    top: 0;
    flex-shrink: 0;
  }

  .sidebar-header {
    padding: 18px 16px 14px;
    border-bottom: 2px solid #222;
    text-align: center;
  }

  .logo-box {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .logo-icon {
    font-size: 1.5rem;
  }

  .logo-title {
    font-size: 1.4rem;
    color: var(--cat-yellow);
    letter-spacing: 1.5px;
  }

  .logo-badge {
    display: inline-block;
    background: #202020;
    color: var(--cat-yellow);
    border: 1px solid var(--cat-yellow);
    font-size: 0.55rem;
    font-weight: 800;
    padding: 2px 6px;
    margin-top: 4px;
  }

  .admin-user-card {
    background: #161616;
    border-bottom: 1px solid #242424;
    padding: 10px 16px;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .admin-user-lbl {
    font-size: 0.55rem;
    color: #777;
  }

  .admin-user-name {
    color: #fff;
    font-size: 0.8rem;
  }

  .admin-user-role {
    font-size: 0.6rem;
    color: var(--cat-yellow);
  }

  .sidebar-nav {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: 10px 8px;
    gap: 4px;
    overflow-y: auto;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    background: transparent;
    border: 1.5px solid transparent;
    color: #888;
    font-family: var(--font-heading);
    font-size: 0.8rem;
    font-weight: 700;
    text-align: left;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .nav-btn:hover {
    color: #fff;
    background: #181818;
  }

  .nav-btn.active {
    background: #1c1c1c;
    color: var(--cat-yellow);
    border-color: var(--cat-yellow);
    box-shadow: 2px 2px 0px #000;
  }

  .nav-icon {
    font-size: 1.1rem;
  }

  .nav-label {
    flex: 1;
    letter-spacing: 0.5px;
  }

  .nav-badge {
    background: var(--cat-red);
    color: #fff;
    font-size: 0.65rem;
    font-weight: 800;
    padding: 1px 6px;
    border-radius: 2px;
  }

  .sidebar-footer {
    padding: 12px;
    border-top: 1px solid #222;
  }

  .btn-logout-erp {
    width: 100%;
    background: #1a1a1a;
    color: #bbb;
    border: 1px solid #333;
    padding: 10px;
    font-size: 0.7rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-logout-erp:hover {
    background: var(--cat-red);
    color: #fff;
    border-color: var(--cat-red);
  }

  @media (max-width: 900px) {
    .erp-sidebar {
      width: 70px;
    }
    .logo-title, .logo-badge, .admin-user-card, .nav-label, .btn-logout-erp {
      display: none;
    }
    .sidebar-header {
      padding: 12px 6px;
    }
    .nav-btn {
      justify-content: center;
      padding: 12px 0;
    }
  }
</style>
