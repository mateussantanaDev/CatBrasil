<script>
    import { gestaoService } from '../services/gestaoService.js';

    export let user = {};
    export let onRentMachine = () => {};

    let selectedFilter = 'TODOS';

    $: catalog = gestaoService.getMachinesCatalog().filter(m => m.active);
    $: filteredMachines = selectedFilter === 'REINVEST' 
        ? catalog.filter(m => m.reinvestOnly) 
        : selectedFilter === 'PADRAO' 
            ? catalog.filter(m => !m.reinvestOnly) 
            : catalog;
</script>

<div class="tab-content active" style="padding-bottom: 30px; width: 100%; box-sizing: border-box;">
    <!-- Filtros por Categoria -->
    <div style="display: flex; gap: 8px; margin-bottom: 12px; overflow-x: auto; padding-bottom: 4px;">
        <button 
            class="btn" 
            style="padding: 6px 12px; font-size: 0.72rem; border-radius: 20px; font-weight: 700; background: {selectedFilter === 'TODOS' ? '#ffb200' : '#1e293b'}; color: {selectedFilter === 'TODOS' ? '#111' : '#fff'}; border-color: var(--cat-black);"
            on:click={() => selectedFilter = 'TODOS'}
        >
            TODAS AS FROTAS
        </button>
        <button 
            class="btn" 
            style="padding: 6px 12px; font-size: 0.72rem; border-radius: 20px; font-weight: 700; background: {selectedFilter === 'REINVEST' ? '#ffb200' : '#1e293b'}; color: {selectedFilter === 'REINVEST' ? '#111' : '#fff'}; border-color: var(--cat-black);"
            on:click={() => selectedFilter = 'REINVEST'}
        >
            LINHA REINVEST
        </button>
        <button 
            class="btn" 
            style="padding: 6px 12px; font-size: 0.72rem; border-radius: 20px; font-weight: 700; background: {selectedFilter === 'PADRAO' ? '#ffb200' : '#1e293b'}; color: {selectedFilter === 'PADRAO' ? '#111' : '#fff'}; border-color: var(--cat-black);"
            on:click={() => selectedFilter = 'PADRAO'}
        >
            LINHA PADRÃO
        </button>
    </div>

    {#if filteredMachines.length === 0}
        <div style="background: #fff; border: 2px solid var(--cat-black); padding: 24px; text-align: center; font-family: monospace; font-size: 0.8rem; color: #64748b;">
            NENHUMA MÁQUINA DISPONÍVEL NESTA CATEGORIA NO MOMENTO.
        </div>
    {:else}
        {#each filteredMachines as machine}
            <div class="store-card" style="flex-direction: column; min-height: auto; width: 100%; box-sizing: border-box; margin-bottom: 14px;">
                <div class="store-card-img" style="width: 100%; height: 125px; border-right: none; border-bottom: 2px solid var(--cat-black); background-image: url('{machine.imageUrl || 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=400&q=80'}');"></div>
                <div class="store-card-body" style="padding: 12px; width: 100%; box-sizing: border-box;">
                    <h3 class="store-card-title" style="font-size: 1.05rem; margin-bottom: 6px;">{machine.name}</h3>
                    
                    <div class="store-card-details" style="display: flex; flex-direction: column; gap: 4px; margin-bottom: 8px;">
                        <div class="data-row" style="font-size: 0.75rem; margin: 0; padding: 3px 0; border-bottom: 1px dashed #ccc; display: flex; justify-content: space-between; align-items: center; width: 100%;">
                            <span style="color: #444;">Custo de Aluguel:</span>
                            <span class="mono-text" style="font-weight: 800; color: var(--cat-black);">R$ {machine.price ? machine.price.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</span>
                        </div>
                        <div class="data-row" style="font-size: 0.75rem; margin: 0; padding: 3px 0; border-bottom: 1px dashed #ccc; display: flex; justify-content: space-between; align-items: center; width: 100%;">
                            <span style="color: #444;">Duração do Contrato:</span>
                            <span class="mono-text highlight-yellow" style="font-weight: 800;">{machine.durationDays || 30} Dias</span>
                        </div>
                        <div class="data-row" style="font-size: 0.75rem; margin: 0; padding: 3px 0; border-bottom: 1px dashed #ccc; display: flex; justify-content: space-between; align-items: center; width: 100%;">
                            <span style="color: #444;">Rendimento Diário:</span>
                            <span class="mono-text highlight-green" style="font-weight: 800;">R$ {machine.dailyYield ? machine.dailyYield.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</span>
                        </div>
                        <div class="data-row" style="font-size: 0.75rem; margin: 0; padding: 3px 0; border-bottom: 1px dashed #ccc; display: flex; justify-content: space-between; align-items: center; width: 100%;">
                            <span style="color: #444;">Retorno Total:</span>
                            <span class="mono-text highlight-green" style="font-weight: 900;">R$ {(machine.totalContractReturn || (machine.dailyYield * (machine.durationDays || 30))).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</span>
                        </div>
                    </div>

                    <button 
                        class="btn btn-primary" 
                        on:click={() => onRentMachine(machine.id)}
                        style="padding: 8px; font-size: 0.8rem; margin-top: 4px; border-color: var(--cat-black); width: 100%; box-sizing: border-box; font-family: 'Oswald', sans-serif;"
                    >
                        {machine.reinvestOnly ? 'REINVESTIR AGORA' : 'ALUGAR AGORA'}
                    </button>
                </div>
            </div>
        {/each}
    {/if}
</div>
