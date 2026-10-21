<script>
    import { onMount } from 'svelte';
    import { api } from '../../services/api.js';

    export let show = false;
    export let onClose = () => {};

    let currentSlide = 0;
    let activeEvents = [];

    async function loadActiveAnnouncements() {
        try {
            const list = await api.getActiveAnnouncements();
            if (Array.isArray(list)) {
                activeEvents = list;
            }
        } catch (e) {
            console.warn("Erro ao buscar comunicados ativos:", e.message);
        }
    }

    $: if (show) {
        loadActiveAnnouncements();
    }

    onMount(() => {
        loadActiveAnnouncements();
    });

    function nextSlide() {
        if (activeEvents.length === 0 || currentSlide >= activeEvents.length - 1) {
            onClose();
        } else {
            currentSlide++;
        }
    }

    function prevSlide() {
        if (currentSlide > 0) currentSlide--;
    }
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 440px; width: 92%; padding: 16px; box-sizing: border-box;">
            <div class="modal-header" style="margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center;">
                <h3 style="font-family:'Oswald', sans-serif; font-size: 1.05rem; margin:0;">COMUNICADOS & EVENTOS DA GESTÃO</h3>
                <button class="modal-close" on:click={onClose}>✕</button>
            </div>

            {#if activeEvents.length === 0}
                <div style="padding: 20px; text-align: center; font-family: monospace; font-size: 0.75rem; color: #64748b;">
                    Nenhum comunicado ativo no momento.
                </div>
                <button class="btn btn-primary" on:click={onClose} style="margin-top: 10px; font-family: 'Oswald', sans-serif; font-size: 0.8rem;">
                    FECHAR COMUNICADOS
                </button>
            {:else}
                <div class="ann-carousel-wrapper" style="overflow: hidden; width: 100%; box-sizing: border-box;">
                    <div class="ann-carousel-track" style="display: flex; width: {activeEvents.length * 100}%; transform: translateX(-{currentSlide * (100 / activeEvents.length)}%); transition: transform 0.4s ease-in-out;">
                        
                        {#each activeEvents as evt}
                            <div style="width: {100 / activeEvents.length}%; box-sizing: border-box; padding: 8px 4px;">
                                <div style="background: #fff; border: 2px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); padding: 14px; border-radius: 4px; text-align: center;">
                                    <span style="background: var(--cat-yellow); color: var(--cat-black); font-size: 0.58rem; padding: 2px 6px; border: 1.5px solid var(--cat-black); font-weight:800; text-transform:uppercase; border-radius: 2px;">
                                        {evt.category || 'COMUNICADO DA GESTÃO'}
                                    </span>

                                    <h3 style="font-size: 1.15rem; margin-top: 8px; font-family:'Oswald'; color: var(--cat-black);">{evt.title}</h3>
                                    
                                    {#if evt.imageUrl}
                                        <img src={evt.imageUrl} alt="Imagem do Anúncio" style="width: 100%; max-height: 140px; object-fit: cover; border: 1.5px solid var(--cat-black); margin-top: 8px; border-radius: 2px;" />
                                    {/if}

                                    <p style="font-size: 0.75rem; color: #444; line-height: 1.4; margin-top: 8px; text-align: left;">{evt.content}</p>

                                    {#if evt.ctaLink}
                                        <a href={evt.ctaLink} target="_blank" rel="noopener noreferrer" style="display: block; margin-top: 10px; background: var(--cat-black); color: var(--cat-yellow); text-decoration: none; padding: 10px; border: 1.5px solid var(--cat-black); font-family: 'Oswald', sans-serif; font-size: 0.8rem; font-weight: 800; border-radius: 2px; text-transform: uppercase;">
                                            {evt.ctaText || 'ACESSAR AGORA'}
                                        </a>
                                    {/if}
                                </div>
                            </div>
                        {/each}

                    </div>
                </div>

                <!-- Rodapé Carrossel com Indicadores -->
                <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 12px; border-top: 1.5px dashed var(--cat-black); padding-top: 10px; width: 100%; box-sizing: border-box;">
                    <button class="btn-action-small" disabled={currentSlide === 0} on:click={prevSlide} style="margin: 0; width: 90px; flex-shrink: 0; font-family: 'Oswald', sans-serif;">
                        Voltar
                    </button>
                    
                    <div style="display: flex; gap: 8px; align-items: center; justify-content: center; flex-grow: 1;">
                        {#each activeEvents as _, idx}
                            <span style="width: 10px; height: 10px; border-radius: 50%; border: 1.5px solid #000; background: {currentSlide === idx ? 'var(--cat-yellow)' : '#ccc'}; transition: background 0.2s;"></span>
                        {/each}
                    </div>

                    <button class="btn-action-small" on:click={nextSlide} style="margin: 0; width: 95px; flex-shrink: 0; font-family: 'Oswald', sans-serif;">
                        {currentSlide === activeEvents.length - 1 ? 'Concluir' : 'Avançar'}
                    </button>
                </div>
            {/if}

        </div>
    </div>
{/if}
