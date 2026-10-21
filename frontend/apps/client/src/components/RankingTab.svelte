<script>
    import { onMount } from 'svelte';
    import { api } from '../services/api.js';


    let leaderboard = [];
    let userRank = null;
    let seasonEndsInHours = 48;
    let isLoading = true;

    async function loadRankingData() {
        isLoading = true;
        try {
            const token = localStorage.getItem('cb_token');
            const data = await api.getWeeklyRanking(token);
            if (data && data.leaderboard) {
                leaderboard = data.leaderboard;
                userRank = data.userRank;
                seasonEndsInHours = data.seasonEndsInHours || 48;
            }
        } catch (e) {
            console.warn("Falha ao carregar ranking:", e.message);
        } finally {
            isLoading = false;
        }
    }

    onMount(() => {
        loadRankingData();
    });
</script>

<div class="tab-content active" style="padding: 0 10px 40px 10px; width: 100%; box-sizing: border-box; max-width: 100%; overflow-x: hidden;">
    <!-- Título Padrão do Sistema (section-title com dot amarelo) -->
    <h2 class="section-title" style="margin-bottom: 10px; font-size: 1.1rem;">
        CORRIDA DE MINERAÇÃO <span class="dot"></span>
    </h2>

    <!-- Banner Principal Mobile-First Brutalista Caterpillar -->
    <div style="background: var(--cat-black); color: var(--cat-white); padding: 12px; border: var(--brutal-border); border-radius: 4px; box-shadow: var(--brutal-shadow); margin-bottom: 14px; width: 100%; box-sizing: border-box;">
        <div style="display: flex; flex-direction: column; gap: 8px;">
            <div style="display: flex; align-items: center; gap: 10px; width: 100%;">
                <div style="width: 40px; height: 40px; border-radius: 4px; background: var(--cat-yellow); border: 2px solid var(--cat-white); display: flex; align-items: center; justify-content: center; flex-shrink: 0; color: #000;">
                    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M6 9H4.5a2.5 2.5 0 0 1 0-5H6"/>
                        <path d="M18 9h1.5a2.5 2.5 0 0 0 0-5H18"/>
                        <path d="M4 22h16"/>
                        <path d="M10 14.66V17c0 .55-.45 1-1 1H7v4h10v-4h-2c-.55 0-1-.45-1-1v-2.34"/>
                        <path d="M18 4H6v7a6 6 0 0 0 12 0V4z"/>
                    </svg>
                </div>
                <div style="flex: 1; min-width: 0;">
                    <h3 style="font-size: 1.05rem; color: var(--cat-yellow); font-family:'Oswald'; margin: 0; line-height: 1.1; letter-spacing: 0.5px; text-transform: uppercase;">
                        CORRIDA SEMANAL DE EXTRAÇÃO
                    </h3>
                    <span style="font-size: 0.6rem; color: #bbb; text-transform: uppercase; font-weight: 700; display: block; margin-top: 2px; line-height: 1.2;">
                        Dispute posições no placar minerando rendimentos
                    </span>
                </div>
            </div>

            <div style="background: rgba(255, 178, 0, 0.12); border: 1px solid var(--cat-yellow); padding: 5px 8px; border-radius: 2px; display: flex; align-items: center; justify-content: space-between; width: 100%; box-sizing: border-box;">
                <span style="font-size: 0.58rem; color: #bbb; font-weight: 700; text-transform: uppercase; display: flex; align-items: center; gap: 4px;">
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--cat-yellow)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="12" cy="12" r="10"/>
                        <polyline points="12 6 12 12 16 14"/>
                    </svg>
                    TEMPORADA DE MINERAÇÃO (75 DIAS)
                </span>
                <strong class="mono-text highlight-yellow" style="font-size: 0.78rem; font-weight: 900;">
                    {seasonEndsInHours}H RESTANTES
                </strong>
            </div>
            <span style="font-size: 0.58rem; color: #94a3b8; line-height: 1.3; display: block;">
                ℹ️ A temporada de 75 dias renova exclusivamente o ranking e premiações do pódio. Os níveis conquistados no seu Plano de Carreira (1 a 12) são perpétuos e não resetam.
            </span>
        </div>
    </div>

    <!-- Posição Atual do Operador (Brutalist Card White Mobile-First) -->
    {#if userRank}
        <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; padding: 10px; margin-bottom: 16px; display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; box-sizing: border-box;">
            <div style="min-width: 0; flex: 1;">
                <span style="font-size: 0.55rem; color: #666; text-transform: uppercase; font-weight: 800; display: block;">Sua Posição Oficial</span>
                <strong style="font-family: 'Oswald', sans-serif; font-size: 0.95rem; color: var(--cat-black); display: block; line-height: 1.2;">
                    #{userRank.rank || '1º'} LUGAR · <span class="highlight-yellow" style="font-weight: 900;">{userRank.badge || 'MINERADOR LENDÁRIO'}</span>
                </strong>
            </div>
            <div style="text-align: right; flex-shrink: 0;">
                <span style="font-size: 0.55rem; color: #666; text-transform: uppercase; font-weight: 800; display: block;">Total Extraído</span>
                <strong class="mono-text highlight-green" style="font-size: 0.92rem; font-weight: 900;">
                    {(userRank.weeklyTonnage || 0).toLocaleString('pt-BR')} Ton
                </strong>
            </div>
        </div>
    {/if}

    {#if isLoading}
        <div style="background: #fff; border: 2px solid var(--cat-black); padding: 20px; text-align: center; font-family: monospace; font-size: 0.75rem; color: #666; box-shadow: 3px 3px 0px var(--cat-black);">
            Carregando Placar dos Mineradores...
        </div>
    {:else}
        {#if leaderboard.length === 0}
            <div style="background: #fff; border: 2px solid var(--cat-black); padding: 20px; text-align: center; font-family: monospace; font-size: 0.75rem; color: #666; box-shadow: 3px 3px 0px var(--cat-black); margin-top: 10px;">
                Nenhum operador pontuou ainda nesta temporada. Colete rendimentos na Garagem para liderar o Ranking!
            </div>
        {:else}
            <!-- PODIUM DOS 3 PRIMEIROS MINERADORES (EXTREMAMENTE DESTACADOS) -->
            {#if leaderboard.length >= 3}
            <div style="margin-bottom: 20px; width: 100%; box-sizing: border-box;">
                <h4 style="font-family: 'Oswald', sans-serif; font-size: 0.9rem; color: var(--cat-black); margin: 0 0 10px 0; text-transform: uppercase; display: flex; align-items: center; gap: 6px;">
                    PODIUM DOS CAMPEÕES DA SEMANA
                </h4>

                <div style="display: grid; grid-template-columns: 1fr 1.18fr 1fr; gap: 6px; align-items: flex-end; width: 100%; box-sizing: border-box;">
                    
                    <!-- 2º LUGAR (PRATA) -->
                    <div style="background: #fff; border: 3px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 6px; padding: 10px 4px; text-align: center; display: flex; flex-direction: column; justify-content: space-between; min-height: 135px; box-sizing: border-box; overflow: hidden;">
                        <div>
                            <div style="display: inline-flex; align-items: center; justify-content: center; background: #cbd5e1; border: 2px solid var(--cat-black); color: var(--cat-black); font-family: 'Oswald'; font-weight: 900; font-size: 0.85rem; width: 28px; height: 28px; border-radius: 50%; margin: 0 auto 4px auto;">
                                2
                            </div>
                            <strong style="font-family: 'Oswald'; font-size: 0.82rem; color: var(--cat-black); display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
                                {leaderboard[1].username}
                            </strong>
                        </div>
                        <div style="margin-top: 4px;">
                            <span class="mono-text highlight-green" style="font-size: 0.75rem; font-weight: 900; display: block;">
                                {leaderboard[1].weeklyTonnage.toLocaleString('pt-BR')} Ton
                            </span>
                            <span style="font-size: 0.55rem; background: #e2e8f0; border: 1px solid var(--cat-black); color: var(--cat-black); padding: 2px 3px; font-weight: 900; border-radius: 2px; text-transform: uppercase; margin-top: 3px; display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
                                {leaderboard[1].prize}
                            </span>
                        </div>
                    </div>

                    <!-- 1º LUGAR (OURO - CAMPEÃO DESTACADO) -->
                    <div style="background: var(--cat-black); border: 3.5px solid var(--cat-yellow); box-shadow: 4px 4px 0px var(--cat-yellow); border-radius: 6px; padding: 12px 4px; text-align: center; display: flex; flex-direction: column; justify-content: space-between; min-height: 160px; box-sizing: border-box; overflow: hidden; transform: translateY(-4px);">
                        <div>
                            <div style="display: inline-flex; align-items: center; justify-content: center; background: var(--cat-yellow); border: 2px solid #000; color: #000; font-family: 'Oswald'; font-weight: 900; font-size: 1.05rem; width: 34px; height: 34px; border-radius: 50%; margin: 0 auto 4px auto; box-shadow: 0 0 12px rgba(255,178,0,0.6);">
                                1
                            </div>
                            <strong style="font-family: 'Oswald'; font-size: 0.95rem; color: var(--cat-yellow); display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap; text-transform: uppercase; letter-spacing: 0.5px;">
                                {leaderboard[0].username}
                            </strong>
                            <span style="font-size: 0.52rem; color: #fff; background: rgba(255,178,0,0.25); border: 1px solid var(--cat-yellow); padding: 1px 3px; font-weight: 800; border-radius: 2px; display: inline-block; margin-top: 2px;">CAMPEÃO</span>
                        </div>
                        <div style="margin-top: 6px;">
                            <span class="mono-text highlight-green" style="font-size: 0.88rem; font-weight: 900; display: block;">
                                {leaderboard[0].weeklyTonnage.toLocaleString('pt-BR')} Ton
                            </span>
                            <span style="font-size: 0.6rem; background: var(--cat-yellow); color: #000; padding: 3px 4px; font-weight: 900; border-radius: 2px; text-transform: uppercase; margin-top: 3px; display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
                                {leaderboard[0].prize}
                            </span>
                        </div>
                    </div>

                    <!-- 3º LUGAR (BRONZE) -->
                    <div style="background: #fff; border: 3px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 6px; padding: 10px 4px; text-align: center; display: flex; flex-direction: column; justify-content: space-between; min-height: 130px; box-sizing: border-box; overflow: hidden;">
                        <div>
                            <div style="display: inline-flex; align-items: center; justify-content: center; background: #feefc3; border: 2px solid var(--cat-black); color: #b45309; font-family: 'Oswald'; font-weight: 900; font-size: 0.85rem; width: 28px; height: 28px; border-radius: 50%; margin: 0 auto 4px auto;">
                                3
                            </div>
                            <strong style="font-family: 'Oswald'; font-size: 0.82rem; color: var(--cat-black); display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
                                {leaderboard[2].username}
                            </strong>
                        </div>
                        <div style="margin-top: 4px;">
                            <span class="mono-text highlight-green" style="font-size: 0.75rem; font-weight: 900; display: block;">
                                {leaderboard[2].weeklyTonnage.toLocaleString('pt-BR')} Ton
                            </span>
                            <span style="font-size: 0.55rem; background: #feefc3; border: 1px solid var(--cat-black); color: var(--cat-black); padding: 2px 3px; font-weight: 900; border-radius: 2px; text-transform: uppercase; margin-top: 3px; display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
                                {leaderboard[2].prize}
                            </span>
                        </div>
                    </div>

                </div>
            </div>
        {/if}

        <!-- Tabela Estilo Card Lista Brutalista Mobile-First -->
        <h4 style="font-family: 'Oswald', sans-serif; font-size: 0.88rem; color: var(--cat-black); margin: 16px 0 8px 0; text-transform: uppercase;">
            CLASSIFICAÇÃO GERAL DOS MINERADORES
        </h4>

        <div style="display: flex; flex-direction: column; gap: 6px; width: 100%; box-sizing: border-box;">
            {#each leaderboard as item, index}
                <div style="background: {index === 0 ? '#FFF8E1' : '#fff'}; border: 2px solid var(--cat-black); border-radius: 4px; padding: 8px 10px; display: flex; align-items: center; justify-content: space-between; box-shadow: 2px 2px 0px var(--cat-black); gap: 6px; width: 100%; box-sizing: border-box;">
                    <div style="display: flex; align-items: center; gap: 8px; min-width: 0; flex: 1;">
                        <span style="font-family: 'Oswald', sans-serif; font-size: 0.92rem; font-weight: 900; color: {index === 0 ? 'var(--cat-black)' : '#555'}; width: 22px; flex-shrink: 0; text-align: center;">
                            #{index + 1}
                        </span>
                        <div style="min-width: 0; flex: 1;">
                            <strong style="font-family: 'Oswald', sans-serif; font-size: 0.82rem; color: var(--cat-black); display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">{item.username}</strong>
                            <span style="font-size: 0.58rem; color: #555; font-weight: 700; display: block; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">Prêmio: <strong style="color: var(--cat-black);">{item.prize}</strong></span>
                        </div>
                    </div>
                    <div style="text-align: right; flex-shrink: 0;">
                        <span class="mono-text highlight-green" style="font-size: 0.78rem; font-weight: 900;">
                            {item.weeklyTonnage.toLocaleString('pt-BR')} Ton
                        </span>
                    </div>
                </div>
            {/each}
        </div>
        {/if}
    {/if}
</div>
