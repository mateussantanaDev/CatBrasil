<script>
  import { authState } from '../../state/auth.svelte.js';
  import { rouletteState } from '../../state/roulette.svelte.js';
  import { Modal, Button } from '@catbrasil/ui';

  let {
    isOpen = false,
    onclose,
    onopenroleta
  } = $props();

  const user = $derived(authState.user);

  const parts = $derived(() => [
    { id: 'motor', name: 'Motor C15 Heavy Duty', icon: '⚙️', count: user?.motorC15Count || 0 },
    { id: 'esteira', name: 'Esteira Reforçada CAT', icon: '⛓️', count: user?.esteiraReforcadaCount || 0 },
    { id: 'filtro', name: 'Filtro de Óleo Alta Vazão', icon: '🛢️', count: user?.filtroOleoCount || 0 },
    { id: 'turbo', name: 'Turbo Garrett T66', icon: '🌀', count: user?.turboGarrettCount || 0 }
  ]);

  const canAssemble = $derived(() => {
    return (
      (user?.motorC15Count || 0) >= 1 &&
      (user?.esteiraReforcadaCount || 0) >= 1 &&
      (user?.filtroOleoCount || 0) >= 1 &&
      (user?.turboGarrettCount || 0) >= 1
    );
  });

  async function handleAssemble() {
    if (!canAssemble) return;
    const ok = await rouletteState.assembleParts();
    if (ok && onclose) {
      onclose();
    }
  }
</script>

<Modal {isOpen} title="OFICINA DE MONTAGEM CATERPILLAR 🧩" {onclose} maxWidth="max-w-md">
  <div class="oficina-body">
    <!-- Banner Blueprint -->
    <div class="blueprint-card">
      <span class="blueprint-tag font-mono">BLUEPRINT: ESCAVADEIRA CAT 320</span>
      <h3 class="blueprint-title">MONTAGEM A CUSTO ZERO (R$ 0,00)</h3>
      <p class="blueprint-desc">
        Junte 1 unidade de cada uma das 4 peças na Roleta Premiada Diária para montar uma Escavadeira CAT 320 completa (Rendimento: R$ 110,00/dia).
      </p>
    </div>

    <!-- Grid de Peças -->
    <div class="parts-grid">
      {#each parts() as part}
        {@const hasPart = part.count >= 1}
        <div class="part-card {hasPart ? 'part-collected' : 'part-missing'}">
          <div class="part-icon">{part.icon}</div>
          <div class="part-info">
            <h4 class="part-name">{part.name}</h4>
            <span class="part-qty font-mono">ESTOQUE: {part.count} UNIDADE(S)</span>
          </div>
          <div class="part-badge">
            {#if hasPart}
              <span class="badge-ok">✅ PRONTO</span>
            {:else}
              <span class="badge-missing">⏳ FALTA</span>
            {/if}
          </div>
        </div>
      {/each}
    </div>

    <!-- Ação de Montagem -->
    <div class="action-box">
      {#if canAssemble}
        <button
          class="btn-cat btn-assemble-active"
          disabled={rouletteState.isLoading}
          onclick={handleAssemble}
        >
          {#if rouletteState.isLoading}
            <span>⚙️ MONTANDO MÁQUINA NA OFICINA...</span>
          {:else}
            <span>🎉 MONTAR CAT 320 AGORA (CUSTO R$ 0,00) 🚜</span>
          {/if}
        </button>
      {:else}
        <div class="missing-notice">
          <span>Gire a Roleta Diária para conquistar as peças que faltam!</span>
          <button class="btn-spin-shortcut" onclick={() => { if (onclose) onclose(); onopenroleta(); }}>
            🎰 IR PARA A ROLETA DIÁRIA
          </button>
        </div>
      {/if}
    </div>
  </div>
</Modal>

<style>
  .oficina-body {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .blueprint-card {
    background: #0f1c24;
    border: 2px dashed #0288D1;
    padding: 12px 14px;
  }

  .blueprint-tag {
    font-size: 0.6rem;
    color: #4FC3F7;
    font-weight: 700;
  }

  .blueprint-title {
    color: #fff;
    font-size: 1rem;
    margin-top: 2px;
  }

  .blueprint-desc {
    color: #90CAF9;
    font-size: 0.7rem;
    margin-top: 4px;
    line-height: 1.4;
  }

  .parts-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .part-card {
    background: #141414;
    border: 1.5px solid #282828;
    padding: 10px 12px;
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .part-collected {
    border-color: var(--cat-green);
    background: rgba(0, 200, 83, 0.05);
  }

  .part-missing {
    border-color: #333;
    opacity: 0.75;
  }

  .part-icon {
    font-size: 1.4rem;
  }

  .part-info {
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .part-name {
    color: #fff;
    font-size: 0.85rem;
  }

  .part-qty {
    font-size: 0.65rem;
    color: #888;
  }

  .badge-ok {
    color: var(--cat-green);
    font-size: 0.7rem;
    font-weight: 700;
  }

  .badge-missing {
    color: #666;
    font-size: 0.7rem;
    font-weight: 700;
  }

  .btn-assemble-active {
    width: 100%;
    padding: 14px;
    background: var(--cat-green);
    color: #000;
    font-size: 0.95rem;
  }

  .missing-notice {
    background: #141414;
    border: 1px solid #282828;
    padding: 12px;
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 0.75rem;
    color: #888;
  }

  .btn-spin-shortcut {
    background: #FF9100;
    color: #000;
    border: 1px solid #000;
    font-family: var(--font-heading);
    font-size: 0.8rem;
    font-weight: 700;
    padding: 8px;
    cursor: pointer;
  }
</style>
