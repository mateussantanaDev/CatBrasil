<script>
  import { onMount } from 'svelte';
  import { adminDashboardState } from '../state/admin.svelte.js';
  import CatalogEditModal from '../components/CatalogEditModal.svelte';
  import { Button, Card, Badge } from '@catbrasil/ui';

  const catalog = $derived(adminDashboardState.catalog);

  let isModalOpen = $state(false);
  let editingItem = $state(null);

  onMount(() => {
    adminDashboardState.loadCatalog();
  });

  function openCreate() {
    editingItem = null;
    isModalOpen = true;
  }

  function openEdit(item) {
    editingItem = item;
    isModalOpen = true;
  }

  async function handleDelete(id) {
    if (confirm(`Tem certeza que deseja remover o modelo '${id}' do catálogo?`)) {
      await adminDashboardState.deleteCatalogItem(id);
    }
  }
</script>

<div class="frota-tab">
  <div class="tab-top-bar">
    <div>
      <span class="tab-subtitle font-mono">CATÁLOGO DE MÁQUINAS & RETORNOS</span>
      <h2 class="tab-title">GESTÃO DE FROTAS CATERPILLAR ({catalog.length}) 🚜</h2>
    </div>
    <div class="top-actions">
      <button class="btn-create font-mono" onclick={openCreate}>+ CADASTRAR MODELO</button>
      <button class="btn-refresh font-mono" onclick={() => adminDashboardState.loadCatalog()}>🔄 ATUALIZAR</button>
    </div>
  </div>

  {#if adminDashboardState.isLoading && catalog.length === 0}
    <div class="loading-box">
      <span class="animate-spin text-2xl">⚙️</span>
      <p>Carregando catálogo de frotas...</p>
    </div>
  {:else}
    <div class="catalog-table-wrapper font-mono">
      <table class="catalog-table">
        <thead>
          <tr>
            <th>ID SLUG</th>
            <th>NOME DO MODELO</th>
            <th>CATEGORIA</th>
            <th>PREÇO (R$)</th>
            <th>RENDIMENTO 24H</th>
            <th>RETORNO %</th>
            <th>DURAÇÃO</th>
            <th>REINVEST</th>
            <th>STATUS</th>
            <th>AÇÕES</th>
          </tr>
        </thead>
        <tbody>
          {#each catalog as item (item.id)}
            <tr>
              <td>
                <span class="id-tag">{item.id}</span>
              </td>
              <td>
                <strong class="item-name">{item.name}</strong>
              </td>
              <td>
                <span class="cat-tag">{item.category}</span>
              </td>
              <td>
                <strong class="text-yellow">R$ {item.price.toFixed(2)}</strong>
              </td>
              <td>
                <strong class="text-green">R$ {item.dailyYield.toFixed(2)} / dia</strong>
              </td>
              <td>
                <span class="text-green">+{item.returnPercentPerDay.toFixed(1)}%</span>
              </td>
              <td>
                <span>{item.durationDays}d</span>
              </td>
              <td>
                {#if item.reinvestOnly}
                  <span class="badge-reinvest">⚡ REINVEST</span>
                {:else}
                  <span class="badge-padrao">PADRÃO</span>
                {/if}
              </td>
              <td>
                {#if item.active !== false}
                  <span class="status-active">ATIVO</span>
                {:else}
                  <span class="status-inactive">INATIVO</span>
                {/if}
              </td>
              <td>
                <div class="action-btns">
                  <button class="btn-edit" onclick={() => openEdit(item)}>EDITAR</button>
                  <button class="btn-del" onclick={() => handleDelete(item.id)}>✕</button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <CatalogEditModal
    isOpen={isModalOpen}
    item={editingItem}
    onclose={() => isModalOpen = false}
  />
</div>

<style>
  .frota-tab {
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

  .top-actions {
    display: flex;
    gap: 8px;
  }

  .btn-create {
    background: var(--cat-yellow);
    color: var(--cat-black);
    border: 1px solid #000;
    padding: 8px 14px;
    font-size: 0.75rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-refresh {
    background: #222;
    color: #bbb;
    border: 1px solid #444;
    padding: 8px 12px;
    font-size: 0.75rem;
    font-weight: 700;
    cursor: pointer;
  }

  .catalog-table-wrapper {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    overflow-x: auto;
  }

  .catalog-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.75rem;
  }

  .catalog-table th {
    background: #111;
    color: #888;
    padding: 12px 14px;
    border-bottom: 2px solid #282828;
    font-size: 0.65rem;
    font-weight: 700;
  }

  .catalog-table td {
    padding: 10px 14px;
    border-bottom: 1px solid #222;
  }

  .id-tag {
    background: #222;
    color: #888;
    padding: 2px 6px;
    font-size: 0.65rem;
  }

  .item-name {
    color: #fff;
  }

  .cat-tag {
    color: #aaa;
    font-size: 0.7rem;
  }

  .badge-reinvest {
    background: #8E24AA;
    color: #fff;
    padding: 2px 6px;
    font-size: 0.6rem;
    font-weight: 700;
  }

  .badge-padrao {
    background: #282828;
    color: #888;
    padding: 2px 6px;
    font-size: 0.6rem;
  }

  .status-active { color: var(--cat-green); font-weight: 700; }
  .status-inactive { color: var(--cat-red); font-weight: 700; }

  .action-btns {
    display: flex;
    gap: 4px;
  }

  .btn-edit {
    background: #282828;
    color: var(--cat-yellow);
    border: 1px solid #444;
    padding: 4px 8px;
    font-size: 0.65rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-del {
    background: #333;
    color: #ff5252;
    border: 1px solid #444;
    padding: 4px 8px;
    font-size: 0.65rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-del:hover {
    background: var(--cat-red);
    color: #fff;
  }

  .text-yellow { color: var(--cat-yellow); }
  .text-green { color: var(--cat-green); }

  .loading-box {
    background: #161616;
    border: 2px dashed #333;
    padding: 40px;
    text-align: center;
    color: #888;
  }
</style>
