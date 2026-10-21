<script>
    export let show = false;
    export let leaderboard = [];
    export let userRank = null;
    export let seasonEndsInHours = 48;
    export let onClose = () => {};

    const fallbackLeaderboard = [
        { rank: 1, username: "mateus", weeklyTonnage: 8450, prize: "CAT 320 TURBO (7 Dias)", badge: "👑 LENDÁRIO" },
        { rank: 2, username: "carlos_operador", weeklyTonnage: 6200, prize: "CAT 966M (7 Dias)", badge: "🥇 OURO" },
        { rank: 3, username: "minerador_sp", weeklyTonnage: 4890, prize: "CAT 140M (7 Dias)", badge: "🥈 PRATA" },
        { rank: 4, username: "frota_brasil", weeklyTonnage: 3750, prize: "Bônus +20% XP", badge: "🥉 BRONZE" },
        { rank: 5, username: "joao_escavadeira", weeklyTonnage: 2900, prize: "Bônus +15% XP", badge: "⭐ TOP 5" },
        { rank: 6, username: "ana_cat", weeklyTonnage: 2100, prize: "Selo VIP Ouro", badge: "⭐ TOP 10" },
        { rank: 7, username: "pedro_br", weeklyTonnage: 1850, prize: "Selo VIP Prata", badge: "⭐ TOP 10" },
        { rank: 8, username: "lucas_cat", weeklyTonnage: 1400, prize: "Selo VIP Prata", badge: "⭐ TOP 10" }
    ];

    $: displayList = leaderboard && leaderboard.length > 0 ? leaderboard : fallbackLeaderboard;
</script>

