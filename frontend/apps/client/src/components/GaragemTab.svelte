<script>
    export let garage = [];
    export let user = {};
    export let countdownDisplay = "10:00";
    export let onCollectYield = () => {};
    export let onRecoverInsurance = () => {};

    function formatMachineTime(machine) {
        if (typeof machine.secondsRemaining === 'number' && machine.secondsRemaining > 0) {
            const h = Math.floor(machine.secondsRemaining / 3600);
            const m = Math.floor((machine.secondsRemaining % 3600) / 60);
            const s = machine.secondsRemaining % 60;
            if (h > 0) {
                return `${h}h ${m.toString().padStart(2, '0')}m`;
            }
            return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
        }
        return countdownDisplay;
    }

    function getMachineImage(name = '') {
        const lower = name.toLowerCase();
        if (lower.includes('966m') || lower.includes('carregadeira')) {
            return 'https://images.unsplash.com/photo-1541888946425-d0fbb186a5b3?w=500&q=80';
        }
        if (lower.includes('745') || lower.includes('caminhão') || lower.includes('caminhao')) {
            return 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80';
        }
        if (lower.includes('d8t') || lower.includes('trator')) {
            return 'https://images.unsplash.com/photo-1578575437130-527eed3abbec?w=500&q=80';
        }
        if (lower.includes('cs56b') || lower.includes('rolo')) {
            return 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80';
        }
        return 'https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=500&q=80';
    }

    $: totalDailyProduction = garage.reduce((acc, m) => acc + (Number(m.yieldAmount) || 0), 0);
    $: availableCollectsCount = garage.filter(m => m.state === 'disponivel').length;
</script>

