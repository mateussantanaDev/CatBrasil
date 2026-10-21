<script>
  import { onMount } from 'svelte';
  import { adminDashboardState } from '../state/admin.svelte.js';
  import { toastState } from '../state/toast.svelte.js';
  import { Button, Card, Badge } from '@catbrasil/ui';

  const announcements = $derived(adminDashboardState.announcements);

  let title = $state('');
  let category = $state('OFICIAL');
  let content = $state('');
  let imageUrl = $state('');
  let badgeColor = $state('#FFB200');
  let ctaText = $state('');
  let ctaLink = $state('');
  let isPopup = $state(false);

  onMount(() => {
    adminDashboardState.loadAnnouncements();
  });

  async function handlePublish(e) {
    e.preventDefault();
    if (!title.trim() || !content.trim()) {
      toastState.warning('Preencha o título e o conteúdo do comunicado.');
      return;
    }

    const payload = {
      title: title.trim(),
      category,
      content: content.trim(),
      imageUrl: imageUrl.trim() || null,
      badgeColor,
      ctaText: ctaText.trim() || null,
      ctaLink: ctaLink.trim() || null,
      isPopup,
    };

    const ok = await adminDashboardState.createAnnouncement(payload);
    if (ok) {
      title = '';
      content = '';
      imageUrl = '';
      ctaText = '';
      ctaLink = '';
      isPopup = false;
    }
  }

  async function handleDelete(id) {
    if (confirm('Deseja excluir este comunicado do mural dos operadores?')) {
      await adminDashboardState.deleteAnnouncement(id);
    }
  }
</script>

