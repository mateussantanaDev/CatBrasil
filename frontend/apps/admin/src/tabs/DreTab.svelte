<script>
  import { onMount } from 'svelte';
  import { adminDashboardState } from '../state/admin.svelte.js';
  import { Button, Card, Badge } from '@catbrasil/ui';

  const dre = $derived(adminDashboardState.dre);

  onMount(() => {
    adminDashboardState.loadDRE();
  });
</script>

<div class="dre-tab">
  <!-- Top Bar com Ação de Atualização -->
  <div class="tab-top-bar">
    <div>
      <span class="tab-subtitle font-mono">DEMONSTRATIVO DO RESULTADO DO EXERCÍCIO (DRE v3.0)</span>
      <h2 class="tab-title">AUDITORIA CONTÁBIL & LIQUIDEZ EM TEMPO REAL 📊</h2>
    </div>
    <button class="btn-refresh font-mono" onclick={() => adminDashboardState.loadDRE()}>
      🔄 ATUALIZAR MÉTRICAS
    </button>
  </div>

  {#if adminDashboardState.isLoading && !dre}
    <div class="loading-box">
      <span class="animate-spin text-2xl">⚙️</span>
      <p>Calculando balanço contábil da casa...</p>
    </div>
  {:else if dre}
    <!-- Grid de Métricas Principais (KPIs) -->
    <div class="kpi-grid font-mono">
      <div class="kpi-card kpi-deposits">
        <span class="kpi-lbl">RECEITA BRUTA (DEPÓSITOS PIX):</span>
        <strong class="kpi-val text-green">R$ {(dre.totalDeposits || 0).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong>
        <span class="kpi-sub">Taxas de Gateway: - R$ {(dre.gatewayFees || 0).toFixed(2)} ({dre.gatewayFeePercent || 2.5}%)</span>
      </div>

      <div class="kpi-card kpi-net-deposits">
        <span class="kpi-lbl">DEPÓSITOS LÍQUIDOS ENTRADOS:</span>
        <strong class="kpi-val text-yellow">R$ {(dre.netDeposits || 0).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong>
        <span class="kpi-sub">Recurso efetivo disponível no cofre</span>
      </div>

      <div class="kpi-card kpi-withdrawals">
        <span class="kpi-lbl">TOTAL DE SAQUES PAGOS:</span>
        <strong class="kpi-val text-red">- R$ {(dre.totalWithdrawals || 0).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong>
        <span class="kpi-sub">Saques liberados aos operadores</span>
      </div>

      <div class="kpi-card kpi-profit">
        <span class="kpi-lbl">LUCRO LÍQUIDO DA CASA:</span>
        <strong class="kpi-val {(dre.netBankProfit || 0) >= 0 ? 'text-green' : 'text-red'}">
          R$ {(dre.netBankProfit || 0).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
        </strong>
        <span class="kpi-sub">Margem da Banca: {dre.profitMargin || 0}%</span>
      </div>
    </div>

    <!-- Seção de Métricas Avançadas e Retenção Empírica -->
    <div class="metrics-split">
      <!-- Cobertura de Caixa -->
      <div class="card-metric-block">
        <h4 class="block-title">ÍNDICE DE COBERTURA DE LIQUIDEZ (LCR)</h4>
        <div class="gauge-display font-mono">
          <div class="gauge-val text-yellow">
            {dre.liquidityCoverageRatio || 100}%
          </div>
          <p class="gauge-desc">
            Razão entre o Lucro Líquido em Caixa e o Saldo Total Custodiado nas Carteiras dos Operadores (R$ {(dre.totalUserBalances || 0).toFixed(2)}).
          </p>
        </div>
      </div>

      <!-- Retenção por Tolerância de 10 min -->
      <div class="card-metric-block">
        <h4 class="block-title">RETENÇÃO EMPÍRICA (TOLERÂNCIA 10 MIN) ⏳</h4>
        <div class="retention-details font-mono">
          <div class="retention-row">
            <span>RENDIMENTOS RETIDOS (EXPIRADOS):</span>
            <strong class="text-green">+ R$ {(dre.retainedExpiredYields || 0).toFixed(2)}</strong>
          </div>
          <div class="retention-row">
            <span>COLETAS EXPIRADAS vs SUCESSO:</span>
            <span>{dre.expiredCollectCount || 0} Expiradas / {dre.successCollectCount || 0} Coletadas</span>
          </div>
          <div class="retention-row">
            <span>TAXA EMPÍRICA DE ESQUECIMENTO:</span>
            <strong class="text-yellow">{dre.empiricalForgetRate || 33.3}%</strong>
          </div>
          <div class="retention-row">
            <span>PRÊMIOS DE ROLETA RETIDOS:</span>
            <span>R$ {(dre.retainedUnspunPrizes || 0).toFixed(2)}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Formulação Contábil Oficial -->
    <div class="card-formula font-mono">
      <h4 class="formula-title">FORMULAÇÃO MATEMÁTICA CONSOLIDADA:</h4>
      <p class="formula-line">1. Receita Líquida = Depósitos Brutos - Taxa Gateway PIX (2,5%)</p>
      <p class="formula-line">2. Lucro da Banca = Receita Líquida - Saques Pagos + Rendimentos Retidos por Expiração</p>
      <p class="formula-line">3. Margem Líquida = (Lucro da Banca / Depósitos Brutos) × 100%</p>
    </div>
  {/if}
</div>

<style>
  .dre-tab {
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
    letter-spacing: 0.5px;
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

  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 12px;
  }

  .kpi-card {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .kpi-lbl {
    font-size: 0.65rem;
    color: #888;
    font-weight: 700;
  }

  .kpi-val {
    font-size: 1.45rem;
    margin: 4px 0;
  }

  .kpi-sub {
    font-size: 0.65rem;
    color: #666;
  }

  .metrics-split {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .card-metric-block {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    padding: 16px;
  }

  .block-title {
    color: var(--cat-yellow);
    font-size: 0.9rem;
    margin-bottom: 12px;
    border-bottom: 1px solid #282828;
    padding-bottom: 6px;
  }

  .gauge-display {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .gauge-val {
    font-size: 2.2rem;
    font-weight: 800;
  }

  .gauge-desc {
    font-size: 0.75rem;
    color: #888;
    line-height: 1.4;
  }

  .retention-details {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 0.75rem;
  }

  .retention-row {
    display: flex;
    justify-content: space-between;
    border-bottom: 1px dashed #242424;
    padding-bottom: 4px;
    color: #aaa;
  }

  .card-formula {
    background: #111;
    border: 1px solid #282828;
    padding: 14px 16px;
    font-size: 0.75rem;
    color: #777;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .formula-title {
    color: #aaa;
    font-size: 0.8rem;
    margin-bottom: 4px;
  }

  .text-green { color: var(--cat-green); }
  .text-yellow { color: var(--cat-yellow); }
  .text-red { color: var(--cat-red); }

  .loading-box {
    background: #161616;
    border: 2px dashed #333;
    padding: 40px;
    text-align: center;
    color: #888;
  }

  @media (max-width: 768px) {
    .metrics-split {
      grid-template-columns: 1fr;
    }
  }
</style>
