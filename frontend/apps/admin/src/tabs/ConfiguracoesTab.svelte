<script>
  import { onMount } from 'svelte';
  import { adminDashboardState } from '../state/admin.svelte.js';
  import { toastState } from '../state/toast.svelte.js';
  import { Button, Card, Badge } from '@catbrasil/ui';

  let withdrawalsEnabled = $state(true);
  let autoApproveLimit = $state(150);
  let gatewayFeePercent = $state(2.5);
  let collectionGraceMinutes = $state(10);
  let levelBonusPercent = $state(5);
  let referralCommissionPercent = $state(5);
  let isSaving = $state(false);

  onMount(async () => {
    await adminDashboardState.loadConfig();
    const cfg = adminDashboardState.config;
    if (cfg) {
      withdrawalsEnabled = cfg.withdrawalsEnabled !== false;
      autoApproveLimit = cfg.autoApproveLimit ?? 150;
      gatewayFeePercent = cfg.gatewayFeePercent ?? 2.5;
      collectionGraceMinutes = cfg.collectionGraceMinutes ?? 10;
      levelBonusPercent = cfg.levelBonusPercent ?? 5;
      referralCommissionPercent = cfg.referralCommissionPercent ?? 5;
    }
  });

  async function handleSave(e) {
    e.preventDefault();
    isSaving = true;
    const payload = {
      withdrawalsEnabled,
      autoApproveLimit: parseFloat(autoApproveLimit) || 0,
      gatewayFeePercent: parseFloat(gatewayFeePercent) || 2.5,
      collectionGraceMinutes: parseInt(collectionGraceMinutes) || 10,
      levelBonusPercent: parseFloat(levelBonusPercent) || 5,
      referralCommissionPercent: parseFloat(referralCommissionPercent) || 5,
    };

    await adminDashboardState.saveConfig(payload);
    isSaving = false;
  }
</script>

<div class="config-tab font-mono">
  <div class="tab-top-bar">
    <div>
      <span class="tab-subtitle">DIRETRIZES DO SISTEMA & TRAVAS DE SEGURANÇA</span>
      <h2 class="tab-title">CONFIGURAÇÕES GLOBAIS DA BANCA ⚙️</h2>
    </div>
  </div>

  <form onsubmit={handleSave} class="config-form">
    <!-- Bloco de Segurança e Travas de Emergência -->
    <div class="config-section section-emergency">
      <h3 class="section-heading text-red">🚨 TRAVAS DE SEGURANÇA & KILL SWITCH</h3>

      <div class="control-row">
        <label class="toggle-control">
          <input type="checkbox" bind:checked={withdrawalsEnabled} />
          <div class="toggle-text">
            <strong class="toggle-title {withdrawalsEnabled ? 'text-green' : 'text-red'}">
              MODO SAQUES PIX: {withdrawalsEnabled ? 'HABILITADO ✅' : 'TRAVA DE EMERGÊNCIA ATIVA 🚫'}
            </strong>
            <span class="toggle-desc">Se desativado, nenhum saque será processado na plataforma.</span>
          </div>
        </label>
      </div>
    </div>

    <!-- Parâmetros Operacionais -->
    <div class="config-section">
      <h3 class="section-heading text-yellow">⚙️ PARÂMETROS OPERACIONAIS & REGRAS</h3>

      <div class="fields-grid">
        <div class="field-item">
          <label for="cfg-auto-approve" class="field-lbl">LIMITE PARA APROVAÇÃO AUTOMÁTICA DE SAQUE (R$):</label>
          <input
            id="cfg-auto-approve"
            type="number"
            step="10"
            class="input-cat text-green font-bold"
            bind:value={autoApproveLimit}
          />
          <span class="field-hint">Saques abaixo deste valor são liberados instantaneamente.</span>
        </div>

        <div class="field-item">
          <label for="cfg-grace-min" class="field-lbl">JANELA DE TOLERÂNCIA DE COLETA (MINUTOS):</label>
          <input
            id="cfg-grace-min"
            type="number"
            step="1"
            class="input-cat text-yellow font-bold"
            bind:value={collectionGraceMinutes}
          />
          <span class="field-hint">Tempo após 24h para coletar rendimento antes de expirar (Padrão: 10 min).</span>
        </div>

        <div class="field-item">
          <label for="cfg-gateway-fee" class="field-lbl">TAXA DE PROCESSAMENTO GATEWAY PIX (%):</label>
          <input
            id="cfg-gateway-fee"
            type="number"
            step="0.1"
            class="input-cat"
            bind:value={gatewayFeePercent}
          />
          <span class="field-hint">Percentual descontado pelo adquirente em depósitos (Padrão: 2.5%).</span>
        </div>

        <div class="field-item">
          <label for="cfg-lvl-bonus" class="field-lbl">BÔNUS DE RENDIMENTO POR NÍVEL (%):</label>
          <input
            id="cfg-lvl-bonus"
            type="number"
            step="0.5"
            class="input-cat"
            bind:value={levelBonusPercent}
          />
          <span class="field-hint">Acréscimo por nível acima do Nível 1 (Padrão: +5% por nível).</span>
        </div>

        <div class="field-item">
          <label for="cfg-ref-comm" class="field-lbl">COMISSÃO DE AFILIADOS 1º DEPÓSITO (%):</label>
          <input
            id="cfg-ref-comm"
            type="number"
            step="0.5"
            class="input-cat"
            bind:value={referralCommissionPercent}
          />
          <span class="field-hint">Comissão paga ao patrocinador no 1º depósito do indicado (Padrão: 5%).</span>
        </div>
      </div>
    </div>

    <button type="submit" class="btn-cat btn-save-config" disabled={isSaving}>
      {#if isSaving}
        <span>⚙️ GRAVANDO PARÂMETROS NO POSTGRESQL...</span>
      {:else}
        <span>SALVAR TODAS AS DIRETRIZES DA BANCA 💾</span>
      {/if}
    </button>
  </form>
</div>

<style>
  .config-tab {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .tab-top-bar {
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

  .config-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .config-section {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .section-emergency {
    border-color: var(--cat-red);
  }

  .section-heading {
    font-size: 0.9rem;
    border-bottom: 1px solid #282828;
    padding-bottom: 6px;
  }

  .toggle-control {
    display: flex;
    align-items: center;
    gap: 12px;
    cursor: pointer;
    background: #111;
    border: 1px solid #282828;
    padding: 12px 14px;
  }

  .toggle-control input[type="checkbox"] {
    width: 20px;
    height: 20px;
    cursor: pointer;
  }

  .toggle-text {
    display: flex;
    flex-direction: column;
  }

  .toggle-title {
    font-size: 0.85rem;
  }

  .toggle-desc {
    font-size: 0.65rem;
    color: #777;
  }

  .fields-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 14px;
  }

  .field-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .field-lbl {
    font-size: 0.65rem;
    color: #aaa;
    font-weight: 700;
  }

  .field-hint {
    font-size: 0.6rem;
    color: #666;
  }

  .btn-save-config {
    width: 100%;
    padding: 14px;
    font-size: 0.95rem;
  }

  .text-red { color: var(--cat-red); }
  .text-green { color: var(--cat-green); }
  .text-yellow { color: var(--cat-yellow); }
</style>
