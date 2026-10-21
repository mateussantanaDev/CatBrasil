<script>
    export let show = false;
    export let isSpinning = false;
    export let rouletteRotation = 0;
    export let roulettePrizeMessage = "";
    export let onClose = () => {};
    export let onSpin = () => {};
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 380px; width: 92%; box-sizing: border-box;">
            <div class="modal-header">
                <h3>ROLETA DE PRÊMIOS DIÁRIOS</h3>
                <button class="modal-close" on:click={onClose}>✕</button>
            </div>
            <div class="modal-body" style="text-align: center;">
                <p style="font-size: 0.72rem; color: #555; margin-bottom: 12px;">Gire a roleta uma vez por dia e ganhe bônus em dinheiro para sua frota!</p>

                <!-- Roleta Vetorial SVG Perfeitamente Alinhada -->
                <div style="position: relative; width: 170px; height: 170px; margin: 0 auto;">
                    <div style="position: absolute; top: -8px; left: 50%; transform: translateX(-50%); z-index: 10; font-size: 1.2rem; color: var(--cat-red); filter: drop-shadow(0px 2px 2px rgba(0,0,0,0.5));">▼</div>
                    
                    <svg 
                        viewBox="0 0 200 200" 
                        style="width: 100%; height: 100%; transform: rotate({rouletteRotation}deg); transition: transform 3.2s cubic-bezier(0.15, 0.85, 0.2, 1); filter: drop-shadow(2px 2px 0px #000);"
                    >
                        <!-- Borda Externa -->
                        <circle cx="100" cy="100" r="96" fill="#111" stroke="#ffb200" stroke-width="5" />
                        
                        <!-- Slice 1: R$ 10 -->
                        <path d="M100,100 L100,8 A92,92 0 0,1 179.67,54 L100,100 Z" fill="#ffb200" stroke="#111" stroke-width="2"/>
                        <g transform="rotate(30 100 100)">
                            <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#111" font-size="13" font-weight="900" font-family="Oswald">R$ 10</text>
                        </g>
                        
                        <!-- Slice 2: R$ 50 -->
                        <path d="M100,100 L179.67,54 A92,92 0 0,1 179.67,146 L100,100 Z" fill="#111" stroke="#ffb200" stroke-width="2"/>
                        <g transform="rotate(90 100 100)">
                            <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 50</text>
                        </g>
                        
                        <!-- Slice 3: R$ 100 -->
                        <path d="M100,100 L179.67,146 A92,92 0 0,1 100,192 L100,100 Z" fill="#2e7d32" stroke="#111" stroke-width="2"/>
                        <g transform="rotate(150 100 100)">
                            <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 100</text>
                        </g>
                        
                        <!-- Slice 4: R$ 20 -->
                        <path d="M100,100 L100,192 A92,92 0 0,1 20.33,146 L100,100 Z" fill="#d32f2f" stroke="#111" stroke-width="2"/>
                        <g transform="rotate(210 100 100)">
                            <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 20</text>
                        </g>
                        
                        <!-- Slice 5: R$ 500 -->
                        <path d="M100,100 L20.33,146 A92,92 0 0,1 20.33,54 L100,100 Z" fill="#ffb200" stroke="#111" stroke-width="2"/>
                        <g transform="rotate(270 100 100)">
                            <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#111" font-size="13" font-weight="900" font-family="Oswald">R$ 500</text>
                        </g>
                        
                        <!-- Slice 6: R$ 5 -->
                        <path d="M100,100 L20.33,54 A92,92 0 0,1 100,8 L100,100 Z" fill="#111" stroke="#ffb200" stroke-width="2"/>
                        <g transform="rotate(330 100 100)">
                            <text x="100" y="42" text-anchor="middle" dominant-baseline="middle" fill="#fff" font-size="13" font-weight="900" font-family="Oswald">R$ 5</text>
                        </g>
                        
                        <!-- Centro da Roleta -->
                        <circle cx="100" cy="100" r="18" fill="#ffb200" stroke="#111" stroke-width="3" />
                        <circle cx="100" cy="100" r="8" fill="#111" />
                    </svg>
                </div>

                {#if roulettePrizeMessage}
                    <div class="prize-banner" style="margin-top: 12px; font-weight: 800; color: var(--cat-green);">
                        {roulettePrizeMessage}
                    </div>
                {/if}

                <button class="btn btn-primary" disabled={isSpinning} on:click={onSpin} style="margin-top: 14px;">
                    {isSpinning ? 'GIRANDO...' : 'GIRAR ROLETA AGORA'}
                </button>
            </div>
        </div>
    </div>
{/if}
