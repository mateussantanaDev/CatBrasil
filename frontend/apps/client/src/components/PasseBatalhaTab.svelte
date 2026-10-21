<script>
    import { onMount } from 'svelte';
    import { api } from '../services/api.js';

    export let user = {};
    export let onUpdateUser = () => {};
    export let onShowToast = () => {};
    export let onOpenLoja = () => {};
    export let onOpenInvite = () => {};

    let battlePass = null;
    let loading = true;
    let levelingUp = false;
    let recoveringInsurance = false;

    onMount(async () => {
        await loadBattlePass();
    });

    async function loadBattlePass() {
        loading = true;
        try {
            const data = await api.getBattlePass();
            if (data) {
                battlePass = data;
            }
        } catch (err) {
            console.warn('Erro ao carregar Passe de Batalha:', err.message);
        } finally {
            loading = false;
        }
    }

    async function handleLevelUp() {
        levelingUp = true;
        try {
            const res = await api.levelUp();
            if (res) {
                onShowToast(res.message || '🎉 Parabéns! Você avançou no Plano de Carreira CAT!', 'win');
                if (res.user) {
                    onUpdateUser(res.user);
                }
                await loadBattlePass();
            }
        } catch (err) {
            onShowToast(`Não foi possível avançar: ${err.message}`, 'error');
        } finally {
            levelingUp = false;
        }
    }

    async function handleRecoverInsurance() {
        recoveringInsurance = true;
        try {
            const res = await api.recoverInsurance();
            if (res) {
                onShowToast(res.message || '🛡️ Vale Seguro resgatado com sucesso!', 'win');
                if (res.user) {
                    onUpdateUser(res.user);
                }
                await loadBattlePass();
            }
        } catch (err) {
            onShowToast(`Erro ao acionar Vale Seguro: ${err.message}`, 'error');
        } finally {
            recoveringInsurance = false;
        }
    }

    $: currentLvl = battlePass?.currentLevel || user.nivel || 1;
    $: nextLvl = battlePass?.nextLevelConfig;
    $: canLevelUp = battlePass?.canLevelUp || false;
    $: missingReqs = battlePass?.missingRequirements || [];
    $: allLevels = battlePass?.allLevels || [];

    $: refCurrent = battlePass?.currentReferrals || 0;
    $: refRequired = nextLvl?.requiredReferrals || 0;
    $: refDone = refCurrent >= refRequired;

    $: machCurrent = battlePass?.currentActiveMachines || 0;
    $: machRequired = nextLvl?.requiredMachines || 0;
    $: machDone = machCurrent >= machRequired;

    $: colCurrent = battlePass?.currentColetas || 0;
    $: colRequired = nextLvl?.requiredColetas || 0;
    $: colDone = colCurrent >= colRequired;

    $: tonCurrent = battlePass?.currentTonnage || 0;
    $: tonRequired = nextLvl?.requiredTonnage || 0;
    $: tonDone = tonCurrent >= tonRequired;
</script>

