<script>
  import { adminDashboardState } from '../state/admin.svelte.js';
  import { toastState } from '../state/toast.svelte.js';
  import { Modal, Button } from '@catbrasil/ui';

  let {
    isOpen = false,
    item = null,
    onclose
  } = $props();

  let id = $state('');
  let name = $state('');
  let category = $state('ESCAVADEIRAS');
  let price = $state(1000);
  let dailyYield = $state(45);
  let durationDays = $state(30);
  let stockQuantity = $state(999);
  let maxPurchasesPerUser = $state(5);
  let reinvestOnly = $state(false);
  let active = $state(true);

  $effect(() => {
    if (item) {
      id = item.id || '';
      name = item.name || '';
      category = item.category || 'ESCAVADEIRAS';
      price = item.price || 0;
      dailyYield = item.dailyYield || 0;
      durationDays = item.durationDays || 30;
      stockQuantity = item.stockQuantity || 999;
      maxPurchasesPerUser = item.maxPurchasesPerUser || 5;
      reinvestOnly = item.reinvestOnly || false;
      active = item.active !== false;
    } else {
      id = `model_${Date.now()}`;
      name = '';
      category = 'ESCAVADEIRAS';
      price = 500;
      dailyYield = 22;
      durationDays = 30;
      stockQuantity = 999;
      maxPurchasesPerUser = 5;
      reinvestOnly = false;
      active = true;
    }
  });

  const returnPercentPerDay = $derived(() => {
    if (!price || price <= 0) return 0;
    return Math.round(((dailyYield / price) * 100) * 10) / 10;
  });

  const totalContractReturn = $derived(() => {
    return Math.round((dailyYield * durationDays) * 100) / 100;
  });

  async function handleSave(e) {
    e.preventDefault();
    if (!name.trim()) {
      toastState.warning('Informe o nome da máquina.');
      return;
    }

    const payload = {
      id: id.trim().toLowerCase().replace(/\s+/g, '_'),
      name: name.trim(),
      category,
      price: parseFloat(price) || 0,
      dailyYield: parseFloat(dailyYield) || 0,
      returnPercentPerDay: returnPercentPerDay(),
      durationDays: parseInt(durationDays) || 30,
      totalContractReturn: totalContractReturn(),
      stockQuantity: parseInt(stockQuantity) || 999,
      maxPurchasesPerUser: parseInt(maxPurchasesPerUser) || 5,
      reinvestOnly,
      active,
    };

    const ok = await adminDashboardState.saveCatalogItem(payload);
    if (ok && onclose) {
      onclose();
    }
  }
</script>

<Modal {isOpen} title={item ? 'EDITAR MODELO DE FROTA' : 'CADASTRAR NOVO MODELO DE FROTA'} {onclose} maxWidth="max-w-md">
  <form onsubmit={handleSave} class="catalog-form font-mono">
    <div class="form-group">
      <label for="fleet-id" class="form-label">IDENTIFICADOR ÚNICO (SLUG)</label>
      <input
        id="fleet-id"
        type="text"
        class="input-cat"
        bind:value={id}
        readonly={!!item}
        required
      />
    </div>

    <div class="form-group">
      <label for="fleet-name" class="form-label">NOME COMERCIAL DA MÁQUINA</label>
      <input
        id="fleet-name"
        type="text"
        class="input-cat"
        placeholder="ex: Escavadeira CAT 336 Heavy"
        bind:value={name}
        required
      />
    </div>

    <div class="form-row">
      <div class="form-group">
        <label for="fleet-cat" class="form-label">CATEGORIA</label>
        <select id="fleet-cat" class="input-cat" bind:value={category}>
          <option value="ESCAVADEIRAS">ESCAVADEIRAS</option>
          <option value="PESADA">LINHA PESADA</option>
          <option value="CARREGADEIRAS">CARREGADEIRAS</option>
          <option value="CAMINHOES">CAMINHÕES FORA DE ESTRADA</option>
        </select>
      </div>

      <div class="form-group">
        <label for="fleet-price" class="form-label">PREÇO DE LOCAÇÃO (R$)</label>
        <input
          id="fleet-price"
          type="number"
          step="10"
          class="input-cat text-yellow font-bold"
          bind:value={price}
          required
        />
      </div>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label for="fleet-yield" class="form-label">RENDIMENTO 24H (R$)</label>
        <input
          id="fleet-yield"
          type="number"
          step="1"
          class="input-cat text-green font-bold"
          bind:value={dailyYield}
          required
        />
      </div>

      <div class="form-group">
        <label for="fleet-duration" class="form-label">DURAÇÃO (DIAS)</label>
        <input
          id="fleet-duration"
          type="number"
          class="input-cat"
          bind:value={durationDays}
          required
        />
      </div>
    </div>

    <!-- Prévia de Retorno Calculada -->
    <div class="calc-preview">
      <span>Retorno Diário: <strong class="text-green">+{returnPercentPerDay()}% / dia</strong></span>
      <span>Retorno Total: <strong class="text-yellow">R$ {totalContractReturn().toFixed(2)}</strong></span>
    </div>

    <div class="checkbox-row">
      <label class="checkbox-label">
        <input type="checkbox" bind:checked={reinvestOnly} />
        <span>Exclusivo Linha Reinvest Turbinado (Exige Saldo de Coletas)</span>
      </label>

      <label class="checkbox-label">
        <input type="checkbox" bind:checked={active} />
        <span>Modelo Ativo para Locação na Loja</span>
      </label>
    </div>

    <button
      type="submit"
      class="btn-cat btn-save-fleet"
      disabled={adminDashboardState.isLoading}
    >
      {#if adminDashboardState.isLoading}
        <span>⚙️ SALVANDO NO POSTGRESQL...</span>
      {:else}
        <span>SALVAR MODELO DE FROTA 🚜</span>
      {/if}
    </button>
  </form>
</Modal>

<style>
  .catalog-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
    font-size: 0.75rem;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
  }

  .form-row {
    display: flex;
    gap: 10px;
  }

  .form-label {
    font-size: 0.65rem;
    color: #888;
    font-weight: 700;
  }

  .calc-preview {
    background: #111;
    border: 1px solid #282828;
    padding: 8px 12px;
    display: flex;
    justify-content: space-between;
    font-size: 0.7rem;
  }

  .checkbox-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 4px 0;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #bbb;
    font-size: 0.7rem;
    cursor: pointer;
  }

  .btn-save-fleet {
    width: 100%;
    padding: 12px;
    margin-top: 6px;
  }

  .text-green { color: var(--cat-green); }
  .text-yellow { color: var(--cat-yellow); }
</style>
