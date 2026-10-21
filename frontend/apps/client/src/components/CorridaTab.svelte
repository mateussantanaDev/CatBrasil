<script>
  import { onMount } from 'svelte';
  import { api } from '../services/api.js';
  import { authState } from '../state/auth.svelte.js';
  import { Card, Badge } from '@catbrasil/ui';

  const user = $derived(authState.user);

  let leaderboard = $state([]);
  let userRank = $state(null);
  let seasonEndsInHours = $state(48);
  let isLoading = $state(true);

  onMount(async () => {
    try {
      isLoading = true;
      const res = await api.get('/ranking/weekly');
      leaderboard = res.leaderboard || [];
      userRank = res.userRank;
      seasonEndsInHours = res.seasonEndsInHours || 48;
    } catch (err) {
      console.warn('[Ranking] Erro ao carregar placar:', err);
    } finally {
      isLoading = false;
    }
  });

  const prizesList = [
    { pos: '1º LUGAR', prize: 'Escavadeira CAT 320 TURBO (7 Dias de Rendimento 100% Grátis)', icon: '👑' },
    { pos: '2º LUGAR', prize: 'Pá Carregadeira CAT 966M (7 Dias de Rendimento Grátis)', icon: '🥇' },
    { pos: '3º LUGAR', prize: 'Motoniveladora CAT 140M (7 Dias de Rendimento Grátis)', icon: '🥈' },
    { pos: '4º ao 10º', prize: 'Bônus Especial de XP e Selo VIP de Prestígio', icon: '⭐' }
  ];
</script>

