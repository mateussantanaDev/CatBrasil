<script>
  import { onMount } from 'svelte';
  import { adminDashboardState } from '../state/admin.svelte.js';
  import { toastState } from '../state/toast.svelte.js';
  import { Button, Card, Badge } from '@catbrasil/ui';

  let localSlices = $state([]);

  onMount(async () => {
    await adminDashboardState.loadRouletteMatrix();
    localSlices = JSON.parse(JSON.stringify(adminDashboardState.rouletteMatrix || []));
  });

  const totalProb = $derived(() => {
    const sum = localSlices.reduce((acc, s) => acc + (parseFloat(s.probability) || 0), 0);
    return Math.round(sum * 1000) / 1000;
  });

  const isValidProb = $derived(() => {
    return Math.abs(totalProb() - 100.0) < 0.001;
  });

  const expectedValueEV = $derived(() => {
    const ev = localSlices.reduce((acc, s) => {
      const amt = parseFloat(s.amount) || 0;
      const prob = parseFloat(s.probability) || 0;
      return acc + (amt * (prob / 100.0));
    }, 0);
    return Math.round(ev * 100) / 100;
  });

  function addSlice() {
    localSlices.push({
      id: localSlices.length + 1,
      label: 'Novo Prêmio',
      amount: 0,
      probability: 0,
      color: '#262626',
      textColor: '#FFFFFF'
    });
  }

  function removeSlice(idx) {
    localSlices.splice(idx, 1);
  }

  async function handleSave() {
    if (!isValidProb) {
      toastState.error(`A soma das probabilidades deve ser exatamente 100.000% (Atual: ${totalProb()}%)`);
      return;
    }
    await adminDashboardState.saveRouletteMatrix(localSlices);
  }
</script>

<div class="roleta-matrix-tab">
  <div class="tab-top-bar">
    <div>
      <span class="tab-subtitle font-mono">MATRIZ PONDERADA & MODELAGEM DE EV</span>
      <h2 class="tab-title">CONFIGURAÇÃO DA ROLETA DIÁRIA 🎰</h2>
    </div>
    <div class="top-actions">
      <button class="btn-add font-mono" onclick={addSlice}>+ ADICIONAR FATIA</button>
      <button
        class="btn-save font-mono {isValidProb ? 'btn-save-active' : ''}"
        disabled={!isValidProb || adminDashboardState.isLoading}
        onclick={handleSave}
      >
        💾 SALVAR MATRIZ (POSTGRESQL)
      </button>
    </div>
  </div>

  <!-- Indicadores Matemáticos em Tempo Real -->
  <div class="matrix-kpis font-mono">
    <div class="matrix-kpi {isValidProb ? 'kpi-valid' : 'kpi-invalid'}">
      <span class="kpi-lbl">SOMA TOTAL DE PROBABILIDADES:</span>
      <strong class="kpi-num">{totalProb().toFixed(3)}% / 100.000%</strong>
      <span class="kpi-status">{isValidProb ? '✅ VÁLIDO (SOMA 100%)' : '⚠️ ERRO: DEVE SOMAR EXATAMENTE 100%'}</span>
    </div>

    <div class="matrix-kpi kpi-ev">
      <span class="kpi-lbl">VALOR ESPERADO MATEMÁTICO (EV):</span>
      <strong class="kpi-num text-yellow">R$ {expectedValueEV().toFixed(2)} / giro</strong>
      <span class="kpi-status">Custo médio por operador diário</span>
    </div>
  </div>

  <!-- Tabela de Edição da Matriz -->
  <div class="matrix-table-wrapper font-mono">
    <table class="matrix-table">
      <thead>
        <tr>
          <th>#</th>
          <th>LABEL DO PRÊMIO</th>
          <th>VALOR (R$)</th>
          <th>PROBABILIDADE (%)</th>
          <th>COR FUNDO</th>
          <th>COR TEXTO</th>
          <th>PRÉVIA</th>
          <th>AÇÃO</th>
        </tr>
      </thead>
      <tbody>
        {#each localSlices as slice, i}
          <tr>
            <td>{i + 1}</td>
            <td>
              <input type="text" class="input-cell" bind:value={slice.label} />
            </td>
            <td>
              <input type="number" step="0.5" class="input-cell font-bold" bind:value={slice.amount} />
            </td>
            <td>
              <input type="number" step="0.1" class="input-cell font-bold text-yellow" bind:value={slice.probability} />
            </td>
            <td>
              <input type="color" class="color-picker" bind:value={slice.color} />
            </td>
            <td>
              <input type="color" class="color-picker" bind:value={slice.textColor} />
            </td>
            <td>
              <span
                class="slice-preview font-bold"
                style="background: {slice.color}; color: {slice.textColor};"
              >
                {slice.label}
              </span>
            </td>
            <td>
              <button class="btn-del" onclick={() => removeSlice(i)}>✕</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  .roleta-matrix-tab {
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

  .btn-add {
    background: #252525;
    color: #fff;
    border: 1px solid #444;
    padding: 8px 14px;
    font-size: 0.75rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-save {
    background: #202020;
    color: #555;
    border: 1.5px solid #333;
    padding: 8px 16px;
    font-size: 0.75rem;
    font-weight: 700;
    cursor: not-allowed;
  }

  .btn-save-active {
    background: var(--cat-green);
    color: #000;
    border-color: #000;
    cursor: pointer;
  }

  .matrix-kpis {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .matrix-kpi {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 3px 3px 0px #000;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .kpi-valid {
    border-color: var(--cat-green);
  }

  .kpi-invalid {
    border-color: var(--cat-red);
  }

  .kpi-lbl {
    font-size: 0.65rem;
    color: #888;
  }

  .kpi-num {
    font-size: 1.35rem;
  }

  .kpi-status {
    font-size: 0.65rem;
    color: #aaa;
  }

  .matrix-table-wrapper {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    overflow-x: auto;
  }

  .matrix-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.75rem;
  }

  .matrix-table th {
    background: #111;
    color: #888;
    padding: 12px 14px;
    border-bottom: 2px solid #282828;
    font-size: 0.65rem;
    font-weight: 700;
  }

  .matrix-table td {
    padding: 10px 14px;
    border-bottom: 1px solid #222;
  }

  .input-cell {
    background: #0f0f0f;
    border: 1px solid #333;
    color: #fff;
    padding: 6px 8px;
    font-size: 0.75rem;
    width: 100%;
  }

  .color-picker {
    width: 36px;
    height: 28px;
    background: transparent;
    border: 1px solid #444;
    cursor: pointer;
  }

  .slice-preview {
    padding: 3px 8px;
    border: 1px solid #000;
    font-size: 0.7rem;
    display: inline-block;
  }

  .btn-del {
    background: #333;
    color: #ff5252;
    border: 1px solid #444;
    width: 26px;
    height: 26px;
    cursor: pointer;
    font-weight: 700;
  }

  .btn-del:hover {
    background: var(--cat-red);
    color: #fff;
  }

  .text-yellow { color: var(--cat-yellow); }
</style>