{#if show}
    <div 
        class="modal-overlay open active" 
        role="button" 
        tabindex="0" 
        on:click|self={onClose} 
        on:keydown|self={(e) => { if (e.key === 'Escape' || e.key === 'Enter' || e.key === ' ') onClose(); }}
    >
        <div class="modal-card open" style="max-width: 480px; background: #0f172a; color: #fff; border: 2px solid #ffb200; border-radius: 16px; overflow: hidden; box-shadow: 0 10px 30px rgba(0,0,0,0.5);">
            <!-- Header Modal -->
            <div style="background: linear-gradient(135deg, #1e293b, #0f172a); padding: 18px; border-bottom: 2px solid #ffb200; display: flex; justify-content: space-between; align-items: center;">
                <div style="display: flex; align-items: center; gap: 10px;">
                    <div style="background: #ffb200; width: 38px; height: 38px; border-radius: 8px; display: flex; align-items: center; justify-content: center; font-size: 1.3rem;">
                        🏆
                    </div>
                    <div>
                        <h3 style="margin: 0; font-family: 'Oswald', sans-serif; color: #ffb200; font-size: 1.25rem; letter-spacing: 0.5px;">
                            CORRIDA DE MINERAÇÃO DA SEMANA
                        </h3>
                        <span style="font-size: 0.72rem; color: #94a3b8; font-family: monospace;">
                            Prêmios Semanal em Frotas de 7 Dias (Sem Custo)
                        </span>
                    </div>
                </div>
                <button on:click={onClose} style="background: transparent; border: none; color: #94a3b8; font-size: 1.4rem; cursor: pointer;">&times;</button>
            </div>

            <div style="padding: 16px; max-height: 75vh; overflow-y: auto;">
                <!-- Pódio Top 3 -->
                <div style="display: flex; justify-content: center; align-items: flex-end; gap: 8px; margin-bottom: 20px; padding: 10px 0;">
                    <!-- 2º Lugar -->
                    {#if displayList[1]}
                        <div style="text-align: center; flex: 1;">
                            <div style="font-size: 1.2rem;">🥈</div>
                            <div style="background: #1e293b; border: 1px solid #94a3b8; border-radius: 8px; padding: 8px 4px; height: 80px; display: flex; flex-direction: column; justify-content: center;">
                                <strong style="font-size: 0.75rem; color: #e2e8f0; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">{displayList[1].username}</strong>
                                <span style="font-size: 0.7rem; color: #38bdf8; font-family: monospace;">{displayList[1].weeklyTonnage} Ton</span>
                                <span style="font-size: 0.62rem; color: #ffb200; margin-top: 2px;">{displayList[1].prize}</span>
                            </div>
                        </div>
                    {/if}

                    <!-- 1º Lugar -->
                    {#if displayList[0]}
                        <div style="text-align: center; flex: 1.2;">
                            <div style="font-size: 1.6rem; margin-bottom: -4px;">👑</div>
                            <div style="background: linear-gradient(135deg, #332000, #1e293b); border: 2px solid #ffb200; border-radius: 10px; padding: 10px 4px; height: 100px; display: flex; flex-direction: column; justify-content: center; box-shadow: 0 0 12px rgba(255,178,0,0.3);">
                                <strong style="font-size: 0.85rem; color: #ffb200; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">{displayList[0].username}</strong>
                                <span style="font-size: 0.78rem; color: #22c55e; font-family: monospace; font-weight: 800;">{displayList[0].weeklyTonnage} Ton</span>
                                <span style="font-size: 0.65rem; color: #fff; background: #ffb200; color: #111; font-weight: 900; border-radius: 4px; padding: 1px 4px; margin-top: 4px; display: inline-block;">{displayList[0].prize}</span>
                            </div>
                        </div>
                    {/if}

                    <!-- 3º Lugar -->
                    {#if displayList[2]}
                        <div style="text-align: center; flex: 1;">
                            <div style="font-size: 1.2rem;">🥉</div>
                            <div style="background: #1e293b; border: 1px solid #b45309; border-radius: 8px; padding: 8px 4px; height: 75px; display: flex; flex-direction: column; justify-content: center;">
                                <strong style="font-size: 0.75rem; color: #e2e8f0; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">{displayList[2].username}</strong>
                                <span style="font-size: 0.7rem; color: #38bdf8; font-family: monospace;">{displayList[2].weeklyTonnage} Ton</span>
                                <span style="font-size: 0.62rem; color: #ffb200; margin-top: 2px;">{displayList[2].prize}</span>
                            </div>
                        </div>
                    {/if}
                </div>

                <!-- Card de Posição do Usuário -->
                {#if userRank}
                    <div style="background: linear-gradient(90deg, #1e293b, #0f172a); border: 1px solid #ffb200; border-radius: 10px; padding: 12px; margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center;">
                        <div>
                            <span style="font-size: 0.7rem; color: #94a3b8; display: block;">Sua Posição no Ranking</span>
                            <strong style="color: #ffb200; font-size: 1.1rem; font-family: 'Oswald', sans-serif;">
                                #{userRank.rank || '1º'} LUGAR ({userRank.badge || 'LENDÁRIO'})
                            </strong>
                        </div>
                        <div style="text-align: right;">
                            <span style="font-size: 0.7rem; color: #94a3b8; display: block;">Volume Minado</span>
                            <strong style="color: #22c55e; font-family: monospace; font-size: 1rem;">
                                {(userRank.weeklyTonnage || 0).toLocaleString('pt-BR')} Ton
                            </strong>
                        </div>
                    </div>
                {/if}

                <!-- Tabela dos Top 10 -->
                <h4 style="margin: 0 0 10px 0; font-size: 0.85rem; color: #94a3b8; font-family: 'Oswald', sans-serif;">TOP 10 MINERADORES DA SEMANA</h4>
                <div style="display: flex; flex-direction: column; gap: 6px;">
                    {#each displayList as item, index}
                        <div style="background: {index % 2 === 0 ? '#1e293b' : '#0f172a'}; border: 1px solid #334155; border-radius: 8px; padding: 10px 12px; display: flex; align-items: center; justify-content: space-between;">
                            <div style="display: flex; align-items: center; gap: 10px;">
                                <span style="font-family: 'Oswald', sans-serif; font-size: 0.95rem; font-weight: 800; color: {index === 0 ? '#ffb200' : index === 1 ? '#cbd5e1' : index === 2 ? '#d97706' : '#94a3b8'}; width: 24px;">
                                    #{index + 1}
                                </span>
                                <div>
                                    <strong style="font-size: 0.82rem; color: #f8fafc; display: block;">{item.username}</strong>
                                    <span style="font-size: 0.68rem; color: #ffb200;">Prêmio: {item.prize}</span>
                                </div>
                            </div>
                            <div style="text-align: right;">
                                <span style="font-size: 0.82rem; font-family: monospace; font-weight: 700; color: #38bdf8;">
                                    {item.weeklyTonnage.toLocaleString('pt-BR')} Ton
                                </span>
                            </div>
                        </div>
                    {/each}
                </div>
            </div>

            <!-- Footer Modal com Contador -->
            <div style="background: #1e293b; padding: 12px; border-top: 1px solid #334155; text-align: center; font-size: 0.78rem; color: #94a3b8;">
                ⏳ Temporada Semanal Encerra em <strong>{seasonEndsInHours} Horas</strong> (Prêmios Entregues Automaticamente)
            </div>
        </div>
    </div>
{/if}
