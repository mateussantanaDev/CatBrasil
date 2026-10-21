<script>
    import { onMount } from 'svelte';
    import { api } from '../services/api.js';
    import { gestaoService } from '../services/gestaoService.js';

    export let onRentMachine = () => {};

    let selectedFilter = 'TODOS';
    let catalog = [];
    let loading = true;

    async function loadCatalog() {
        try {
            loading = true;
            const res = await api.getStoreMachines();
            if (Array.isArray(res) && res.length > 0) {
                catalog = res.filter(m => m.active !== false);
            } else {
                catalog = gestaoService.getMachinesCatalog().filter(m => m.active !== false);
            }
        } catch (err) {
            console.warn('[Loja] Carregando catálogo via fallback local:', err.message);
            catalog = gestaoService.getMachinesCatalog().filter(m => m.active !== false);
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        loadCatalog();
    });

    $: filteredMachines = selectedFilter === 'REINVEST' 
        ? catalog.filter(m => m.reinvestOnly || m.category === 'REINVEST' || m.category === 'REINVESTIMENTO') 
        : selectedFilter === 'PADRAO' 
            ? catalog.filter(m => !m.reinvestOnly && m.category !== 'REINVEST' && m.category !== 'REINVESTIMENTO') 
            : catalog;
</script>