<div class="comunicados-tab">
  <div class="tab-top-bar">
    <div>
      <span class="tab-subtitle font-mono">BROADCAST EM TEMPO REAL & MURAL</span>
      <h2 class="tab-title">MURAL DE COMUNICADOS & AVISOS 📢</h2>
    </div>
    <button class="btn-refresh font-mono" onclick={() => adminDashboardState.loadAnnouncements()}>
      🔄 ATUALIZAR
    </button>
  </div>

  <div class="announcements-grid">
    <!-- Formulário de Criação -->
    <div class="create-card">
      <h3 class="card-section-title">PUBLICAR NOVO COMUNICADO OFICIAL</h3>

      <form onsubmit={handlePublish} class="create-form font-mono">
        <div class="form-group">
          <label for="ann-title" class="form-label">TÍTULO DO COMUNICADO</label>
          <input
            id="ann-title"
            type="text"
            class="input-cat"
            placeholder="ex: 🚜 NOVA FROTA CAT 336 DISPONÍVEL!"
            bind:value={title}
            required
          />
        </div>

        <div class="form-row">
          <div class="form-group">
            <label for="ann-cat" class="form-label">CATEGORIA</label>
            <select id="ann-cat" class="input-cat" bind:value={category}>
              <option value="OFICIAL">OFICIAL</option>
              <option value="PROMOÇÃO">PROMOÇÃO</option>
              <option value="NOVIDADE">NOVIDADE</option>
              <option value="MANUTENÇÃO">MANUTENÇÃO</option>
            </select>
          </div>

          <div class="form-group">
            <label for="ann-color" class="form-label">COR DA TAG</label>
            <input id="ann-color" type="color" class="color-picker" bind:value={badgeColor} />
          </div>
        </div>

        <div class="form-group">
          <label for="ann-content" class="form-label">MENSAGEM / CONTEÚDO DO AVISO</label>
          <textarea
            id="ann-content"
            class="input-cat textarea-content"
            rows="4"
            placeholder="Escreva o comunicado detalhado para os operadores..."
            bind:value={content}
            required
          ></textarea>
        </div>

        <div class="form-group">
          <label for="ann-img" class="form-label">URL DA IMAGEM DE BANNER (OPCIONAL)</label>
          <input
            id="ann-img"
            type="url"
            class="input-cat"
            placeholder="https://..."
            bind:value={imageUrl}
          />
        </div>

        <div class="form-row">
          <div class="form-group">
            <label for="ann-cta-txt" class="form-label">TEXTO DO BOTÃO CTA (OPCIONAL)</label>
            <input
              id="ann-cta-txt"
              type="text"
              class="input-cat"
              placeholder="ex: ABRIR LOJA"
              bind:value={ctaText}
            />
          </div>

          <div class="form-group">
            <label for="ann-cta-lnk" class="form-label">LINK DO BOTÃO CTA</label>
            <input
              id="ann-cta-lnk"
              type="text"
              class="input-cat"
              placeholder="ex: #loja"
              bind:value={ctaLink}
            />
          </div>
        </div>

        <label class="checkbox-label">
          <input type="checkbox" bind:checked={isPopup} />
          <span>Exibir como Janela Modal Obrigatória no Login do Cliente</span>
        </label>

        <button type="submit" class="btn-cat btn-publish" disabled={adminDashboardState.isLoading}>
          DISPARAR COMUNICADO 📢
        </button>
      </form>
    </div>

    <!-- Lista de Comunicados Ativos -->
    <div class="list-card">
      <h3 class="card-section-title">COMUNICADOS PUBLICADOS ({announcements.length})</h3>

      {#if announcements.length === 0}
        <div class="empty-box">
          <p>Nenhum comunicado publicado no momento.</p>
        </div>
      {:else}
        <div class="announcements-stream">
          {#each announcements as ann}
            <div class="ann-item">
              <div class="ann-item-top">
                <span class="badge-cat" style="background: {ann.badgeColor || '#FFB200'}; color: #000;">
                  {ann.category}
                </span>
                <button class="btn-del" onclick={() => handleDelete(ann.id)}>✕</button>
              </div>

              <h4 class="ann-item-title">{ann.title}</h4>
              <p class="ann-item-desc">{ann.content}</p>

              <div class="ann-item-footer font-mono">
                <span>Visualizações: {ann.viewsCount || 0}</span>
                <span>{new Date(ann.createdAt).toLocaleDateString('pt-BR')}</span>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .comunicados-tab {
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

  .btn-refresh {
    background: #222;
    color: #bbb;
    border: 1px solid #444;
    padding: 8px 12px;
    font-size: 0.75rem;
    font-weight: 700;
    cursor: pointer;
  }

  .announcements-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .create-card, .list-card {
    background: #161616;
    border: 2px solid #282828;
    box-shadow: 4px 4px 0px #000;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .card-section-title {
    color: var(--cat-yellow);
    font-size: 0.95rem;
    border-bottom: 1px solid #282828;
    padding-bottom: 8px;
  }

  .create-form {
    display: flex;
    flex-direction: column;
    gap: 10px;
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

  .textarea-content {
    resize: vertical;
  }

  .color-picker {
    width: 100%;
    height: 36px;
    background: transparent;
    border: 1px solid #444;
    cursor: pointer;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #bbb;
    font-size: 0.7rem;
    cursor: pointer;
    margin: 4px 0;
  }

  .btn-publish {
    width: 100%;
    padding: 12px;
  }

  .announcements-stream {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .ann-item {
    background: #111;
    border: 1px solid #282828;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .ann-item-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .ann-item-title {
    color: #fff;
    font-size: 0.9rem;
  }

  .ann-item-desc {
    color: #aaa;
    font-size: 0.75rem;
    line-height: 1.4;
  }

  .ann-item-footer {
    display: flex;
    justify-content: space-between;
    font-size: 0.65rem;
    color: #666;
    border-top: 1px dashed #242424;
    padding-top: 6px;
    margin-top: 2px;
  }

  .btn-del {
    background: #333;
    color: #ff5252;
    border: 1px solid #444;
    padding: 2px 6px;
    cursor: pointer;
    font-weight: 700;
  }

  .empty-box {
    background: #111;
    border: 1px dashed #333;
    padding: 30px;
    text-align: center;
    color: #777;
    font-size: 0.8rem;
  }

  @media (max-width: 900px) {
    .announcements-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
