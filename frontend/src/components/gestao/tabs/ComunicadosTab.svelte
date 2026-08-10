<script>
    export let events = [];
    export let newTitle = "";
    export let newCategory = "CAMPANHA DA GESTÃO";
    export let newContent = "";
    export let newImageUrl = "";
    export let newCtaLink = "";
    export let newCtaText = "ACESSAR AGORA";
    export let newShowInWelcome = false;

    export let onCreateEvent = () => {};
    export let onToggleEventActive = () => {};
    export let onDeleteEvent = () => {};
</script>

<!-- ABA 5: PUBLICAR COMUNICADOS & CAMPANHAS DE MÍDIA DESACOPLADA -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- Formulário Rich Media de Novo Comunicado -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <div style="border-bottom: 1px solid #e2e8f0; padding-bottom: 8px; margin-bottom: 14px; display: flex; justify-content: space-between; align-items: center;">
            <h3 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">PUBLICADOR RICH MEDIA DE COMUNICADOS & CAMPANHAS</h3>
            <span style="font-family: monospace; font-size: 0.65rem; color: #64748b;">SUPORTE A IMAGENS, LINKS EXTERNOS & BOTOES DE ACAO</span>
        </div>

        <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 16px;">
            
            <!-- Coluna 1: Campos do Formulário -->
            <div style="display: flex; flex-direction: column; gap: 10px;">
                <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 10px;">
                    <div>
                        <label for="evt-t-inp" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">TÍTULO DO COMUNICADO / ANÚNCIO:</label>
                        <input id="evt-t-inp" type="text" placeholder="Ex: Oferta Trator D8T +20% Rendimento..." bind:value={newTitle} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <div>
                        <label for="evt-c-sel" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">CATEGORIA BADGE:</label>
                        <select id="evt-c-sel" bind:value={newCategory} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;">
                            <option value="CAMPANHA DA GESTÃO">CAMPANHA DA GESTÃO</option>
                            <option value="BÔNUS DE REDE">BÔNUS DE REDE</option>
                            <option value="COMUNICADO OFICIAL">COMUNICADO OFICIAL</option>
                            <option value="OFERTA DE MÁQUINAS">OFERTA DE MÁQUINAS</option>
                            <option value="ALERTA DE SISTEMA">ALERTA DE SISTEMA</option>
                        </select>
                    </div>
                </div>

                <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 10px;">
                    <div>
                        <label for="evt-img-inp" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">URL DA IMAGEM ANEXA (CAPA DO ANÚNCIO):</label>
                        <input id="evt-img-inp" type="url" placeholder="https://exemplo.com/imagem.jpg" bind:value={newImageUrl} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <div>
                        <label for="evt-cta-lbl" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">TEXTO DO BOTÃO (CTA):</label>
                        <input id="evt-cta-lbl" type="text" placeholder="Ex: VER OFERTA NA LOJA" bind:value={newCtaText} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>
                </div>

                <div>
                    <label for="evt-cta-url" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">LINK DE AÇÃO / REDIRECIONAMENTO (CTA URL):</label>
                    <input id="evt-cta-url" type="url" placeholder="https://t.me/seu_canal ou https://catbrasil.app/loja" bind:value={newCtaLink} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>

                <div>
                    <label for="evt-b-txt" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 3px;">CONTEÚDO DA MENSAGEM / ANÚNCIO:</label>
                    <textarea id="evt-b-txt" placeholder="Escreva o texto completo do comunicado..." bind:value={newContent} style="width: 100%; height: 75px; padding: 8px; font-family: sans-serif; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;"></textarea>
                </div>

                <div style="display: flex; align-items: center; gap: 8px;">
                    <input id="chk-wlc-mdl" type="checkbox" bind:checked={newShowInWelcome} style="width: 16px; height: 16px;" />
                    <label for="chk-wlc-mdl" style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #0f172a; cursor: pointer;">EXIBIR TAMBÉM NO POPUP DE ENTRADA (BOAS-VINDAS PRIMEIRO ACESSO)</label>
                </div>

                <button on:click={onCreateEvent} style="width: 100%; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.75rem; font-weight: 700; text-transform: uppercase; padding: 12px; border: 1px solid #000; cursor: pointer; margin-top: 4px;">
                    PUBLICAR COMUNICADO DE MÍDIA INSTANTANEMENTE
                </button>
            </div>

            <!-- Coluna 2: Pré-visualização ao Vivo em Tempo Real -->
            <div style="background: #f8fafc; border: 1px solid #cbd5e1; padding: 12px; display: flex; flex-direction: column; gap: 8px;">
                <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">PRÉ-VISUALIZAÇÃO AO VIVO NO CLIENTE:</span>

                <div style="background: #fff; border: 2px solid #0f172a; padding: 12px; box-shadow: 3px 3px 0px #0f172a; text-align: center;">
                    <span style="background: #ffb200; color: #000; font-size: 0.55rem; padding: 2px 6px; border: 1px solid #000; font-weight: 800; text-transform: uppercase; font-family: monospace;">
                        {newCategory}
                    </span>
                    <h4 style="font-family: 'Oswald', sans-serif; font-size: 1rem; color: #0f172a; margin: 6px 0 4px 0;">{newTitle || 'TÍTULO DO COMUNICADO'}</h4>
                    
                    {#if newImageUrl}
                        <img src={newImageUrl} alt="Capa Anexa" style="width: 100%; height: 110px; object-fit: cover; border: 1px solid #000; margin-bottom: 6px;" />
                    {/if}

                    <p style="font-size: 0.68rem; color: #475569; line-height: 1.3; margin: 0 0 8px 0; text-align: left;">
                        {newContent || 'O texto completo do comunicado aparecerá aqui.'}
                    </p>

                    {#if newCtaLink}
                        <div style="background: #0f172a; color: #ffb200; font-family: monospace; font-size: 0.65rem; font-weight: 800; padding: 6px; border: 1px solid #000; text-transform: uppercase;">
                            {newCtaText || 'ACESSAR AGORA'}
                        </div>
                    {/if}
                </div>
            </div>

        </div>
    </div>

    <!-- Tabela & Galeria de Comunicados Publicados Ativos -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 12px 0;">COMUNICADOS E CAMPANHAS ATIVAS ({events.length})</h4>

        <div style="display: flex; flex-direction: column; gap: 8px;">
            {#each events as evt}
                <div style="border: 1px solid #e2e8f0; padding: 12px; background: #fff; display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap;">
                    
                    <div style="display: flex; align-items: center; gap: 12px; min-width: 0; flex: 1;">
                        {#if evt.imageUrl}
                            <img src={evt.imageUrl} alt="Thumbnail" style="width: 60px; height: 45px; object-fit: cover; border: 1px solid #000; flex-shrink: 0;" />
                        {:else}
                            <div style="width: 60px; height: 45px; background: #f1f5f9; border: 1px solid #cbd5e1; display: flex; align-items: center; justify-content: center; font-family: monospace; font-size: 0.55rem; color: #94a3b8; flex-shrink: 0;">TEXTO</div>
                        {/if}

                        <div style="min-width: 0;">
                            <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
                                <strong style="font-size: 0.8rem; color: #0f172a;">{evt.title}</strong>
                                <span style="font-family: monospace; font-size: 0.55rem; background: #f1f5f9; border: 1px solid #cbd5e1; padding: 1px 5px; font-weight: 700;">{evt.category}</span>
                            </div>
                            <p style="font-size: 0.68rem; color: #64748b; margin: 2px 0 0 0; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">{evt.content}</p>
                            <span style="font-family: monospace; font-size: 0.58rem; color: #94a3b8;">DATA: {evt.date} · VISUALIZAÇÕES: {evt.viewsCount || 0}</span>
                        </div>
                    </div>

                    <div style="display: flex; align-items: center; gap: 8px;">
                        <button 
                            on:click={() => onToggleEventActive(evt.id)}
                            style="padding: 6px 10px; font-size: 0.65rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: {evt.active ? '#047857' : '#b45309'}; color: #fff; cursor: pointer;"
                        >
                            {evt.active ? 'STATUS: ATIVO' : 'STATUS: PAUSADO'}
                        </button>

                        <button 
                            on:click={() => onDeleteEvent(evt.id)}
                            style="padding: 6px 10px; font-size: 0.65rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #b91c1c; color: #fff; cursor: pointer;"
                        >
                            EXCLUIR
                        </button>
                    </div>

                </div>
            {/each}
        </div>
    </div>

</div>
