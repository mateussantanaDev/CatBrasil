<script>
    export let garage = [];
    export let countdownDisplay = "10:00";
    export let onCollectYield = () => {};

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
</script>

<div class="tab-content active">
    <h2 class="section-title">Minha Frota <span class="dot"></span></h2>
    
    {#each garage as machine (machine.id)}
        <div class="alert-box" style="background: {machine.state === 'expirado' ? '#FFCDD2' : '#FFF3CD'}; border-color: {machine.state === 'expirado' ? 'var(--cat-red)' : '#E0C36A'};">
            {#if machine.state === 'aguardando'}
                <span>Produção em andamento. Liberação em <strong>{formatMachineTime(machine)}</strong>.</span>
            {:else if machine.state === 'disponivel'}
                <span><strong>COLETA DISPONÍVEL!</strong> Expira em <strong class="highlight-red">{formatMachineTime(machine)}</strong>.</span>
            {:else if machine.state === 'expirado'}
                <span><strong>ATRASO &gt; 10MIN!</strong> Você perdeu o prazo limite de coleta. Nova liberação amanhã.</span>
            {/if}
        </div>
        
        <div class="store-card" id="machineCard">
            <div class="store-card-img" style="background-image: url('https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=400&q=80');"></div>
            <div class="store-card-body">
                <h3 class="store-card-title">{machine.name}</h3>
                <div class="store-card-details">
                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;">
                        <span>Produção hoje:</span>
                        <span class="mono-text highlight-green">+R$ {machine.yieldAmount ? machine.yieldAmount.toFixed(2) : '0.00'}</span>
                    </div>
                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;">
                        <span>Status:</span>
                        <span class="mono-text {machine.state === 'expirado' ? 'highlight-red' : 'highlight-yellow'}">
                            {machine.state === 'aguardando' ? 'Aguardando Liberação' : machine.state === 'disponivel' ? 'Disponível' : 'Expirado'}
                        </span>
                    </div>
                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;">
                        <span>{machine.state === 'aguardando' ? 'Liberação em:' : 'Tolerância Limite:'}</span>
                        <span class="mono-text">{formatMachineTime(machine)}</span>
                    </div>
                </div>
                
                <button 
                    class="btn {machine.state === 'disponivel' ? 'btn-claim' : 'btn-primary'}" 
                    disabled={machine.state !== 'disponivel'} 
                    on:click={() => onCollectYield(machine.id)}
                    style="padding: 6px; font-size: 0.75rem; margin-top: 4px; border-color: var(--cat-black);"
                >
                    {#if machine.state === 'aguardando'}
                        Aguardando
                    {:else if machine.state === 'disponivel'}
                        Coletar
                    {:else}
                        Expirado
                    {/if}
                </button>
            </div>
        </div>
    {/each}
</div>
