<script>
  import { onMount } from 'svelte';
  import { api } from '../services/api.js';
  import { authState } from '../state/auth.svelte.js';
  import { toastState } from '../state/toast.svelte.js';
  import { Button, Card, Badge } from '@catbrasil/ui';

  const user = $derived(authState.user);

  let referrals = $state([]);
  let isLoading = $state(true);

  onMount(async () => {
    try {
      isLoading = true;
      const res = await api.get('/user/referrals');
      referrals = res.referrals || res || [];
    } catch (err) {
      console.warn('[Referrals] Erro ao carregar afiliados:', err);
    } finally {
      isLoading = false;
    }
  });

  const inviteUrl = $derived(() => {
    const base = window.location.origin;
    return `${base}/?ref=${user?.id || ''}`;
  });

  const activeCount = $derived(() => {
    return referrals.filter((r) => r.status === 'ativo').length;
  });

  const totalCommissions = $derived(() => {
    return referrals.reduce((acc, r) => {
      if (r.status === 'ativo') {
        return acc + (r.deposito * 0.05);
      }
      return acc;
    }, 0);
  });

  function copyToClipboard(text, label = 'Código') {
    navigator.clipboard.writeText(text);
    toastState.success(`${label} copiado para a área de transferência!`);
  }
</script>

<div class="equipe-tab">
  <!-- Banner de Afiliados -->
  <div class="affiliate-banner">
    <div class="hazard-stripes banner-hazard"></div>
    <div class="banner-content">
      <h2 class="banner-title">PROGRAMA DE INDICAÇÃO CATERPILLAR</h2>
      <p class="banner-desc">Convide operadores para a sua frente de mineração e receba <strong>5% de comissão instantânea</strong> no primeiro depósito PIX de cada indicado ativo.</p>
    </div>
  </div>

  <!-- Cards de Link e Código de Convite -->
  <div class="card-invite-tools">
    <div class="invite-box">
      <span class="invite-lbl">SEU CÓDIGO DE CONVITE:</span>
      <div class="copy-field font-mono">
        <span class="copy-value text-yellow">{user?.id || 'CB-0000'}</span>
        <button class="btn-copy" onclick={() => copyToClipboard(user?.id || '', 'Código')}>COPIAR</button>
      </div>
    </div>

    <div class="invite-box">
      <span class="invite-lbl">LINK DE CADASTRO DIRETO:</span>
      <div class="copy-field font-mono">
        <span class="copy-value text-truncate">{inviteUrl()}</span>
        <button class="btn-copy" onclick={() => copyToClipboard(inviteUrl(), 'Link')}>COPIAR LINK</button>
      </div>
    </div>
  </div>

  <!-- Métricas de Rede -->
  <div class="metrics-row font-mono">
    <div class="metric-card">
      <span class="metric-lbl">INDICADOS ATIVOS:</span>
      <strong class="metric-val text-green">{activeCount()} Operadores</strong>
    </div>
    <div class="metric-card">
      <span class="metric-lbl">TOTAL DE COMISSÕES:</span>
      <strong class="metric-val text-yellow">R$ {totalCommissions().toFixed(2)}</strong>
    </div>
  </div>

  <!-- Lista de Membros da Equipe -->
  <div class="team-section">
    <h3 class="team-title">OPERADORES INDICADOS ({referrals.length})</h3>

    {#if isLoading}
      <div class="loading-box">
        <span class="animate-spin text-2xl">⚙️</span>
        <p>Carregando sua rede de indicados...</p>
      </div>
    {:else if referrals.length === 0}
      <div class="empty-box">
        <p>Você ainda não indicou novos operadores. Compartilhe seu link exclusivo para começar a lucrar comissões de 5%!</p>
      </div>
    {:else}
      <div class="team-list">
        {#each referrals as ref}
          <div class="ref-card">
            <div class="ref-info">
              <span class="ref-name font-bold">@{ref.nome}</span>
              <span class="ref-date font-mono">{new Date(ref.createdAt).toLocaleDateString('pt-BR')}</span>
            </div>

            <div class="ref-status">
              {#if ref.status === 'ativo'}
                <span class="badge-cat badge-cat-green">✅ ATIVO (R$ {ref.deposito.toFixed(2)})</span>
              {:else}
                <span class="badge-cat badge-cat-yellow">⏳ PENDENTE</span>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .equipe-tab {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding-bottom: 90px;
  }

  .affiliate-banner {
    background: #181818;
    border: 2px solid var(--cat-black);
    box-shadow: 4px 4px 0px #000;
    overflow: hidden;
  }

  .banner-hazard {
    height: 6px;
    width: 100%;
  }

  .banner-content {
    padding: 16px;
  }

  .banner-title {
    color: var(--cat-yellow);
    font-size: 1.15rem;
  }

  .banner-desc {
    color: #999;
    font-size: 0.75rem;
    margin-top: 4px;
  }

  .card-invite-tools {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .invite-box {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .invite-lbl {
    font-size: 0.65rem;
    color: #888;
    font-weight: 700;
  }

  .copy-field {
    display: flex;
    align-items: center;
    background: #0d0d0d;
    border: 1.5px solid #333;
    padding: 4px 6px 4px 10px;
    gap: 8px;
  }

  .copy-value {
    flex: 1;
    font-size: 0.85rem;
  }

  .text-truncate {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #aaa;
  }

  .btn-copy {
    background: var(--cat-yellow);
    color: var(--cat-black);
    border: 1px solid #000;
    font-family: var(--font-heading);
    font-size: 0.75rem;
    font-weight: 700;
    padding: 6px 12px;
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-copy:active {
    transform: scale(0.96);
  }

  .metrics-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }

  .metric-card {
    background: #181818;
    border: 2px solid #282828;
    box-shadow: 3px 3px 0px #000;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .metric-lbl {
    font-size: 0.6rem;
    color: #777;
  }

  .metric-val {
    font-size: 1.1rem;
  }

  .team-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .team-title {
    color: #fff;
    font-size: 0.95rem;
  }

  .team-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .ref-card {
    background: #161616;
    border: 1.5px solid #242424;
    box-shadow: 2px 2px 0px #000;
    padding: 10px 14px;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .ref-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .ref-name {
    color: #fff;
    font-size: 0.85rem;
  }

  .ref-date {
    color: #666;
    font-size: 0.65rem;
  }

  .text-yellow { color: var(--cat-yellow); }
  .text-green { color: var(--cat-green); }

  .loading-box, .empty-box {
    background: #161616;
    border: 2px dashed #333;
    padding: 30px;
    text-align: center;
    color: #888;
    font-size: 0.85rem;
  }
</style>
