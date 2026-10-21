<script>
    import { onMount } from 'svelte';
    import { api } from '../../../services/api.js';

    export let machines = []; // Catálogo oficial de frotas
    export let gestaoConfig = {};
    export let onShowToast = () => {};

    let levels = [];
    let loading = true;
    let savingLevel = null;
    let selectedTier = 'all'; // 'all' | 'tier1' | 'tier2' | 'tier3'
    let showResetModal = false;
    let resetDurationDays = 75;
    let resetPinInput = '';
    let resetting = false;

    // Manutenção e Atividade do Nível 12 (Nível Máximo)
    let level12MaintenanceDays = 30;
    let level12RequiredReferrals = 1;
    let savingLevel12 = false;

    onMount(async () => {
        await loadLevels();
    });

    $: if (gestaoConfig) {
        if (gestaoConfig.level12MaintenanceDays) level12MaintenanceDays = gestaoConfig.level12MaintenanceDays;
        if (gestaoConfig.level12RequiredReferrals) level12RequiredReferrals = gestaoConfig.level12RequiredReferrals;
    }

    async function handleSaveLevel12Maintenance() {
        savingLevel12 = true;
        try {
            await api.updateLevel12Maintenance(parseInt(level12MaintenanceDays) || 30, parseInt(level12RequiredReferrals) || 1);
            onShowToast('Regras de atividade e manutenção do Nível 12 salvas com sucesso!', 'win');
        } catch (err) {
            onShowToast(`Erro ao salvar regras do Nível 12: ${err.message}`, 'error');
        } finally {
            savingLevel12 = false;
        }
    }

    async function loadLevels() {
        loading = true;
        try {
            const data = await api.getLevels();
            if (Array.isArray(data)) {
                levels = data.sort((a, b) => a.level - b.level);
            }
        } catch (err) {
            onShowToast(`Erro ao carregar níveis: ${err.message}`, 'error');
        } finally {
            loading = false;
        }
    }

    async function handleSaveLevel(lvl) {
        savingLevel = lvl.level;
        try {
            // Se uma máquina do catálogo foi selecionada, vincular o nome correto
            if (lvl.unlockedMachineId && machines.length > 0) {
                const found = machines.find(m => m.id === lvl.unlockedMachineId);
                if (found) {
                    lvl.unlockedMachineName = found.name;
                }
            } else if (!lvl.unlockedMachineId) {
                lvl.unlockedMachineName = '';
            }

            const payload = {
                ...lvl,
                requiredReferrals: parseInt(lvl.requiredReferrals) || 0,
                requiredMachines: parseInt(lvl.requiredMachines) || 0,
                requiredColetas: parseInt(lvl.requiredColetas) || 0,
                requiredTonnage: parseFloat(lvl.requiredTonnage) || 0,
                commissionPercent: parseFloat(lvl.commissionPercent) || 0,
                bonusYieldPercent: parseFloat(lvl.bonusYieldPercent) || 0,
                insuranceDaysRecovered: parseInt(lvl.insuranceDaysRecovered) || 0,
                maxMachinesAllowed: parseInt(lvl.maxMachinesAllowed) || 15
            };

            await api.updateLevel(lvl.level, payload);
            onShowToast(`Regras do Nível ${lvl.level} (${lvl.title}) salvas com sucesso!`, 'win');
        } catch (err) {
            onShowToast(`Falha ao salvar Nível ${lvl.level}: ${err.message}`, 'error');
        } finally {
            savingLevel = null;
        }
    }

    async function handleResetSeason() {
        if (resetPinInput !== 'REINICIAR') {
            onShowToast('Digite REINICIAR para confirmar.', 'error');
            return;
        }

        resetting = true;
        try {
            await api.resetSeason(parseInt(resetDurationDays) || 75);
            onShowToast('Temporada de Ranking reiniciada com sucesso! Níveis dos operadores mantidos intactos.', 'win');
            showResetModal = false;
            resetPinInput = '';
            await loadLevels();
        } catch (err) {
            onShowToast(`Erro ao reiniciar temporada: ${err.message}`, 'error');
        } finally {
            resetting = false;
        }
    }

    $: filteredLevels = levels.filter(l => {
        if (selectedTier === 'tier1') return l.level >= 1 && l.level <= 5;
        if (selectedTier === 'tier2') return l.level >= 6 && l.level <= 10;
        if (selectedTier === 'tier3') return l.level >= 11 && l.level <= 12;
        return true;
    });

    // Informações da temporada atual a partir do systemConfig
    $: seasonNum = gestaoConfig?.currentSeasonNumber || 1;
    $: seasonDays = gestaoConfig?.seasonDurationDays || 75;
