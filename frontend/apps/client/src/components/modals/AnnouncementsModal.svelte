<script>
    export let show = false;
    export let user = {};
    export let isSpinning = false;
    export let rouletteRotation = 0;
    export let roulettePrizeMessage = "";
    
    export let onCloseModal = () => {};
    export let onSpinRoulette = () => {};

    let currentSlide = 0;

    function nextSlide() {
        if (currentSlide < 3) {
            currentSlide++;
        } else {
            onCloseModal();
        }
    }

    function prevSlide() {
        if (currentSlide > 0) currentSlide--;
    }
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 420px; width: 92%; padding: 14px; box-sizing: border-box; height: auto; min-height: auto;">
            <div class="modal-header" style="margin-bottom: 8px; display: flex; justify-content: space-between; align-items: center; padding: 10px 12px;">
                <h3 style="font-family:'Oswald', sans-serif; font-size:1rem; margin:0;">BOAS-VINDAS & GIRO DIÁRIO</h3>
                <button class="modal-close" on:click={onCloseModal} title="Fechar modal">✕</button>
            </div>
            
            <div class="ann-carousel-wrapper" style="overflow: hidden; width: 100%; box-sizing: border-box; height: auto;">
                <div class="ann-carousel-track" style="display: flex; width: 400%; transform: translateX(-{currentSlide * 25}%); transition: transform 0.4s ease-in-out; align-items: center;">
                    
                    <!-- Slide 1: Bem-Vindo -->
                    <div class="ann-carousel-slide" style="width: 25%; box-sizing: border-box; padding: 6px 4px; text-align: center; min-height: auto;">
                        <h3 style="font-size: 1.15rem; font-family:'Oswald'; color: var(--cat-black); margin-bottom: 8px;">BEM-VINDO AO TURNO!</h3>
                        <div style="background: #fafafa; border: 1.5px solid var(--cat-black); padding: 10px; border-radius: 4px; font-size: 0.78rem; color: #333; line-height: 1.4;">
                            <p style="margin: 0;">Operador, novas diretrizes e oportunidades foram lançadas na plataforma CatBrasil.</p>
                            <p style="margin-top: 6px; font-weight: 800; color: var(--cat-green);">Avance os slides para girar sua Roleta Diária!</p>
                        </div>
                    </div>

                    <!-- Slide 2: Novidades na Loja -->
                    <div class="ann-carousel-slide" style="width: 25%; box-sizing: border-box; padding: 6px 4px; min-height: auto;">
                        <div style="text-align: center; margin-bottom: 6px;">
                            <h3 style="font-size: 1.15rem; font-family:'Oswald'; color: var(--cat-black); margin: 0;">NOVIDADES NA LOJA</h3>
                        </div>
                        <div class="store-card" style="margin-bottom: 0; box-shadow: none; border-color: var(--cat-black); background: #fff;">
                            <div class="store-card-img" style="background-image:url('https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=300&q=80'); width: 75px;"></div>
                            <div class="store-card-body" style="padding: 6px 8px;">
                                <h4 class="store-card-title" style="font-size: 0.82rem;">Trator D8T</h4>
                                <div class="data-row" style="font-size: 0.62rem; margin: 2px 0 0; padding: 0;"><span>Custo:</span><strong>R$ 1.500,00</strong></div>
                                <div class="data-row" style="font-size: 0.62rem; margin: 0; padding: 0;"><span>Retorno:</span><strong class="highlight-green">R$ 75,00/dia</strong></div>
                            </div>
                        </div>
                    </div>

                    <!-- Slide 3: Eventos -->
                    <div class="ann-carousel-slide" style="width: 25%; box-sizing: border-box; padding: 6px 4px; text-align: center; min-height: auto;">
                        <h3 style="font-size: 1.15rem; font-family:'Oswald'; color: var(--cat-black); margin-bottom: 8px;">EVENTOS ATIVOS</h3>
                        <div style="background: #FFF8E1; border: 1.5px solid var(--cat-yellow); padding: 8px; font-size: 0.7rem; color: #333; border-radius: 4px; text-align: left; margin-bottom: 6px;">
                            <strong>Campanha de Verão:</strong> +20% de retorno acumulado no saldo para coletas diárias.
                        </div>
                        <div style="background: #E8F5E9; border: 1.5px solid #2e7d32; padding: 8px; font-size: 0.7rem; color: #1b5e20; border-radius: 4px; text-align: left;">
                            <strong>Mutirão de Rede:</strong> Convide 3 operadores ativos e resgate R$ 500,00 adicionais.
                        </div>
                    </div>

                    <!-- Slide 4: Roleta Diária Premiada -->
                    <div class="ann-carousel-slide" style="width: 25%; box-sizing: border-box; padding: 6px 4px; text-align: center; min-height: auto;">
                        <h3 style="font-size: 1.1rem; font-family:'Oswald'; color: var(--cat-black); margin-bottom: 6px;">GIRO DIÁRIO PREMIADO</h3>
                        
                        {#if user.alreadySpunToday}
                            <div style="background: #E8F5E9; border: 2px solid #2e7d32; padding: 10px; border-radius: 4px; margin-top: 6px;">
                                <strong style="color: #2e7d32; font-size: 0.82rem; display: block;">GIRO DO DIA UTILIZADO</strong>
                                <p style="font-size: 0.65rem; color: #555; margin-top: 2px;">Volte amanhã no seu próximo login para um novo giro.</p>
                            </div>
                        {:else}
                            <div style="position: relative; width: 155px; height: 155px; margin: 0 auto;">
                                <div style="position: absolute; top: -8px; left: 50%; transform: translateX(-50%); z-index: 10; font-size: 1.1rem; color: var(--cat-red); filter: drop-shadow(0px 2px 2px rgba(0,0,0,0.5));">▼</div>
                                
                                <svg 
                                    viewBox="0 0 200 200" 
                                    style="width: 100%; height: 100%; transform: rotate({rouletteRotation}deg); transition: transform 3.2s cubic-bezier(0.15, 0.85, 0.2, 1); filter: drop-shadow(2px 2px 0px #000);"
                                >
                                    <circle cx="100" cy="100" r="96" fill="#111" stroke="#ffb200" stroke-width="5" />
                                    
                                    <path d="M100,100 L100,8 A92,92 0 0,1 179.67,54 L100,100 Z" fill="#ffb200" stroke="#111" stroke-width="2"/>
                                    <g transform="rotate(30 100 100)">
                                        <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#111" font-size="13" font-weight="900" font-family="Oswald">R$ 10</text>
                                    </g>
                                    
                                    <path d="M100,100 L179.67,54 A92,92 0 0,1 179.67,146 L100,100 Z" fill="#111" stroke="#ffb200" stroke-width="2"/>
                                    <g transform="rotate(90 100 100)">
                                        <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 50</text>
                                    </g>
                                    
                                    <path d="M100,100 L179.67,146 A92,92 0 0,1 100,192 L100,100 Z" fill="#2e7d32" stroke="#111" stroke-width="2"/>
                                    <g transform="rotate(150 100 100)">
                                        <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 100</text>
                                    </g>
                                    
                                    <path d="M100,100 L100,192 A92,92 0 0,1 20.33,146 L100,100 Z" fill="#d32f2f" stroke="#111" stroke-width="2"/>
                                    <g transform="rotate(210 100 100)">
                                        <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 20</text>
                                    </g>
                                    
                                    <path d="M100,100 L20.33,146 A92,92 0 0,1 20.33,54 L100,100 Z" fill="#ffb200" stroke="#111" stroke-width="2"/>
                                    <g transform="rotate(270 100 100)">
                                        <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#111" font-size="13" font-weight="900" font-family="Oswald">R$ 500</text>
                                    </g>
                                    
                                    <path d="M100,100 L20.33,54 A92,92 0 0,1 100,8 L100,100 Z" fill="#111" stroke="#ffb200" stroke-width="2"/>
                                    <g transform="rotate(330 100 100)">
                                        <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 5</text>
                                    </g>
                                    
                                    <circle cx="100" cy="100" r="18" fill="#ffb200" stroke="#111" stroke-width="3" />
                                    <circle cx="100" cy="100" r="8" fill="#111" />
                                </svg>
                            </div>

                            {#if roulettePrizeMessage}
                                <div style="font-size: 0.75rem; font-weight: 800; color: var(--cat-green); margin-top: 4px;">
                                    {roulettePrizeMessage}
                                </div>
                            {/if}

                            <button class="btn btn-primary" disabled={isSpinning} on:click={onSpinRoulette} style="margin-top: 4px; padding: 7px; font-size: 0.72rem; width: 100%;">
                                {isSpinning ? 'GIRANDO...' : 'GIRAR ROLETA AGORA'}
                            </button>
                        {/if}
                    </div>

                </div>
            </div>

            <!-- Rodapé com Navegação Separada dos Pontos -->
            <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 8px; border-top: 1.5px dashed var(--cat-black); padding-top: 8px; width: 100%; box-sizing: border-box;">
                <button class="btn-action-small" disabled={currentSlide === 0} on:click={prevSlide} style="margin: 0; width: 90px; flex-shrink: 0; font-family: 'Oswald', sans-serif;">
                    Voltar
                </button>
                
                <div style="display: flex; gap: 8px; align-items: center; justify-content: center; flex-grow: 1;">
                    <span style="width: 9px; height: 9px; border-radius: 50%; border: 1.5px solid #000; background: {currentSlide === 0 ? 'var(--cat-yellow)' : '#ccc'}; transition: background 0.2s;"></span>
                    <span style="width: 9px; height: 9px; border-radius: 50%; border: 1.5px solid #000; background: {currentSlide === 1 ? 'var(--cat-yellow)' : '#ccc'}; transition: background 0.2s;"></span>
                    <span style="width: 9px; height: 9px; border-radius: 50%; border: 1.5px solid #000; background: {currentSlide === 2 ? 'var(--cat-yellow)' : '#ccc'}; transition: background 0.2s;"></span>
                    <span style="width: 9px; height: 9px; border-radius: 50%; border: 1.5px solid #000; background: {currentSlide === 3 ? 'var(--cat-yellow)' : '#ccc'}; transition: background 0.2s;"></span>
                </div>

                <button class="btn-action-small" on:click={nextSlide} style="margin: 0; width: 95px; flex-shrink: 0; font-family: 'Oswald', sans-serif;">
                    {currentSlide === 3 ? 'Concluir' : 'Avançar'}
                </button>
            </div>
        </div>
    </div>
{/if}
