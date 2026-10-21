<script>
    import { gestaoService } from '../../../services/gestaoService.js';

    export let rouletteSlices = [];
    export let gestaoConfig = {};
    export let bankMetrics = {};
    export let rouletteMath = {};

    export let onSaveRouletteSettings = () => {};
    export let onNormalizeProbabilities = () => {};
    export let onApplyPresetMaxProfit = () => {};
    export let onApplyPresetViral = () => {};
</script>

<!-- ABA 4: ROLETA & MATRIZ DE PROBABILIDADES DESACOPLADA -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- PanelHeader com KPIs Matemáticos -->
    <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px;">
        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #1e3a8a; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">EXPECTATIVA MATEMÁTICA (EV)</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #1e3a8a; margin-top: 2px;">
                R$ {rouletteMath.expectedValueEV ? rouletteMath.expectedValueEV.toFixed(2) : '0.00'} /giro
            </div>
            <span style="font-size: 0.62rem; color: #64748b;">Custo médio diário por operador</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid {rouletteMath.isProbValid ? '#059669' : '#b91c1c'}; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">SOMA MATRIZ PROBABILIDADES</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: {rouletteMath.isProbValid ? '#065f46' : '#b91c1c'}; margin-top: 2px;">
                {rouletteMath.totalProb || 100}%
            </div>
            <span style="font-size: 0.62rem; color: {rouletteMath.isProbValid ? '#065f46' : '#b91c1c'}; font-weight: 700;">
                {rouletteMath.isProbValid ? 'MATRIZ VÁLIDA (100%)' : 'ALERTA: DIFERENTE DE 100%'}
            </span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #059669; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">PRÊMIOS ROLETA ENTREGUES</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #065f46; margin-top: 2px;">
                R$ {bankMetrics.totalRoulettePrizes ? bankMetrics.totalRoulettePrizes.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #b45309; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">ECONOMIA POR GIROS CANCELADOS</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b45309; margin-top: 2px;">
                R$ {bankMetrics.retainedUnspunPrizes ? bankMetrics.retainedUnspunPrizes.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
        </div>
    </div>

    <!-- Painel de Regras Operacionais da Roleta -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 14px; display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap;">
        <div style="display: flex; align-items: center; gap: 16px;">
            <div style="display: flex; align-items: center; gap: 8px;">
                <input id="chk-strict-spn" type="checkbox" bind:checked={gestaoConfig.roulette.strictModalMode} style="width: 16px; height: 16px;" />
                <label for="chk-strict-spn" style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #0f172a; cursor: pointer;">MODO RÍGIDO: SE FECHAR O MODAL BOAS-VINDAS SEM GIRAR, PERDE O GIRO DO DIA</label>
            </div>
        </div>

        <div style="display: flex; align-items: center; gap: 8px;">
            <input id="chk-dep-spn" type="checkbox" bind:checked={gestaoConfig.roulette.requireDepositToSpin} style="width: 16px; height: 16px;" />
            <label for="chk-dep-spn" style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #b91c1c; cursor: pointer;">EXIGIR AO MENOS 1 DEPÓSITO PARA LIBERAR O GIRO</label>
        </div>
    </div>

    <!-- Presets Rápidos de Probabilidade -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 12px; display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap;">
        <span style="font-size: 0.6rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase;">APLICAR MATRIZ PRESET INSTANTÂNEA:</span>
        
        <div style="display: flex; gap: 8px;">
            <button on:click={onNormalizeProbabilities} style="padding: 6px 12px; font-size: 0.65rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #ffb200; color: #000; cursor: pointer;">
                NORMALIZAR MATRIZ PARA 100%
            </button>
            <button on:click={onApplyPresetMaxProfit} style="padding: 6px 12px; font-size: 0.65rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #047857; color: #fff; cursor: pointer;">
                PRESET: LUCRO MÁXIMO BANCA (EV R$ 6.20)
            </button>
            <button on:click={onApplyPresetViral} style="padding: 6px 12px; font-size: 0.65rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #1e3a8a; color: #fff; cursor: pointer;">
                PRESET: ATRAÇÃO VIRAL (EV R$ 14.50)
            </button>
        </div>
    </div>

    <!-- Tabela Densa de Edição da Matriz de Fatias da Roleta -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 12px 0;">MATRIZ INDIVIDUAL DE FATIAS & CHANCE DE SAÍDA</h4>
        
        <div style="display: flex; flex-direction: column; gap: 8px; margin-bottom: 16px;">
            {#each rouletteSlices as slice, idx}
                {@const sliceEV = (parseFloat(slice.amount) || 0) * ((parseFloat(slice.probability) || 0) / 100)}
                <div style="display: grid; grid-template-columns: 90px 1fr 1fr 1fr 1fr 120px; gap: 12px; padding: 10px; border: 1px solid #e2e8f0; background: #f8fafc; align-items: center;">
                    <div>
                        <strong style="font-family: monospace; font-size: 0.75rem; display: block;">FATIA #{slice.id}</strong>
                        <span style="font-size: 0.55rem; color: #64748b;">SVG SECTOR</span>
                    </div>

                    <div>
                        <label for="slice-lbl-{idx}" style="font-size: 0.55rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">RÓTULO EXIBIDO:</label>
                        <input id="slice-lbl-{idx}" type="text" bind:value={slice.label} style="width: 100%; padding: 4px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <div>
                        <label for="slice-val-{idx}" style="font-size: 0.55rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">VALOR DO PRÊMIO (R$):</label>
                        <input id="slice-val-{idx}" type="number" bind:value={slice.amount} style="width: 100%; padding: 4px; font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #065f46; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <div>
                        <label for="slice-prb-{idx}" style="font-size: 0.55rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">CHANCE DE SAÍDA (%):</label>
                        <input id="slice-prb-{idx}" type="number" step="0.01" bind:value={slice.probability} style="width: 100%; padding: 4px; font-family: monospace; font-size: 0.75rem; font-weight: 900; color: #b91c1c; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <div>
                        <label for="slice-clr-{idx}" style="font-size: 0.55rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">COR FATIA / TEXTO:</label>
                        <div style="display: flex; gap: 4px;">
                            <input id="slice-clr-{idx}" type="color" bind:value={slice.color} style="width: 32px; height: 26px; border: 1px solid #cbd5e1; cursor: pointer;" />
                            <input type="color" bind:value={slice.textColor} style="width: 32px; height: 26px; border: 1px solid #cbd5e1; cursor: pointer;" />
                        </div>
                    </div>

                    <div style="text-align: right;">
                        <span style="font-size: 0.55rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">IMPACTO EV UN.:</span>
                        <strong style="font-family: monospace; font-size: 0.8rem; color: #1e3a8a;">R$ {sliceEV.toFixed(2)}</strong>
                    </div>
                </div>
            {/each}
        </div>

        <button on:click={onSaveRouletteSettings} style="width: 100%; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.75rem; font-weight: 700; text-transform: uppercase; padding: 12px; border: 1px solid #000; cursor: pointer;">
            SALVAR MATRIZ DE PROBABILIDADES E REGRAS DA ROLETA
        </button>
    </div>

</div>