<div class="tab-content active" style="width: 100%; box-sizing: border-box; padding-bottom: 20px;">
    <!-- Título da Seção -->
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
        <h2 class="section-title" style="margin-bottom: 0;">Minha Frota <span class="dot"></span></h2>
        <span class="mono-text" style="font-size: 0.75rem; background: var(--cat-black); color: var(--cat-yellow); padding: 4px 8px; border-radius: 2px; border: 1px solid var(--cat-yellow);">
            {garage.length} EQUIPAMENTOS
        </span>
    </div>

    <!-- Banner Resumo Operacional da Garagem -->
    <div style="background: #fff; border: 2px solid var(--cat-black); box-shadow: var(--brutal-shadow); padding: 12px 14px; margin-bottom: 14px; border-radius: 2px;">
        <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.78rem;">
            <span style="color: #444; font-weight: 600;">Produção Diária Estimada:</span>
            <strong class="mono-text highlight-green" style="font-size: 0.95rem;">
                +R$ {totalDailyProduction.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}/dia
            </strong>
        </div>
        {#if availableCollectsCount > 0}
            <div style="margin-top: 6px; padding-top: 6px; border-top: 1px dashed #ddd; display: flex; align-items: center; gap: 6px; font-size: 0.72rem; color: var(--cat-green); font-weight: 700;">
                <span>🟢</span>
                <span>Você tem <strong>{availableCollectsCount} máquina(s)</strong> pronta(s) para coleta agora!</span>
            </div>
        {/if}
    </div>

    <!-- Banner Vale Seguro CAT se o operador possuir vouchers -->
    {#if (user.insuranceVouchersCount || 0) > 0}
        <div style="background: #0f172a; border: 2px solid #3b82f6; box-shadow: var(--brutal-shadow); padding: 12px 14px; margin-bottom: 14px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap;">
            <div>
                <div style="display: flex; align-items: center; gap: 6px;">
                    <span style="font-size: 1.2rem;">🛡️</span>
                    <strong style="color: #93c5fd; font-size: 0.82rem; text-transform: uppercase;">
                        VALE SEGURO CAT ATIVO ({user.insuranceVouchersCount})
                    </strong>
                </div>
                <span style="font-size: 0.7rem; color: #cbd5e1; display: block; margin-top: 2px;">
                    Recupere rendimentos de até 2 dias de coletas perdidas da sua frota ativa.
                </span>
            </div>
            <button 
                on:click={onRecoverInsurance}
                style="background: #2563eb; color: #fff; border: 1px solid #1d4ed8; padding: 7px 12px; font-family: 'JetBrains Mono', monospace; font-size: 0.72rem; font-weight: 800; cursor: pointer; text-transform: uppercase; white-space: nowrap;"
            >
                RESGATAR AGORA
            </button>
        </div>
    {/if}

    <!-- Lista de Máquinas da Garagem sem Cortes -->
    {#if garage.length === 0}
        <div style="background: #fff; border: 2px solid var(--cat-black); box-shadow: var(--brutal-shadow); padding: 30px 20px; text-align: center; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; color: #64748b;">
            🚜 NENHUM EQUIPAMENTO NA SUA GARAGEM.<br />
            <span style="font-size: 0.7rem; color: #94a3b8; display: block; margin-top: 8px;">
                Acesse a aba <strong>Loja</strong> para alugar sua primeira máquina Caterpillar!
            </span>
        </div>
    {:else}
        {#each garage as machine (machine.id)}
            {@const isAvailable = machine.state === 'disponivel'}
            {@const isExpired = machine.state === 'expirado'}
            {@const yieldVal = Number(machine.yieldAmount || 0)}

            <div 
                class="card" 
                style="width: 100%; height: auto; min-height: fit-content; display: flex; flex-direction: column; overflow: visible; margin-bottom: 16px; box-sizing: border-box;"
            >
                <!-- Imagem do Equipamento com Badges Sobrepostos -->
                <div 
                    class="card-img" 
                    style="height: 130px; width: 100%; background-image: url('{getMachineImage(machine.name)}'); position: relative; background-size: cover; background-position: center; border-bottom: 2px solid var(--cat-black);"
                >
                    <!-- Status Badge -->
                    <span 
                        style="position: absolute; top: 8px; left: 8px; font-size: 0.65rem; font-weight: 800; font-family: 'Oswald', sans-serif; padding: 3px 8px; border: 2px solid var(--cat-black); border-radius: 2px; letter-spacing: 0.5px; box-shadow: 2px 2px 0px rgba(0,0,0,0.4); background: {isAvailable ? '#4CAF50' : isExpired ? 'var(--cat-red)' : 'var(--cat-yellow)'}; color: {isAvailable || isExpired ? '#fff' : 'var(--cat-black)'};"
                    >
                        {isAvailable ? '🟢 PRONTO PARA COLETA' : isExpired ? '🔴 PRAZO EXPIRADO' : '⏳ EM PRODUÇÃO'}
                    </span>

                    <!-- Rendimento Badge -->
                    <span 
                        style="position: absolute; top: 8px; right: 8px; background: var(--cat-black); color: var(--cat-yellow); font-size: 0.68rem; font-weight: 800; font-family: 'JetBrains Mono', monospace; padding: 3px 8px; border: 2px solid var(--cat-yellow); border-radius: 2px; box-shadow: 2px 2px 0px rgba(0,0,0,0.4);"
                    >
                        +R$ {yieldVal.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                    </span>
                </div>

                <!-- Corpo do Card -->
                <div class="card-body" style="padding: 12px 14px; display: flex; flex-direction: column; gap: 6px; width: 100%; box-sizing: border-box;">
                    <!-- Nome do Equipamento -->
                    <h3 class="card-title" style="font-size: 1.1rem; font-family: 'Oswald', sans-serif; text-transform: uppercase; color: var(--cat-black); margin: 0; line-height: 1.2; word-break: break-word;">
                        {machine.name}
                    </h3>

                    <!-- Alerta de Tolerância / Liberação -->
                    {#if isAvailable}
                        <div style="background: #e8f5e9; border: 1px solid #81c784; padding: 6px 10px; border-radius: 2px; font-size: 0.72rem; color: #1b5e20; display: flex; justify-content: space-between; align-items: center;">
                            <span>⏳ Tolerância Limite (10 min):</span>
                            <strong class="mono-text" style="color: var(--cat-red); font-size: 0.8rem;">{formatMachineTime(machine)}</strong>
                        </div>
                    {:else if isExpired}
                        <div style="background: #ffebee; border: 1px solid #ef9a9a; padding: 6px 10px; border-radius: 2px; font-size: 0.72rem; color: #b71c1c;">
                            ⚠️ Tolerância de 10 min ultrapassada. Nova liberação no próximo ciclo.
                        </div>
                    {:else}
                        <div style="background: #fff8e1; border: 1px solid #ffe082; padding: 6px 10px; border-radius: 2px; font-size: 0.72rem; color: #f57f17; display: flex; justify-content: space-between; align-items: center;">
                            <span>⏳ Liberação de Coleta em:</span>
                            <strong class="mono-text" style="color: var(--cat-black); font-size: 0.8rem;">{formatMachineTime(machine)}</strong>
                        </div>
                    {/if}

                    <!-- Dados Técnicos -->
                    <div style="display: flex; flex-direction: column; gap: 3px; margin: 4px 0;">
                        <div class="data-row" style="margin: 0; padding: 2px 0; font-size: 0.75rem;">
                            <span style="color: #555;">Rendimento do Ciclo:</span>
                            <strong class="mono-text highlight-green" style="font-size: 0.82rem;">
                                +R$ {yieldVal.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                            </strong>
                        </div>
                        <div class="data-row" style="margin: 0; padding: 2px 0; font-size: 0.75rem; border-bottom: none;">
                            <span style="color: #555;">Status da Operação:</span>
                            <strong class="mono-text" style="color: {isAvailable ? 'var(--cat-green)' : isExpired ? 'var(--cat-red)' : '#b8860b'}; font-size: 0.75rem;">
                                {isAvailable ? 'LIBERADO P/ SAQUE' : isExpired ? 'CICLO ENCERRADO' : 'MINERANDO'}
                            </strong>
                        </div>
                    </div>

                    <!-- Botão de Coleta sem Cortes -->
                    <button 
                        class="btn {isAvailable ? 'btn-claim' : 'btn-primary'}" 
                        disabled={!isAvailable}
                        on:click={() => onCollectYield(machine.id)}
                        style="width: 100%; min-height: 42px; padding: 8px 12px; font-size: 0.85rem; font-family: 'Oswald', sans-serif; font-weight: 700; text-transform: uppercase; letter-spacing: 0.5px; border: 2px solid var(--cat-black); box-shadow: 2px 2px 0px var(--cat-black); display: flex; align-items: center; justify-content: center; margin-top: 4px; cursor: {isAvailable ? 'pointer' : 'not-allowed'}; box-sizing: border-box; background: {isAvailable ? '#2E7D32' : isExpired ? '#9e9e9e' : 'var(--cat-gray)'}; color: #fff;"
                    >
                        {#if isAvailable}
                            💰 COLETAR +R$ {yieldVal.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                        {:else if isExpired}
                            CICLO EXPIRADO
                        {:else}
                            ⏳ AGUARDANDO LIBERAÇÃO ({formatMachineTime(machine)})
                        {/if}
                    </button>
                </div>
            </div>
        {/each}
    {/if}
</div>