<div class="tab-content active" style="width: 100%; box-sizing: border-box; padding-bottom: 30px; font-family: 'JetBrains Mono', monospace;">

    <!-- 1. BANNER DO PLANO DE CARREIRA PROFISSIONAL CATBRASIL (12 NÍVEIS PERPÉTUOS) -->
    <div style="background: #0f172a; border: 2px solid var(--cat-yellow, #ffb200); box-shadow: var(--brutal-shadow, 4px 4px 0px #000); padding: 14px; margin-bottom: 14px; border-radius: 2px; position: relative;">
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px;">
            <div style="display: flex; align-items: center; gap: 8px;">
                <span style="font-size: 1.6rem;">🎖️</span>
                <div>
                    <h2 style="margin: 0; font-size: 0.95rem; font-weight: 900; color: var(--cat-yellow, #ffb200); text-transform: uppercase; font-family: 'Oswald', sans-serif; letter-spacing: 0.5px;">
                        PLANO DE CARREIRA CATBRASIL (12 NÍVEIS)
                    </h2>
                    <span style="font-size: 0.68rem; color: #94a3b8; display: block;">
                        Progresso Perpétuo · O seu nível de operador nunca expira ou regride com o tempo
                    </span>
                </div>
            </div>

            <div style="background: #1e293b; border: 1px solid #334155; padding: 4px 10px; border-radius: 2px; text-align: right;">
                <span style="font-size: 0.6rem; color: #94a3b8; display: block; text-transform: uppercase;">STATUS DA CARREIRA</span>
                <strong style="font-size: 0.85rem; color: #4ade80;">
                    🛡️ VITALÍCIO
                </strong>
            </div>
        </div>

        <!-- MANUTENÇÃO & STATUS DO NÍVEL 12 (NÍVEL MÁXIMO) -->
        {#if user.hasMaxLevelPlaque || currentLvl >= 12 || battlePass?.isLevel12}
            <!-- Placa de Honra Hall da Fama -->
            <div style="margin-top: 10px; background: linear-gradient(135deg, #eab308 0%, #ca8a04 100%); color: #000; padding: 10px 12px; border: 2px solid #000; font-family: 'Oswald', sans-serif; display: flex; align-items: center; justify-content: space-between; gap: 8px;">
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="font-size: 1.5rem;">👑</span>
                    <div>
                        <strong style="font-size: 0.9rem; text-transform: uppercase; display: block;">
                            PLACA DE NÍVEL MÁXIMO — HALL DA FAMA CATBRASIL
                        </strong>
                        <span style="font-size: 0.68rem; font-family: 'JetBrains Mono', monospace; font-weight: 700;">
                            OPERADOR LENDA VITALÍCIO · COMISSÃO SUPREMA DE 10%
                        </span>
                    </div>
                </div>
                <span style="background: #000; color: #facc15; padding: 2px 6px; font-size: 0.65rem; font-weight: 900;">
                    NÍVEL 12 MAX
                </span>
            </div>

            <!-- Card de Alerta de Manutenção ou Suspensão de Bônus -->
            {#if battlePass?.level12BonusSuspended}
                <div style="margin-top: 10px; background: #450a0a; border: 2px solid #ef4444; color: #fecaca; padding: 12px; font-size: 0.72rem; display: flex; flex-direction: column; gap: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <span style="font-size: 1.3rem;">🛑</span>
                        <div>
                            <strong style="color: #f87171; font-size: 0.82rem; text-transform: uppercase;">
                                BÔNUS EXTRAORDINÁRIOS DO NÍVEL 12 EM PAUSA
                            </strong>
                            <span style="display: block; font-size: 0.68rem; color: #cbd5e1; margin-top: 2px;">
                                {battlePass?.level12WarningMessage || 'O período sem novos convites expirou. Sua comissão retornou temporariamente para 5% e o bônus de rendimento foi suspenso.'}
                            </span>
                        </div>
                    </div>
                    <div style="background: rgba(0,0,0,0.4); padding: 8px; border-left: 3px solid #ef4444; font-size: 0.68rem; color: #e2e8f0;">
                        💡 <strong>Como reativar:</strong> Cadastre <strong>1 novo indicado ativo</strong> com seu link de indicação para restabelecer imediatamente a comissão suprema de 10% e o bônus de produção!
                    </div>
                    <button 
                        on:click={onOpenInvite}
                        style="background: #ef4444; color: #fff; border: 1px solid #b91c1c; padding: 8px 12px; font-family: monospace; font-size: 0.72rem; font-weight: 900; cursor: pointer; text-transform: uppercase;"
                    >
                        👥 CONVIDAR OPERADOR & REATIVAR BÔNUS
                    </button>
                </div>
            {:else if battlePass?.level12WarningActive}
                <div style="margin-top: 10px; background: #422006; border: 2px solid #f59e0b; color: #fef3c7; padding: 12px; font-size: 0.72rem; display: flex; flex-direction: column; gap: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <span style="font-size: 1.3rem;">⚠️</span>
                        <div>
                            <strong style="color: #fbbf24; font-size: 0.82rem; text-transform: uppercase;">
                                ALERTA DE ATIVIDADE DO NÍVEL 12
                            </strong>
                            <span style="display: block; font-size: 0.68rem; color: #fde68a; margin-top: 2px;">
                                {battlePass?.level12WarningMessage || `Restam apenas ${battlePass?.level12MaintenanceDaysRemaining ?? 7} dias para convidar novos operadores.`}
                            </span>
                        </div>
                    </div>
                    <div style="background: rgba(0,0,0,0.4); padding: 8px; border-left: 3px solid #f59e0b; font-size: 0.68rem; color: #fde68a;">
                        Convide pelo menos 1 novo membro antes do término do prazo para manter sua taxa de 10% e rendimento extra ativos sem interrupção.
                    </div>
                    <button 
                        on:click={onOpenInvite}
                        style="background: #f59e0b; color: #000; border: 1px solid #d97706; padding: 8px 12px; font-family: monospace; font-size: 0.72rem; font-weight: 900; cursor: pointer; text-transform: uppercase;"
                    >
                        👥 CONVIDAR AGORA & GARANTIR MANUTENÇÃO
                    </button>
                </div>
            {:else}
                <div style="margin-top: 10px; background: #064e3b; border: 1px solid #10b981; color: #d1fae5; padding: 8px 12px; font-size: 0.7rem; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap;">
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <span>✅</span>
                        <span><strong>NÍVEL 12 ATIVO & PROTEGIDO:</strong> Bônus e comissão máxima de 10% 100% ativos.</span>
                    </div>
                    {#if battlePass?.level12MaintenanceDaysRemaining}
                        <span style="font-size: 0.65rem; color: #a7f3d0; font-family: monospace;">
                            Próxima checagem: {battlePass.level12MaintenanceDaysRemaining} dias
                        </span>
                    {/if}
                </div>
            {/if}
        {/if}
    </div>

    <!-- 2. CARD DO NÍVEL ATUAL & VANTAGENS ATIVAS -->
    <div style="background: #fff; border: 2px solid var(--cat-black, #111); box-shadow: var(--brutal-shadow, 4px 4px 0px #000); padding: 14px; margin-bottom: 14px; border-radius: 2px;">
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; border-bottom: 2px solid var(--cat-black, #111); padding-bottom: 10px; margin-bottom: 10px;">
            <div style="display: flex; align-items: center; gap: 10px;">
                <div style="width: 44px; height: 44px; background: var(--cat-yellow, #ffb200); border: 2px solid var(--cat-black, #111); display: flex; align-items: center; justify-content: center; font-size: 1.4rem;">
                    {currentLvl >= 11 ? '👑' : currentLvl >= 6 ? '⚡' : '🚜'}
                </div>
                <div>
                    <span style="font-size: 0.65rem; font-weight: 800; background: var(--cat-black, #111); color: var(--cat-yellow, #ffb200); padding: 2px 6px;">
                        NÍVEL ATUAL: {currentLvl}
                    </span>
                    <h3 style="margin: 2px 0 0 0; font-size: 1rem; font-family: 'Oswald', sans-serif; font-weight: 800; color: var(--cat-black, #111); text-transform: uppercase;">
                        {currentLvl >= 12 ? 'Comandante Supremo' : currentLvl >= 6 ? 'Gerente de Mina' : 'Operador de Frota'}
                    </h3>
                </div>
            </div>

            <!-- Vantagens Ativas -->
            <div style="display: flex; gap: 6px; flex-wrap: wrap; font-size: 0.68rem;">
                <span style="background: #ecfdf5; border: 1px solid #10b981; color: #047857; padding: 3px 6px; font-weight: 700;">
                    💰 {battlePass?.commissionPercent || 5}% Comissão
                </span>
                <span style="background: #fefce8; border: 1px solid #eab308; color: #854d0e; padding: 3px 6px; font-weight: 700;">
                    ⚡ +{battlePass?.bonusYieldPercent || 0}% Produção
                </span>
                <span style="background: #eff6ff; border: 1px solid #3b82f6; color: #1d4ed8; padding: 3px 6px; font-weight: 700;">
                    🚜 Max {battlePass?.maxMachinesAllowed || 15} Máquinas
                </span>
            </div>
        </div>

        <!-- VALE SEGURO CAT SE O USUÁRIO TIVER ACUMULADO -->
        {#if (battlePass?.insuranceVouchers || user.insuranceVouchersCount || 0) > 0}
            <div style="background: #0f172a; border: 2px solid #3b82f6; padding: 10px 12px; margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap;">
                <div>
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <span style="font-size: 1.1rem;">🛡️</span>
                        <strong style="color: #93c5fd; font-size: 0.78rem; text-transform: uppercase;">
                            VALE SEGURO CAT DISPONÍVEL ({battlePass?.insuranceVouchers || user.insuranceVouchersCount})
                        </strong>
                    </div>
                    <span style="font-size: 0.65rem; color: #cbd5e1; display: block; margin-top: 2px;">
                        Recupera até 2 dias de coletas perdidas da sua frota ativa.
                    </span>
                </div>
                <button 
                    on:click={handleRecoverInsurance}
                    disabled={recoveringInsurance}
                    style="background: #2563eb; color: #fff; border: 1px solid #1d4ed8; padding: 6px 12px; font-family: monospace; font-size: 0.7rem; font-weight: 800; cursor: pointer; text-transform: uppercase;"
                >
                    {recoveringInsurance ? 'RESGATANDO...' : 'USAR VALE SEGURO'}
                </button>
            </div>
        {/if}
    </div>

    <!-- 3. MISSÕES OBRIGATÓRIAS PARA O PRÓXIMO NÍVEL -->
    {#if currentLvl < 12 && nextLvl}
        <div style="background: #fff; border: 2px solid var(--cat-black, #111); box-shadow: var(--brutal-shadow, 4px 4px 0px #000); padding: 14px; margin-bottom: 14px; border-radius: 2px;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; flex-wrap: wrap; gap: 6px;">
                <div>
                    <span style="font-size: 0.62rem; color: #64748b; font-weight: 800; text-transform: uppercase;">PRÓXIMO NÍVEL A DESBLOQUEAR</span>
                    <h3 style="margin: 0; font-size: 1.05rem; font-family: 'Oswald', sans-serif; font-weight: 800; color: var(--cat-black, #111);">
                        {nextLvl.badgeIcon || '🚜'} NÍVEL {nextLvl.level} — {nextLvl.title}
                    </h3>
                </div>
                <span style="font-size: 0.68rem; background: var(--cat-yellow, #ffb200); color: #000; font-weight: 900; padding: 2px 6px;">
                    {nextLvl.tierName}
                </span>
            </div>

            <!-- Premiações Que Esperam no Próximo Nível -->
            <div style="background: #f8fafc; border: 1px solid #e2e8f0; padding: 10px; margin-bottom: 12px; font-size: 0.72rem;">
                <strong style="color: #0f172a; display: block; margin-bottom: 4px;">🎁 RECOMPENSAS AO ATINGIR O NÍVEL {nextLvl.level}:</strong>
                <div style="display: flex; flex-direction: column; gap: 3px; color: #334155;">
                    {#if nextLvl.unlockedMachineName}
                        <div>🚜 <strong style="color: #b45309;">Desbloqueio de Máquina:</strong> {nextLvl.unlockedMachineName}</div>
                    {/if}
                    <div>💰 <strong style="color: #047857;">Nova Comissão:</strong> {nextLvl.commissionPercent}%</div>
                    {#if nextLvl.hasInsuranceVoucher}
                        <div>🛡️ <strong style="color: #1d4ed8;">Vale Seguro CAT:</strong> Recupera {nextLvl.insuranceDaysRecovered} dias de produção perdida!</div>
                    {/if}
                    {#if nextLvl.hasExtraRouletteSpin}
                        <div>🎰 <strong style="color: #7e22ce;">Giro Extra:</strong> 1 giro grátis garantido na Roleta</div>
                    {/if}
                    {#if nextLvl.hasVipRoulette}
                        <div>💎 <strong style="color: #ca8a04;">Acesso VIP:</strong> Desbloqueio da Roleta Diamante</div>
                    {/if}
                    {#if nextLvl.specialRewardText}
                        <div>⭐ <strong style="color: #dc2626;">Prêmio Especial:</strong> {nextLvl.specialRewardText}</div>
                    {/if}
                </div>
            </div>

            <!-- CHECKLIST DAS 4 MISSÕES OBRIGATÓRIAS -->
            <div style="margin-bottom: 14px;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
                    <strong style="font-size: 0.75rem; color: #0f172a; text-transform: uppercase;">
                        MISSÕES OBRIGATÓRIAS PARA PROMOÇÃO:
                    </strong>
                    <span style="font-size: 0.65rem; color: {canLevelUp ? '#16a34a' : '#ea580c'}; font-weight: 800;">
                        {canLevelUp ? '✅ 100% CONCLUÍDAS!' : '⏳ MISSÕES PENDENTES'}
                    </span>
                </div>

                <div style="display: flex; flex-direction: column; gap: 8px;">
                    
                    <!-- Missão 1: Convidados -->
                    <div style="background: {refDone ? '#ecfdf5' : '#f8fafc'}; border: 1px solid {refDone ? '#10b981' : '#cbd5e1'}; padding: 8px 10px; display: flex; justify-content: space-between; align-items: center;">
                        <div>
                            <span style="font-size: 0.72rem; font-weight: 700; color: #0f172a;">
                                {refDone ? '✅' : '👥'} Convidar Operadores Ativos
                            </span>
                            <span style="font-size: 0.65rem; color: #64748b; display: block;">
                                Meta: {refRequired} convidados (você tem {refCurrent})
                            </span>
                        </div>
                        {#if !refDone}
                            <button 
                                on:click={onOpenInvite}
                                style="background: #0284c7; color: #fff; border: 1px solid #0369a1; padding: 4px 8px; font-size: 0.65rem; font-weight: 800; cursor: pointer;"
                            >
                                CONVIDAR
                            </button>
                        {:else}
                            <span style="font-size: 0.65rem; font-weight: 800; color: #059669;">CUMPRIDA</span>
                        {/if}
                    </div>

                    <!-- Missão 2: Máquinas Ativas -->
                    <div style="background: {machDone ? '#ecfdf5' : '#f8fafc'}; border: 1px solid {machDone ? '#10b981' : '#cbd5e1'}; padding: 8px 10px; display: flex; justify-content: space-between; align-items: center;">
                        <div>
                            <span style="font-size: 0.72rem; font-weight: 700; color: #0f172a;">
                                {machDone ? '✅' : '🚜'} Possuir Máquinas Ativas na Garagem
                            </span>
                            <span style="font-size: 0.65rem; color: #64748b; display: block;">
                                Meta: {machRequired} máquinas ativas (você tem {machCurrent})
                            </span>
                        </div>
                        {#if !machDone}
                            <button 
                                on:click={onOpenLoja}
                                style="background: var(--cat-yellow, #ffb200); color: #000; border: 1px solid #000; padding: 4px 8px; font-size: 0.65rem; font-weight: 900; cursor: pointer;"
                            >
                                ALUGAR
                            </button>
                        {:else}
                            <span style="font-size: 0.65rem; font-weight: 800; color: #059669;">CUMPRIDA</span>
                        {/if}
                    </div>

                    <!-- Missão 3: Coletas Manuais -->
                    <div style="background: {colDone ? '#ecfdf5' : '#f8fafc'}; border: 1px solid {colDone ? '#10b981' : '#cbd5e1'}; padding: 8px 10px; display: flex; justify-content: space-between; align-items: center;">
                        <div>
                            <span style="font-size: 0.72rem; font-weight: 700; color: #0f172a;">
                                {colDone ? '✅' : '⛏️'} Coletas Manuais de Rendimento
                            </span>
                            <span style="font-size: 0.65rem; color: #64748b; display: block;">
                                Meta: {colRequired} coletas (você realizou {colCurrent})
                            </span>
                        </div>
                        <span style="font-size: 0.65rem; font-weight: 800; color: {colDone ? '#059669' : '#d97706'};">
                            {colDone ? 'CUMPRIDA' : `${colCurrent} / ${colRequired}`}
                        </span>
                    </div>

                    <!-- Missão 4: Tonelagem no Ranking -->
                    {#if nextLvl.requiredTonnage > 0}
                        <div style="background: {tonDone ? '#ecfdf5' : '#f8fafc'}; border: 1px solid {tonDone ? '#10b981' : '#cbd5e1'}; padding: 8px 10px; display: flex; justify-content: space-between; align-items: center;">
                            <div>
                                <span style="font-size: 0.72rem; font-weight: 700; color: #0f172a;">
                                    {tonDone ? '✅' : '⛰️'} Tonelagem Extraída no Ranking
                                </span>
                                <span style="font-size: 0.65rem; color: #64748b; display: block;">
                                    Meta: {tonRequired.toFixed(0)}t (você produziu {tonCurrent.toFixed(0)}t)
                                </span>
                            </div>
                            <span style="font-size: 0.65rem; font-weight: 800; color: {tonDone ? '#059669' : '#d97706'};">
                                {tonDone ? 'CUMPRIDA' : `${tonCurrent.toFixed(0)}t / ${tonRequired.toFixed(0)}t`}
                            </span>
                        </div>
                    {/if}

                </div>
            </div>

            <!-- BOTÃO DE PROMOÇÃO DE NÍVEL -->
            <button 
                on:click={handleLevelUp}
                disabled={!canLevelUp || levelingUp}
                style="width: 100%; padding: 12px; font-family: 'Oswald', sans-serif; font-size: 0.95rem; font-weight: 900; letter-spacing: 0.5px; text-transform: uppercase; border: 2px solid var(--cat-black, #111); box-shadow: var(--brutal-shadow, 4px 4px 0px #000); cursor: {canLevelUp ? 'pointer' : 'not-allowed'}; background: {canLevelUp ? '#22c55e' : '#cbd5e1'}; color: {canLevelUp ? '#fff' : '#64748b'}; transition: all 0.2s;"
            >
                {#if levelingUp}
                    PROMOVENDO OPERADOR...
                {:else if canLevelUp}
                    🎉 AVANÇAR PARA O NÍVEL {nextLvl.level} AGORA!
                {:else}
                    🔒 CUMPRA AS MISSÕES ACIMA PARA AVANÇAR
                {/if}
            </button>
        </div>
    {/if}

    <!-- 4. TRILHA VISUAL DOS 12 NÍVEIS DO PLANO DE CARREIRA -->
    <div style="background: #0f172a; border: 2px solid var(--cat-black, #111); box-shadow: var(--brutal-shadow, 4px 4px 0px #000); padding: 14px; border-radius: 2px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid #334155; padding-bottom: 8px;">
            <strong style="font-size: 0.8rem; color: var(--cat-yellow, #ffb200); text-transform: uppercase;">
                🗺️ TRILHA DO PLANO DE CARREIRA (12 NÍVEIS)
            </strong>
            <span style="font-size: 0.65rem; color: #94a3b8;">
                Seu progresso: Nível {currentLvl} de 12 (Perpétuo)
            </span>
        </div>

        <div style="display: flex; flex-direction: column; gap: 8px;">
            {#each allLevels as lvl (lvl.level)}
                {@const isPassed = lvl.level < currentLvl}
                {@const isCurrent = lvl.level === currentLvl}
                {@const isLocked = lvl.level > currentLvl}

                <div style="background: {isCurrent ? '#1e293b' : '#090d16'}; border: 1px solid {isCurrent ? 'var(--cat-yellow, #ffb200)' : isPassed ? '#10b981' : '#334155'}; padding: 10px 12px; display: flex; align-items: center; justify-content: space-between; gap: 8px;">
                    <div style="display: flex; align-items: center; gap: 10px;">
                        <span style="font-size: 1.2rem; width: 32px; height: 32px; background: {isCurrent ? 'var(--cat-yellow, #ffb200)' : '#1e293b'}; color: {isCurrent ? '#000' : '#fff'}; display: flex; align-items: center; justify-content: center; border-radius: 2px;">
                            {lvl.badgeIcon || '🚜'}
                        </span>
                        <div>
                            <div style="display: flex; align-items: center; gap: 6px;">
                                <span style="font-size: 0.62rem; font-weight: 800; color: {isCurrent ? 'var(--cat-yellow, #ffb200)' : '#94a3b8'};">
                                    NÍVEL {lvl.level}
                                </span>
                                <span style="font-size: 0.58rem; color: #64748b; text-transform: uppercase;">
                                    {lvl.tierName}
                                </span>
                            </div>
                            <strong style="font-size: 0.78rem; color: {isPassed ? '#94a3b8' : isCurrent ? '#f8fafc' : '#64748b'}; display: block;">
                                {lvl.title}
                            </strong>
                            {#if lvl.unlockedMachineName}
                                <span style="font-size: 0.62rem; color: #fbbf24; display: block;">
                                    🚜 Desbloqueia: {lvl.unlockedMachineName}
                                </span>
                            {/if}
                        </div>
                    </div>

                    <div style="text-align: right;">
                        {#if isPassed}
                            <span style="font-size: 0.65rem; font-weight: 800; color: #10b981; background: #064e3b; padding: 2px 6px; border-radius: 2px;">
                                CONCLUÍDO ✓
                            </span>
                        {:else if isCurrent}
                            <span style="font-size: 0.65rem; font-weight: 900; color: #000; background: var(--cat-yellow, #ffb200); padding: 2px 6px; border-radius: 2px;">
                                NÍVEL ATUAL
                            </span>
                        {:else}
                            <span style="font-size: 0.62rem; color: #64748b; background: #1e293b; padding: 2px 6px; border-radius: 2px;">
                                🔒 BLOQUEADO
                            </span>
                        {/if}
                    </div>
                </div>
            {/each}
        </div>
    </div>

</div>