<div class="tab-content active" style="width: 100%; box-sizing: border-box; padding-bottom: 20px;">
    <!-- Título da Seção -->
    <h2 class="section-title">Loja de Frotas & Contratos <span class="dot"></span></h2>

    <!-- Filtros por Categoria -->
    <div style="display: flex; gap: 8px; margin-bottom: 16px; overflow-x: auto; padding-bottom: 4px; -webkit-overflow-scrolling: touch;">
        <button 
            class="btn" 
            style="padding: 7px 14px; font-size: 0.75rem; font-family: 'Oswald', sans-serif; text-transform: uppercase; font-weight: 700; background: {selectedFilter === 'TODOS' ? 'var(--cat-yellow)' : 'var(--cat-gray)'}; color: {selectedFilter === 'TODOS' ? 'var(--cat-black)' : 'var(--cat-white)'}; border: 2px solid var(--cat-black); box-shadow: {selectedFilter === 'TODOS' ? '2px 2px 0px var(--cat-black)' : 'none'}; cursor: pointer; flex-shrink: 0;"
            on:click={() => selectedFilter = 'TODOS'}
        >
            TODAS AS FROTAS ({catalog.length})
        </button>
        <button 
            class="btn" 
            style="padding: 7px 14px; font-size: 0.75rem; font-family: 'Oswald', sans-serif; text-transform: uppercase; font-weight: 700; background: {selectedFilter === 'REINVEST' ? 'var(--cat-yellow)' : 'var(--cat-gray)'}; color: {selectedFilter === 'REINVEST' ? 'var(--cat-black)' : 'var(--cat-white)'}; border: 2px solid var(--cat-black); box-shadow: {selectedFilter === 'REINVEST' ? '2px 2px 0px var(--cat-black)' : 'none'}; cursor: pointer; flex-shrink: 0;"
            on:click={() => selectedFilter = 'REINVEST'}
        >
            ⚡ REINVEST TURBO (7 DIAS)
        </button>
        <button 
            class="btn" 
            style="padding: 7px 14px; font-size: 0.75rem; font-family: 'Oswald', sans-serif; text-transform: uppercase; font-weight: 700; background: {selectedFilter === 'PADRAO' ? 'var(--cat-yellow)' : 'var(--cat-gray)'}; color: {selectedFilter === 'PADRAO' ? 'var(--cat-black)' : 'var(--cat-white)'}; border: 2px solid var(--cat-black); box-shadow: {selectedFilter === 'PADRAO' ? '2px 2px 0px var(--cat-black)' : 'none'}; cursor: pointer; flex-shrink: 0;"
            on:click={() => selectedFilter = 'PADRAO'}
        >
            LINHA PESADA (30 DIAS)
        </button>
    </div>

    <!-- Lista de Cards que se Autoajustam sem Cortar Texto -->
    {#if loading && catalog.length === 0}
        <div style="background: #fff; border: 2px solid var(--cat-black); box-shadow: var(--brutal-shadow); padding: 24px; text-align: center; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; color: #64748b;">
            🚜 CARREGANDO EQUIPAMENTOS CATERPILLAR...
        </div>
    {:else if filteredMachines.length === 0}
        <div style="background: #fff; border: 2px solid var(--cat-black); box-shadow: var(--brutal-shadow); padding: 24px; text-align: center; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; color: #64748b;">
            NENHUMA MÁQUINA DISPONÍVEL NESTA CATEGORIA NO MOMENTO.
        </div>
    {:else}
        {#each filteredMachines as machine (machine.id)}
            {@const isReinvest = machine.reinvestOnly || machine.category === 'REINVEST' || machine.category === 'REINVESTIMENTO'}
            {@const duration = machine.durationDays || (isReinvest ? 7 : 30)}
            {@const price = Number(machine.price || 0)}
            {@const dailyYield = Number(machine.dailyYield || 0)}
            {@const totalReturn = Number(machine.totalContractReturn || (dailyYield * duration))}
            {@const netProfit = totalReturn - price}

            <!-- Card com Altura Autoajustável e Overflow Visível -->
            <div 
                class="card" 
                style="width: 100%; height: auto; min-height: fit-content; display: flex; flex-direction: column; overflow: visible; margin-bottom: 18px; box-sizing: border-box;"
            >
                <!-- Imagem do Equipamento -->
                <div 
                    class="card-img" 
                    style="height: 140px; width: 100%; background-image: url('{machine.imageUrl || 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80'}'); position: relative; background-size: cover; background-position: center; border-bottom: 2px solid var(--cat-black);"
                >
                    <!-- Badge de Categoria -->
                    {#if isReinvest}
                        <span style="position: absolute; top: 8px; left: 8px; background: var(--cat-yellow); color: var(--cat-black); font-size: 0.65rem; font-weight: 900; font-family: 'Oswald', sans-serif; padding: 3px 8px; border: 2px solid var(--cat-black); border-radius: 2px; letter-spacing: 0.5px; box-shadow: 2px 2px 0px rgba(0,0,0,0.4);">
                            ⚡ REINVEST TURBO
                        </span>
                    {/if}

                    <!-- Badge de Duração -->
                    <span style="position: absolute; top: 8px; right: 8px; background: var(--cat-black); color: var(--cat-yellow); font-size: 0.65rem; font-weight: 700; font-family: 'JetBrains Mono', monospace; padding: 3px 8px; border: 2px solid var(--cat-yellow); border-radius: 2px; box-shadow: 2px 2px 0px rgba(0,0,0,0.4);">
                        {duration} DIAS
                    </span>
                </div>

                <!-- Corpo do Card -->
                <div class="card-body" style="padding: 14px; display: flex; flex-direction: column; gap: 8px; width: 100%; box-sizing: border-box;">
                    <!-- Título -->
                    <h3 class="card-title" style="font-size: 1.15rem; font-family: 'Oswald', sans-serif; text-transform: uppercase; color: var(--cat-black); margin: 0; line-height: 1.2; word-break: break-word;">
                        {machine.name}
                    </h3>

                    <!-- Tabela de Dados Financeiros sem quebra feia -->
                    <div style="display: flex; flex-direction: column; gap: 4px; margin: 4px 0 6px 0;">
                        <div class="data-row" style="margin: 0; padding: 3px 0; font-size: 0.78rem;">
                            <span style="color: #444;">Custo do Contrato:</span>
                            <strong class="mono-text" style="color: var(--cat-black); font-size: 0.85rem;">
                                R$ {price.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                            </strong>
                        </div>
                        <div class="data-row" style="margin: 0; padding: 3px 0; font-size: 0.78rem;">
                            <span style="color: #444;">Rendimento Diário:</span>
                            <strong class="mono-text highlight-green" style="font-size: 0.85rem;">
                                +R$ {dailyYield.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}/dia
                            </strong>
                        </div>
                        <div class="data-row" style="margin: 0; padding: 3px 0; font-size: 0.78rem;">
                            <span style="color: #444;">Retorno Total ({duration}d):</span>
                            <strong class="mono-text highlight-green" style="font-size: 0.9rem; font-weight: 900;">
                                R$ {totalReturn.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                            </strong>
                        </div>
                        <div class="data-row" style="margin: 0; padding: 3px 0; font-size: 0.78rem; border-bottom: none;">
                            <span style="color: #666;">Lucro Líquido Estimado:</span>
                            <strong class="mono-text highlight-yellow" style="font-size: 0.82rem;">
                                +R$ {netProfit.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                            </strong>
                        </div>
                    </div>

                    <!-- Botão de Contratação Amplo e sem Cortar -->
                    <button 
                        class="btn btn-primary" 
                        on:click={() => onRentMachine(machine.id)}
                        style="width: 100%; min-height: 44px; padding: 10px; font-size: 0.85rem; font-family: 'Oswald', sans-serif; font-weight: 700; text-transform: uppercase; letter-spacing: 0.5px; border: 2px solid var(--cat-black); box-shadow: 2px 2px 0px var(--cat-black); display: flex; align-items: center; justify-content: center; margin-top: 4px; cursor: pointer; box-sizing: border-box;"
                    >
                        {isReinvest ? 'REINVESTIR AGORA' : 'ALUGAR EQUIPAMENTO'}
                    </button>
                </div>
            </div>
        {/each}
    {/if}
</div>
