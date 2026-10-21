<script>
    export let show = false;
    export let user = {};
    export let onClose = () => {};
    export let onAssembleMachine = () => {};

    $: motorVal = (user?.partsInventory?.motorC15 ?? user?.motorC15Count ?? 0);
    $: esteiraVal = (user?.partsInventory?.esteiraReforcada ?? user?.esteiraReforcadaCount ?? 0);
    $: filtroVal = (user?.partsInventory?.filtroOleo ?? user?.filtroOleoCount ?? 0);
    $: turboVal = (user?.partsInventory?.turboGarrett ?? user?.turboGarrettCount ?? 0);
    $: collectedCount = (motorVal > 0 ? 1 : 0) + (esteiraVal > 0 ? 1 : 0) + (filtroVal > 0 ? 1 : 0) + (turboVal > 0 ? 1 : 0);
    $: canAssemble = collectedCount === 4;
</script>

{#if show}
    <div 
        class="modal-overlay open active" 
        role="button" 
        tabindex="0" 
        on:click|self={onClose} 
        on:keydown|self={(e) => { if (e.key === 'Escape' || e.key === 'Enter' || e.key === ' ') onClose(); }}
    >
        <div class="modal-card open" style="max-width: 460px; background: #0f172a; color: #fff; border: 2px solid #ffb200; border-radius: 16px; overflow: hidden; box-shadow: 0 10px 30px rgba(0,0,0,0.5);">
            <!-- Header Modal -->
            <div style="background: linear-gradient(135deg, #1e293b, #0f172a); padding: 18px; border-bottom: 2px solid #ffb200; display: flex; justify-content: space-between; align-items: center;">
                <div style="display: flex; align-items: center; gap: 10px;">
                    <div style="background: #ffb200; width: 38px; height: 38px; border-radius: 8px; display: flex; align-items: center; justify-content: center; font-size: 1.3rem;">
                        🧩
                    </div>
                    <div>
                        <h3 style="margin: 0; font-family: 'Oswald', sans-serif; color: #ffb200; font-size: 1.2rem; letter-spacing: 0.5px;">
                            OFICINA DE MONTAGEM CATERPILLAR
                        </h3>
                        <span style="font-size: 0.72rem; color: #94a3b8; font-family: monospace;">
                            Monte 1x Escavadeira CAT 320 Gratuitamente
                        </span>
                    </div>
                </div>
                <button class="modal-close btn-close-pecas" on:click={onClose} style="background: transparent; border: none; color: #94a3b8; font-size: 1.4rem; cursor: pointer;">&times;</button>
            </div>

            <div style="padding: 16px;">
                <!-- Card Informativo do Progresso -->
                <div style="background: #1e293b; border: 1px solid #334155; border-radius: 10px; padding: 14px; margin-bottom: 16px; text-align: center;">
                    <span style="font-size: 0.75rem; color: #94a3b8; display: block; margin-bottom: 4px;">PROGRESSO DO CONJUNTO DE PEÇAS</span>
                    <strong style="color: #ffb200; font-size: 1.3rem; font-family: 'Oswald', sans-serif;">
                        {collectedCount} DE 4 PEÇAS COLETADAS
                    </strong>
                    <div style="background: #0f172a; height: 8px; border-radius: 4px; overflow: hidden; margin-top: 8px; border: 1px solid #334155;">
                        <div style="background: linear-gradient(90deg, #ffb200, #22c55e); height: 100%; width: {(collectedCount / 4) * 100}%; transition: width 0.3s;"></div>
                    </div>
                </div>

                <!-- Grid de Peças -->
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 20px;">
                    <!-- Peça 1: Motor C15 -->
                    <div style="background: {motorVal > 0 ? '#1e293b' : '#0f172a'}; border: 2px solid {motorVal > 0 ? '#22c55e' : '#334155'}; border-radius: 10px; padding: 12px; text-align: center;">
                        <div style="font-size: 1.8rem; margin-bottom: 4px; opacity: {motorVal > 0 ? 1 : 0.4};">⚙️</div>
                        <strong style="font-size: 0.78rem; color: {motorVal > 0 ? '#fff' : '#64748b'}; display: block;">Motor C15 Heavy Duty</strong>
                        <span style="font-size: 0.7rem; font-weight: 800; color: {motorVal > 0 ? '#22c55e' : '#64748b'}; font-family: monospace;">
                            {motorVal > 0 ? `POSSUI (${motorVal})` : 'FALTA'}
                        </span>
                    </div>

                    <!-- Peça 2: Esteira Reforçada -->
                    <div style="background: {esteiraVal > 0 ? '#1e293b' : '#0f172a'}; border: 2px solid {esteiraVal > 0 ? '#22c55e' : '#334155'}; border-radius: 10px; padding: 12px; text-align: center;">
                        <div style="font-size: 1.8rem; margin-bottom: 4px; opacity: {esteiraVal > 0 ? 1 : 0.4};">🚜</div>
                        <strong style="font-size: 0.78rem; color: {esteiraVal > 0 ? '#fff' : '#64748b'}; display: block;">Esteira Reforçada CAT</strong>
                        <span style="font-size: 0.7rem; font-weight: 800; color: {esteiraVal > 0 ? '#22c55e' : '#64748b'}; font-family: monospace;">
                            {esteiraVal > 0 ? `POSSUI (${esteiraVal})` : 'FALTA'}
                        </span>
                    </div>

                    <!-- Peça 3: Filtro de Óleo -->
                    <div style="background: {filtroVal > 0 ? '#1e293b' : '#0f172a'}; border: 2px solid {filtroVal > 0 ? '#22c55e' : '#334155'}; border-radius: 10px; padding: 12px; text-align: center;">
                        <div style="font-size: 1.8rem; margin-bottom: 4px; opacity: {filtroVal > 0 ? 1 : 0.4};">🛢️</div>
                        <strong style="font-size: 0.78rem; color: {filtroVal > 0 ? '#fff' : '#64748b'}; display: block;">Filtro de Óleo Alta Vazão</strong>
                        <span style="font-size: 0.7rem; font-weight: 800; color: {filtroVal > 0 ? '#22c55e' : '#64748b'}; font-family: monospace;">
                            {filtroVal > 0 ? `POSSUI (${filtroVal})` : 'FALTA'}
                        </span>
                    </div>

                    <!-- Peça 4: Turbo Garrett -->
                    <div style="background: {turboVal > 0 ? '#1e293b' : '#0f172a'}; border: 2px solid {turboVal > 0 ? '#22c55e' : '#334155'}; border-radius: 10px; padding: 12px; text-align: center;">
                        <div style="font-size: 1.8rem; margin-bottom: 4px; opacity: {turboVal > 0 ? 1 : 0.4};">🌀</div>
                        <strong style="font-size: 0.78rem; color: {turboVal > 0 ? '#fff' : '#64748b'}; display: block;">Turbo Garrett T66</strong>
                        <span style="font-size: 0.7rem; font-weight: 800; color: {turboVal > 0 ? '#22c55e' : '#64748b'}; font-family: monospace;">
                            {turboVal > 0 ? `POSSUI (${turboVal})` : 'FALTA'}
                        </span>
                    </div>
                </div>

                <!-- Botão de Montagem Completa -->
                {#if canAssemble}
                    <button 
                        class="btn btn-primary" 
                        style="width: 100%; padding: 14px; font-size: 0.95rem; font-weight: 900; background: linear-gradient(90deg, #ffb200, #22c55e); color: #111; border-radius: 10px; box-shadow: 0 4px 14px rgba(34,197,94,0.4);"
                        on:click={onAssembleMachine}
                    >
                        ⚙️ MONTAR ESCAVADEIRA CAT 320 AGORA (CUSTO R$ 0,00)
                    </button>
                {:else}
                    <button 
                        class="btn" 
                        disabled
                        style="width: 100%; padding: 14px; font-size: 0.85rem; font-weight: 700; background: #334155; color: #94a3b8; border-radius: 10px; cursor: not-allowed;"
                    >
                        🔒 COLETE AS 4 PEÇAS NA ROLETA/CAIXAS PARA LIBERAR MONTAGEM
                    </button>
                {/if}
            </div>

            <!-- Footer Modal -->
            <div style="background: #1e293b; padding: 10px; border-top: 1px solid #334155; text-align: center; font-size: 0.75rem; color: #94a3b8;">
                💡 Obtenha peças girando a Roleta Diária ou abrindo Caixas Misteriosas Caterpillar!
            </div>
        </div>
    </div>
{/if}