</script>

<div style="display: flex; flex-direction: column; gap: 20px; font-family: monospace;">

    <!-- 1. BANNER PRINCIPAL DO PLANO DE CARREIRA & CONTROLE DA TEMPORADA DE RANKING (75 DIAS) -->
    <div style="background: #0f172a; border: 2px solid #ffb200; padding: 20px; box-shadow: 0 4px 20px rgba(0,0,0,0.5);">
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 16px;">
            <div>
                <div style="display: flex; align-items: center; gap: 10px;">
                    <span style="font-size: 1.6rem;">🎖️</span>
                    <div>
                        <h2 style="margin: 0; font-size: 1.15rem; font-weight: 900; color: #ffb200; text-transform: uppercase; letter-spacing: 0.5px;">
                            PLANO DE CARREIRA PROFISSIONAL CATBRASIL (12 NÍVEIS PERPÉTUOS)
                        </h2>
                        <span style="font-size: 0.75rem; color: #94a3b8; display: block; margin-top: 2px;">
                            O nível do operador é permanente e nunca regride. A temporada de {seasonDays} dias (2 meses e meio) é exclusiva para rotação do Ranking Semanal / Pódio.
                        </span>
                    </div>
                </div>
            </div>

            <div style="display: flex; align-items: center; gap: 12px;">
                <div style="background: #1e293b; border: 1px solid #334155; padding: 8px 14px; text-align: right;">
                    <span style="font-size: 0.62rem; color: #94a3b8; display: block; text-transform: uppercase;">CICLO DE RANKING</span>
                    <strong style="font-size: 0.95rem; color: #38bdf8;">{seasonDays} DIAS (TEMP. {seasonNum})</strong>
                </div>

                <button 
                    on:click={() => showResetModal = true}
                    style="background: #991b1b; color: #fef2f2; border: 1px solid #ef4444; padding: 10px 16px; font-family: monospace; font-size: 0.75rem; font-weight: 900; cursor: pointer; text-transform: uppercase; display: flex; align-items: center; gap: 6px;"
                >
                    <span>🔄</span> RESETAR RANKING (75D)
                </button>
            </div>
        </div>

        <!-- REGRAS GERAIS E TIERS EXPLICATIVOS -->
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 12px; margin-top: 16px;">
            <div style="background: #1e293b; border-left: 4px solid #3b82f6; padding: 12px;">
                <strong style="color: #60a5fa; font-size: 0.8rem; display: block;">TIER 1 (NÍVEIS 1 A 5) — OPERADOR</strong>
                <span style="font-size: 0.7rem; color: #cbd5e1; display: block; margin-top: 4px; line-height: 1.4;">
                    Meta: até 8 convidados e 5 máquinas. Libera frotas rápidas. <strong style="color: #fbbf24;">No Nível 5: Vale Seguro CAT</strong> (recupera 2 dias de coletas perdidas).
                </span>
            </div>

            <div style="background: #1e293b; border-left: 4px solid #a855f7; padding: 12px;">
                <strong style="color: #c084fc; font-size: 0.8rem; display: block;">TIER 2 (NÍVEIS 6 A 10) — GERÊNCIA DE MINA</strong>
                <span style="font-size: 0.7rem; color: #cbd5e1; display: block; margin-top: 4px; line-height: 1.4;">
                    Meta: até 12 convidados e 8 máquinas. 1 giro de roleta a cada promoção. Desbloqueio de frotas raras (dobro de rendimento) e comissão até 8%.
                </span>
            </div>

            <div style="background: #1e293b; border-left: 4px solid #eab308; padding: 12px;">
                <strong style="color: #facc15; font-size: 0.8rem; display: block;">TIER 3 (NÍVEIS 11 E 12) — ELITE SUPREMA</strong>
                <span style="font-size: 0.7rem; color: #cbd5e1; display: block; margin-top: 4px; line-height: 1.4;">
                    Meta: 15 convidados e 12 máquinas. Smartphone ou R$ 3.500 PIX, comissão de 10%, Roleta Diamante, 25 máquinas e Placa no Hall da Fama!
                </span>
            </div>
        </div>

        <!-- BLOCO EXCLUSIVO: MANUTENÇÃO & ATIVIDADE DO NÍVEL 12 -->
        <div style="background: #020617; border: 1px solid #eab308; padding: 16px; margin-top: 16px; border-radius: 2px;">
            <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; margin-bottom: 12px;">
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="font-size: 1.3rem;">👑</span>
                    <div>
                        <strong style="font-size: 0.85rem; color: #facc15; text-transform: uppercase;">
                            MANUTENÇÃO DE ATIVIDADE & BÔNUS DO NÍVEL 12 (NÍVEL MÁXIMO)
                        </strong>
                        <span style="font-size: 0.7rem; color: #94a3b8; display: block;">
                            Regra: Se o operador atingir o Nível 12 e parar de convidar, ele é notificado e após o prazo perde temporariamente os bônus até chamar novo membro.
                        </span>
                    </div>
                </div>
            </div>

            <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 14px; align-items: flex-end;">
                <div>
                    <label for="level12-maintenance-days" style="font-size: 0.7rem; color: #cbd5e1; display: block; margin-bottom: 4px; font-weight: 700;">
                        ⏳ DIAS DE TOLERÂNCIA SEM CONVITES:
                    </label>
                    <input 
                        id="level12-maintenance-days"
                        type="number" 
                        min="5" 
                        max="180" 
                        bind:value={level12MaintenanceDays}
                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #ffb200; font-weight: 700; padding: 8px 10px; font-family: monospace; font-size: 0.82rem;"
                    />
                    <span style="font-size: 0.62rem; color: #64748b; display: block; margin-top: 2px;">
                        Janela para trazer novos operadores (Padrão: 30 dias).
                    </span>
                </div>

                <div>
                    <label for="level12-required-referrals" style="font-size: 0.7rem; color: #cbd5e1; display: block; margin-bottom: 4px; font-weight: 700;">
                        👥 NOVOS CONVIDADOS EXIGIDOS NO PERÍODO:
                    </label>
                    <input 
                        id="level12-required-referrals"
                        type="number" 
                        min="1" 
                        max="10" 
                        bind:value={level12RequiredReferrals}
                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #ffb200; font-weight: 700; padding: 8px 10px; font-family: monospace; font-size: 0.82rem;"
                    />
                    <span style="font-size: 0.62rem; color: #64748b; display: block; margin-top: 2px;">
                        Mínimo de novos cadastrados para manter bônus ativos (Padrão: 1).
                    </span>
                </div>

                <div>
                    <button 
                        on:click={handleSaveLevel12Maintenance}
                        disabled={savingLevel12}
                        style="width: 100%; background: #eab308; color: #000; border: 1px solid #000; padding: 9px 14px; font-family: monospace; font-size: 0.75rem; font-weight: 900; cursor: pointer; text-transform: uppercase;"
                    >
                        {savingLevel12 ? 'SALVANDO...' : '💾 SALVAR REGRAS DO NÍVEL 12'}
                    </button>
                </div>
            </div>
        </div>
    </div>

    <!-- 2. FILTROS DE NAVEGAÇÃO ENTRE TIERS -->
    <div style="display: flex; gap: 8px; flex-wrap: wrap; align-items: center; justify-content: space-between;">
        <div style="display: flex; gap: 8px; flex-wrap: wrap;">
            <button 
                on:click={() => selectedTier = 'all'}
                style="background: {selectedTier === 'all' ? '#ffb200' : '#1e293b'}; color: {selectedTier === 'all' ? '#000' : '#cbd5e1'}; border: 1px solid #334155; padding: 8px 14px; font-family: monospace; font-size: 0.75rem; font-weight: 700; cursor: pointer; text-transform: uppercase;"
            >
                TODOS OS 12 NÍVEIS ({levels.length})
            </button>
            <button 
                on:click={() => selectedTier = 'tier1'}
                style="background: {selectedTier === 'tier1' ? '#3b82f6' : '#1e293b'}; color: {selectedTier === 'tier1' ? '#fff' : '#cbd5e1'}; border: 1px solid #334155; padding: 8px 14px; font-family: monospace; font-size: 0.75rem; font-weight: 700; cursor: pointer; text-transform: uppercase;"
            >
                TIER 1 (NÍVEIS 1 - 5)
            </button>
            <button 
                on:click={() => selectedTier = 'tier2'}
                style="background: {selectedTier === 'tier2' ? '#a855f7' : '#1e293b'}; color: {selectedTier === 'tier2' ? '#fff' : '#cbd5e1'}; border: 1px solid #334155; padding: 8px 14px; font-family: monospace; font-size: 0.75rem; font-weight: 700; cursor: pointer; text-transform: uppercase;"
            >
                TIER 2 (NÍVEIS 6 - 10)
            </button>
            <button 
                on:click={() => selectedTier = 'tier3'}
                style="background: {selectedTier === 'tier3' ? '#eab308' : '#1e293b'}; color: {selectedTier === 'tier3' ? '#000' : '#cbd5e1'}; border: 1px solid #334155; padding: 8px 14px; font-family: monospace; font-size: 0.75rem; font-weight: 700; cursor: pointer; text-transform: uppercase;"
            >
                TIER 3 (NÍVEIS 11 - 12)
            </button>
        </div>

        <button 
            on:click={loadLevels}
            style="background: #1e293b; color: #94a3b8; border: 1px solid #334155; padding: 8px 12px; font-family: monospace; font-size: 0.72rem; cursor: pointer;"
        >
            🔄 RECARREGAR REGRAS
        </button>
    </div>

    <!-- 3. LISTA DOS 12 NÍVEIS COM FORMULÁRIOS COMPLETOS DE EDIÇÃO -->
    {#if loading}
        <div style="background: #0f172a; border: 1px solid #1e293b; padding: 40px; text-align: center; color: #94a3b8;">
            Carregando tabela de configuração dos 12 níveis...
        </div>
    {:else}
        <div style="display: flex; flex-direction: column; gap: 16px;">
            {#each filteredLevels as lvl (lvl.level)}
                <div style="background: #0f172a; border: 1px solid {lvl.level === 12 ? '#eab308' : lvl.level === 5 ? '#3b82f6' : '#1e293b'}; box-shadow: 0 2px 10px rgba(0,0,0,0.3);">
                    
                    <!-- Topo do Card do Nível -->
                    <div style="background: #1e293b; padding: 12px 16px; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px; border-bottom: 1px solid #334155;">
                        <div style="display: flex; align-items: center; gap: 12px;">
                            <span style="font-size: 1.5rem; background: #0f172a; width: 42px; height: 42px; display: flex; align-items: center; justify-content: center; border: 1px solid #334155;">
                                {lvl.badgeIcon || '🚜'}
                            </span>
                            <div>
                                <div style="display: flex; align-items: center; gap: 8px;">
                                    <span style="font-size: 0.65rem; font-weight: 900; padding: 2px 6px; background: #ffb200; color: #000;">
                                        NÍVEL {lvl.level}
                                    </span>
                                    <span style="font-size: 0.65rem; color: #94a3b8; text-transform: uppercase;">
                                        {lvl.tierName}
                                    </span>
                                </div>
                                <h3 style="margin: 2px 0 0 0; font-size: 0.95rem; font-weight: 700; color: #f8fafc; text-transform: uppercase;">
                                    {lvl.title}
                                </h3>
                            </div>
                        </div>

                        <!-- Pílulas de Resumo Rápido -->
                        <div style="display: flex; gap: 8px; flex-wrap: wrap; font-size: 0.7rem;">
                            <span style="background: #0f172a; border: 1px solid #334155; padding: 4px 8px; color: #38bdf8;">
                                👥 {lvl.requiredReferrals} Convidados
                            </span>
                            <span style="background: #0f172a; border: 1px solid #334155; padding: 4px 8px; color: #fbbf24;">
                                🚜 {lvl.requiredMachines} Máquinas
                            </span>
                            <span style="background: #0f172a; border: 1px solid #334155; padding: 4px 8px; color: #4ade80;">
                                💰 {lvl.commissionPercent}% Comissão
                            </span>
                            {#if lvl.hasInsuranceVoucher}
                                <span style="background: #1e3a8a; border: 1px solid #3b82f6; padding: 4px 8px; color: #93c5fd; font-weight: 700;">
                                    🛡️ Vale Seguro ({lvl.insuranceDaysRecovered}d)
                                </span>
                            {/if}
                            {#if lvl.hasExtraRouletteSpin}
                                <span style="background: #581c87; border: 1px solid #a855f7; padding: 4px 8px; color: #e9d5ff;">
                                    🎰 Giro Extra
                                </span>
                            {/if}
                            {#if lvl.hasVipRoulette}
                                <span style="background: #713f12; border: 1px solid #eab308; padding: 4px 8px; color: #fef08a; font-weight: 700;">
                                    💎 Roleta VIP
                                </span>
                            {/if}
                        </div>
                    </div>

                    <!-- Formulário de Edição Completa -->
                    <div style="padding: 16px; display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 16px;">
                        
                        <!-- Coluna 1: Missões Obrigatórias para Avançar -->
                        <div style="background: #090d16; border: 1px solid #1e293b; padding: 14px; display: flex; flex-direction: column; gap: 12px;">
                            <div style="border-bottom: 1px solid #1e293b; padding-bottom: 6px;">
                                <strong style="font-size: 0.75rem; color: #ffb200; text-transform: uppercase;">
                                    🎯 1. REQUISITOS OBRIGATÓRIOS (MISSÕES DO NÍVEL)
                                </strong>
                                <span style="font-size: 0.65rem; color: #94a3b8; display: block;">
                                    O operador só avança se cumprir rigorosamente 100% destes itens.
                                </span>
                            </div>

                            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                                <div>
                                    <label for="lvl-{lvl.level}-title" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">TÍTULO DO NÍVEL</label>
                                    <input 
                                        id="lvl-{lvl.level}-title"
                                        type="text" 
                                        bind:value={lvl.title}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                    />
                                </div>
                                <div>
                                    <label for="lvl-{lvl.level}-tier-name" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">NOME DO TIER</label>
                                    <input 
                                        id="lvl-{lvl.level}-tier-name"
                                        type="text" 
                                        bind:value={lvl.tierName}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                    />
                                </div>
                            </div>

                            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                                <div>
                                    <label for="lvl-{lvl.level}-required-referrals" style="font-size: 0.68rem; color: #38bdf8; display: block; margin-bottom: 4px; font-weight: 700;">
                                        👥 CONVIDADOS ATIVOS
                                    </label>
                                    <input 
                                        id="lvl-{lvl.level}-required-referrals"
                                        type="number" 
                                        min="0"
                                        bind:value={lvl.requiredReferrals}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #0284c7; color: #38bdf8; padding: 7px 9px; font-family: monospace; font-size: 0.8rem; font-weight: 700;"
                                    />
                                </div>
                                <div>
                                    <label for="lvl-{lvl.level}-required-machines" style="font-size: 0.68rem; color: #fbbf24; display: block; margin-bottom: 4px; font-weight: 700;">
                                        🚜 MÁQUINAS ATIVAS
                                    </label>
                                    <input 
                                        id="lvl-{lvl.level}-required-machines"
                                        type="number" 
                                        min="0"
                                        bind:value={lvl.requiredMachines}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #d97706; color: #fbbf24; padding: 7px 9px; font-family: monospace; font-size: 0.8rem; font-weight: 700;"
                                    />
                                </div>
                            </div>

                            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                                <div>
                                    <label for="lvl-{lvl.level}-required-coletas" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">COLETAS MANUAIS</label>
                                    <input 
                                        id="lvl-{lvl.level}-required-coletas"
                                        type="number" 
                                        min="0"
                                        bind:value={lvl.requiredColetas}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                    />
                                </div>
                                <div>
                                    <label for="lvl-{lvl.level}-required-tonnage" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">TONELAGEM NO RANKING (T)</label>
                                    <input 
                                        id="lvl-{lvl.level}-required-tonnage"
                                        type="number" 
                                        min="0"
                                        step="100"
                                        bind:value={lvl.requiredTonnage}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                    />
                                </div>
                            </div>

                            <div style="display: grid; grid-template-columns: 80px 1fr; gap: 10px;">
                                <div>
                                    <label for="lvl-{lvl.level}-badge-icon" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">ÍCONE</label>
                                    <input 
                                        id="lvl-{lvl.level}-badge-icon"
                                        type="text" 
                                        bind:value={lvl.badgeIcon}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem; text-align: center;"
                                    />
                                </div>
                                <div>
                                    <label for="lvl-{lvl.level}-badge-name" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">NOME DO BADGE</label>
                                    <input 
                                        id="lvl-{lvl.level}-badge-name"
                                        type="text" 
                                        bind:value={lvl.badgeName}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                    />
                                </div>
                            </div>
                        </div>

                        <!-- Coluna 2: Premiações & Vantagens ao Desbloquear -->
                        <div style="background: #090d16; border: 1px solid #1e293b; padding: 14px; display: flex; flex-direction: column; gap: 12px;">
                            <div style="border-bottom: 1px solid #1e293b; padding-bottom: 6px;">
                                <strong style="font-size: 0.75rem; color: #4ade80; text-transform: uppercase;">
                                    🎁 2. PREMIAÇÕES & BENEFÍCIOS AO PASSAR DE NÍVEL
                                </strong>
                                <span style="font-size: 0.65rem; color: #94a3b8; display: block;">
                                    Bônus creditados instantaneamente no perfil do operador na promoção.
                                </span>
                            </div>

                            <!-- Seleção da Máquina Desbloqueada do Catálogo -->
                            <div>
                                <label for="lvl-{lvl.level}-unlocked-machine" style="font-size: 0.68rem; color: #fbbf24; display: block; margin-bottom: 4px; font-weight: 700;">
                                    🚜 MÁQUINA DESBLOQUEADA DO CATÁLOGO DE FROTAS
                                </label>
                                <select 
                                    id="lvl-{lvl.level}-unlocked-machine"
                                    bind:value={lvl.unlockedMachineId}
                                    style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #d97706; color: #fbbf24; padding: 7px 9px; font-family: monospace; font-size: 0.75rem; font-weight: 700;"
                                >
                                    <option value="">[ NENHUMA MÁQUINA ESPECÍFICA LIBERADA ]</option>
                                    {#each machines as mach}
                                        <option value={mach.id}>
                                            {mach.name} — ({mach.category}) | Preço: R$ {mach.price} | R$ {mach.dailyYield}/dia
                                        </option>
                                    {/each}
                                </select>
                            </div>

                            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                                <div>
                                    <label for="lvl-{lvl.level}-commission" style="font-size: 0.68rem; color: #4ade80; display: block; margin-bottom: 4px; font-weight: 700;">
                                        💰 COMISSÃO DE AFILIADOS (%)
                                    </label>
                                    <input 
                                        id="lvl-{lvl.level}-commission"
                                        type="number" 
                                        min="0"
                                        max="100"
                                        step="0.5"
                                        bind:value={lvl.commissionPercent}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #16a34a; color: #4ade80; padding: 7px 9px; font-family: monospace; font-size: 0.8rem; font-weight: 700;"
                                    />
                                </div>
                                <div>
                                    <label for="lvl-{lvl.level}-bonus-yield" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">
                                        ⚡ BÔNUS RENDIMENTO DIÁRIO (%)
                                    </label>
                                    <input 
                                        id="lvl-{lvl.level}-bonus-yield"
                                        type="number" 
                                        min="0"
                                        max="50"
                                        step="0.5"
                                        bind:value={lvl.bonusYieldPercent}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                    />
                                </div>
                            </div>

                            <!-- Vale Seguro CAT (Nível 5) -->
                            <div style="background: #0f172a; border: 1px solid {lvl.hasInsuranceVoucher ? '#3b82f6' : '#334155'}; padding: 10px;">
                                <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                                    <input type="checkbox" bind:checked={lvl.hasInsuranceVoucher} />
                                    <span style="font-size: 0.72rem; color: #93c5fd; font-weight: 700;">
                                        🛡️ CONCEDER VALE SEGURO CAT (RESGATE DE DIAS PERDIDOS)
                                    </span>
                                </label>
                                {#if lvl.hasInsuranceVoucher}
                                    <div style="margin-top: 8px; display: flex; align-items: center; gap: 8px;">
                                        <span style="font-size: 0.68rem; color: #cbd5e1;">DIAS DE RENDIMENTO RESGATADOS:</span>
                                        <input 
                                            type="number" 
                                            min="1" 
                                            max="7" 
                                            bind:value={lvl.insuranceDaysRecovered}
                                            style="width: 60px; background: #1e293b; border: 1px solid #3b82f6; color: #93c5fd; padding: 4px 6px; font-family: monospace; font-size: 0.75rem; text-align: center;"
                                        />
                                    </div>
                                {/if}
                            </div>

                            <!-- Roleta e Limite de Máquinas -->
                            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                                <div style="display: flex; flex-direction: column; gap: 6px;">
                                    <label style="display: flex; align-items: center; gap: 6px; cursor: pointer;">
                                        <input type="checkbox" bind:checked={lvl.hasExtraRouletteSpin} />
                                        <span style="font-size: 0.68rem; color: #c084fc;">🎰 GIRO NA ROLETA</span>
                                    </label>
                                    <label style="display: flex; align-items: center; gap: 6px; cursor: pointer;">
                                        <input type="checkbox" bind:checked={lvl.hasVipRoulette} />
                                        <span style="font-size: 0.68rem; color: #facc15; font-weight: 700;">💎 ROLETA VIP</span>
                                    </label>
                                </div>
                                <div>
                                    <label for="lvl-{lvl.level}-max-machines" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">LIMITE DE MÁQUINAS</label>
                                    <input 
                                        id="lvl-{lvl.level}-max-machines"
                                        type="number" 
                                        min="10" 
                                        max="50"
                                        bind:value={lvl.maxMachinesAllowed}
                                        style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                    />
                                </div>
                            </div>

                            <!-- Texto de Premiação Especial Física -->
                            <div>
                                <label for="lvl-{lvl.level}-special-reward" style="font-size: 0.68rem; color: #94a3b8; display: block; margin-bottom: 4px;">
                                    📱 PREMIAÇÃO ESPECIAL / FÍSICA (EX: SMARTPHONE, PLACA)
                                </label>
                                <input 
                                    id="lvl-{lvl.level}-special-reward"
                                    type="text" 
                                    bind:value={lvl.specialRewardText}
                                    placeholder="Ex: Smartphone de Última Geração ou R$ 3.500 PIX"
                                    style="width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155; color: #fff; padding: 7px 9px; font-family: monospace; font-size: 0.75rem;"
                                />
                            </div>
                        </div>
                    </div>

                    <!-- Rodapé de Ação do Card -->
                    <div style="padding: 10px 16px; background: #1e293b; border-top: 1px solid #334155; display: flex; justify-content: flex-end; align-items: center; gap: 10px;">
                        <button 
                            on:click={() => handleSaveLevel(lvl)}
                            disabled={savingLevel === lvl.level}
                            style="background: #ffb200; color: #000; border: 1px solid #000; padding: 8px 18px; font-family: monospace; font-size: 0.75rem; font-weight: 900; cursor: pointer; text-transform: uppercase;"
                        >
                            {savingLevel === lvl.level ? 'GRAVANDO...' : `💾 SALVAR REGRAS DO NÍVEL ${lvl.level}`}
                        </button>
                    </div>

                </div>
            {/each}
        </div>
    {/if}

    <!-- MODAL DE CONFIRMAÇÃO DE REINÍCIO DE TEMPORADA (RESET SEASON) -->
    {#if showResetModal}
        <div style="position: fixed; inset: 0; background: rgba(0,0,0,0.85); display: flex; align-items: center; justify-content: center; z-index: 9999; padding: 20px;">
            <div style="background: #0f172a; border: 2px solid #ef4444; max-width: 540px; width: 100%; padding: 24px; box-shadow: 0 10px 40px rgba(0,0,0,0.8);">
                <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 14px;">
                    <span style="font-size: 1.8rem;">⚠️</span>
                    <div>
                        <h3 style="margin: 0; color: #ef4444; font-size: 1.1rem; font-weight: 900; text-transform: uppercase;">
                            CONFIRMAÇÃO DE REINÍCIO DO RANKING (75 DIAS)
                        </h3>
                        <span style="font-size: 0.7rem; color: #94a3b8;">
                            Ação de virada da Temporada de Ranking da Mineração (Ciclo de 75 dias)
                        </span>
                    </div>
                </div>

                <div style="background: #1e293b; border-left: 4px solid #ef4444; padding: 12px; margin-bottom: 16px; font-size: 0.75rem; color: #cbd5e1; line-height: 1.5;">
                    <p style="margin: 0 0 6px 0;"><strong>O que esta ação faz:</strong></p>
                    <ul style="margin: 0; padding-left: 18px;">
                        <li>Reinicia o pódio e a tonelagem da Corrida de Mineração da Temporada.</li>
                        <li>Arquiva os campeões da Temporada {seasonNum} no histórico permanente de Hall da Fama / Ranking.</li>
                        <li><strong style="color: #4ade80;">PRESERVA 100%</strong> dos níveis de carreira dos operadores (1 a 12), contratos de máquinas ativas e saldos.</li>
                        <li>Avança o ciclo da temporada de ranking para a Temporada {seasonNum + 1}.</li>
                    </ul>
                </div>

                <div style="display: flex; flex-direction: column; gap: 10px; margin-bottom: 18px;">
                    <div>
                        <label for="reset-duration-days" style="font-size: 0.7rem; color: #94a3b8; display: block; margin-bottom: 4px;">
                            DURAÇÃO DA PRÓXIMA TEMPORADA (DIAS):
                        </label>
                        <input 
                            id="reset-duration-days"
                            type="number" 
                            min="30" 
                            max="180" 
                            bind:value={resetDurationDays}
                            style="width: 100%; box-sizing: border-box; background: #1e293b; border: 1px solid #334155; color: #fff; padding: 8px 10px; font-family: monospace; font-size: 0.8rem;"
                        />
                    </div>

                    <div>
                        <label for="reset-confirm-pin" style="font-size: 0.7rem; color: #ef4444; display: block; margin-bottom: 4px; font-weight: 700;">
                            DIGITE "REINICIAR" PARA CONFIRMAR:
                        </label>
                        <input 
                            id="reset-confirm-pin"
                            type="password" 
                            bind:value={resetPinInput}
                            placeholder="Digite REINICIAR"
                            style="width: 100%; box-sizing: border-box; background: #1e293b; border: 1px solid #ef4444; color: #fff; padding: 8px 10px; font-family: monospace; font-size: 0.8rem;"
                        />
                    </div>
                </div>

                <div style="display: flex; justify-content: flex-end; gap: 10px;">
                    <button 
                        on:click={() => { showResetModal = false; resetPinInput = ''; }}
                        style="background: #1e293b; color: #cbd5e1; border: 1px solid #334155; padding: 8px 16px; font-family: monospace; font-size: 0.75rem; cursor: pointer;"
                    >
                        CANCELAR
                    </button>
                    <button 
                        on:click={handleResetSeason}
                        disabled={resetting}
                        style="background: #dc2626; color: #fff; border: 1px solid #b91c1c; padding: 8px 18px; font-family: monospace; font-size: 0.75rem; font-weight: 900; cursor: pointer;"
                    >
                        {resetting ? 'REINICIANDO...' : 'CONFIRMAR E REINICIAR RANKING'}
                    </button>
                </div>
            </div>
        </div>
    {/if}

</div>