<div class="corrida-tab">
  <!-- Banner da Corrida -->
  <div class="race-banner">
    <div class="hazard-stripes banner-hazard"></div>
    <div class="race-banner-content">
      <div class="race-badge-top">TEMPORADA SEMANAL DE MINERAÇÃO</div>
      <h2 class="race-title">CORRIDA DE MINERAÇÃO CATERPILLAR ⛏️</h2>
      <p class="race-desc">Colete rendimentos nas suas frotas para minerar toneladas e disputar os prêmios semanais da CatBrasil.</p>

      <div class="season-timer font-mono">
        <span>TEMPO RESTANTE NA TEMPORADA:</span>
        <strong class="text-yellow">~{seasonEndsInHours} HORAS</strong>
      </div>
    </div>
  </div>

  <!-- Posição Atual do Operador -->
  <div class="card-user-rank">
    <div class="user-rank-info">
      <span class="user-rank-lbl">SUA CLASSIFICAÇÃO ATUAL:</span>
      <div class="user-rank-digits font-mono">
        <span class="rank-pos">#{userRank?.rank || 99}</span>
        <span class="rank-name">{user?.username || 'Operador'}</span>
      </div>
    </div>
    <div class="user-tonnage font-mono">
      <span class="ton-lbl">TONELADAS MINERADAS:</span>
      <strong class="ton-val text-yellow">{(userRank?.weeklyTonnage || 0).toLocaleString('pt-BR')} T</strong>
    </div>
  </div>

  <!-- Showcase de Premiações -->
  <div class="card-prizes">
    <h4 class="prizes-title">PREMIAÇÕES OFICIAIS DA TEMPORADA</h4>
    <div class="prizes-grid">
      {#each prizesList as p}
        <div class="prize-item">
          <span class="prize-icon">{p.icon}</span>
          <div class="prize-text">
            <strong class="prize-pos">{p.pos}</strong>
            <span class="prize-desc">{p.prize}</span>
          </div>
        </div>
      {/each}
    </div>
  </div>

  <!-- Placar Oficial Top 10 -->
  <div class="leaderboard-section">
    <h3 class="leaderboard-title">TOP 10 MAIORES MINERADORES DA SEMANA</h3>

    {#if isLoading}
      <div class="loading-box">
        <span class="animate-spin text-2xl">⚙️</span>
        <p>Calculando tonelagens extraídas...</p>
      </div>
    {:else if leaderboard.length === 0}
      <div class="empty-box">
        <p>A corrida semanal acabou de iniciar! Seja o primeiro a coletar rendimentos para liderar o ranking.</p>
      </div>
    {:else}
      <div class="leaderboard-list">
        {#each leaderboard as row, i}
          <div class="leader-card rank-{i + 1}">
            <div class="leader-rank-box font-mono">
              {#if i === 0}👑
              {:else if i === 1}🥇
              {:else if i === 2}🥈
              {:else}#{i + 1}{/if}
            </div>

            <div class="leader-info">
              <div class="leader-top">
                <span class="leader-username">{row.username}</span>
                <span class="leader-badge">{row.badge}</span>
              </div>
              <span class="leader-prize">🏆 {row.prize}</span>
            </div>

            <div class="leader-tonnage font-mono">
              <span class="ton-lbl">MINERADO:</span>
              <strong class="ton-num">{row.weeklyTonnage.toLocaleString('pt-BR')} T</strong>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .corrida-tab {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding-bottom: 90px;
  }

  .race-banner {
    background: #181818;
    border: 2px solid var(--cat-black);
    box-shadow: 4px 4px 0px #000;
    overflow: hidden;
  }

  .banner-hazard {
    height: 6px;
    width: 100%;
  }

  .race-banner-content {
    padding: 16px;
  }

  .race-badge-top {
    font-size: 0.65rem;
    font-weight: 800;
    color: var(--cat-yellow);
    letter-spacing: 1px;
  }

  .race-title {
    color: #fff;
    font-size: 1.15rem;
    margin-top: 4px;
  }

  .race-desc {
    color: #999;
    font-size: 0.75rem;
    margin-top: 4px;
  }

  .season-timer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: #0f0f0f;
    border: 1px solid #282828;
    padding: 8px 12px;
    font-size: 0.75rem;
    margin-top: 12px;
  }

  .card-user-rank {
    background: #1c1c1c;
    border: 2px solid var(--cat-yellow);
    box-shadow: 4px 4px 0px #000;
    padding: 14px 16px;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .user-rank-lbl, .ton-lbl {
    display: block;
    font-size: 0.6rem;
    color: #888;
  }

  .user-rank-digits {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .rank-pos {
    font-size: 1.4rem;
    color: var(--cat-yellow);
    font-weight: 800;
  }

  .rank-name {
    font-size: 0.9rem;
    color: #fff;
  }

  .user-tonnage {
    text-align: right;
  }

  .ton-val {
    font-size: 1.25rem;
  }

  .card-prizes {
    background: #141414;
    border: 2px solid #282828;
    box-shadow: 3px 3px 0px #000;
    padding: 14px 16px;
  }

  .prizes-title {
    color: var(--cat-yellow);
    font-size: 0.85rem;
    margin-bottom: 10px;
  }

  .prizes-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .prize-item {
    display: flex;
    align-items: center;
    gap: 10px;
    background: #1c1c1c;
    padding: 8px 10px;
    border: 1px solid #282828;
  }

  .prize-icon {
    font-size: 1.2rem;
  }

  .prize-text {
    display: flex;
    flex-direction: column;
  }

  .prize-pos {
    font-size: 0.75rem;
    color: var(--cat-yellow);
  }

  .prize-desc {
    font-size: 0.7rem;
    color: #aaa;
  }

  .leaderboard-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .leaderboard-title {
    color: #fff;
    font-size: 0.95rem;
  }

  .leaderboard-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .leader-card {
    background: #161616;
    border: 2px solid #242424;
    box-shadow: 3px 3px 0px #000;
    padding: 10px 14px;
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .rank-1 {
    border-color: var(--cat-yellow);
    background: #1c1808;
  }

  .rank-2 {
    border-color: #A0A0A0;
  }

  .rank-3 {
    border-color: #CD7F32;
  }

  .leader-rank-box {
    font-size: 1.2rem;
    font-weight: 800;
    color: var(--cat-yellow);
    min-width: 32px;
    text-align: center;
  }

  .leader-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .leader-top {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .leader-username {
    font-weight: 700;
    font-size: 0.85rem;
    color: #fff;
  }

  .leader-badge {
    background: #252525;
    color: var(--cat-yellow);
    font-size: 0.55rem;
    padding: 1px 5px;
    border: 1px solid #333;
    font-weight: 700;
  }

  .leader-prize {
    font-size: 0.65rem;
    color: #888;
  }

  .leader-tonnage {
    text-align: right;
  }

  .ton-num {
    font-size: 0.95rem;
    color: var(--cat-yellow);
  }

  .text-yellow { color: var(--cat-yellow); }

  .loading-box, .empty-box {
    background: #161616;
    border: 2px dashed #333;
    padding: 30px;
    text-align: center;
    color: #888;
  }
</style>
