<script>
    import { gestaoService } from '../../../services/gestaoService.js';

    export let users = [];
    export let transactions = [];
    export let gestaoConfig = {};

    export let onSaveCommission = () => {};
    export let onToggleReferralSuspension = () => {};

    $: networkMetrics = gestaoService.getAffiliateNetworkMetrics(users, transactions);
</script>

<!-- ABA 6: REGRAS DE COMISSÃO & REDE DE AFILIADOS DESACOPLADA (APROFUNDADA) -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- PanelHeader com KPIs da Rede de Afiliados -->
    <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px;">
        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #1e3a8a; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">TOTAL COMISSÕES PAGAS</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #1e3a8a; margin-top: 2px;">
                R$ {networkMetrics.totalCommissionsPaid ? networkMetrics.totalCommissionsPaid.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
            <span style="font-size: 0.62rem; color: #64748b;">Repassado à rede de afiliados</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #059669; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">TAXA DE INDICAÇÃO DIRETA</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #065f46; margin-top: 2px;">
                {gestaoConfig.referralCommissionPercent || 5.0}%
            </div>
            <span style="font-size: 0.62rem; color: #065f46; font-weight: 700;">BÔNUS AUTOMÁTICO</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #b45309; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">META DE MUTIRÃO DE REDE</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b45309; margin-top: 2px;">
                {gestaoConfig.requiredReferralsCount || 3} INDICADOS
            </div>
            <span style="font-size: 0.62rem; color: #64748b;">Prêmio Mutirão: R$ {gestaoConfig.bonusPerMutiraoAmount || 50}</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #b91c1c; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">COMISSÕES SUSPENSAS</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b91c1c; margin-top: 2px;">
                {networkMetrics.suspendedCount || 0} LÍDERES
            </div>
            <span style="font-size: 0.62rem; color: #b91c1c; font-weight: 700;">RESTRITOS POR FRAUDE/MULTICONTA</span>
        </div>
    </div>

    <!-- Painel de Parâmetros Táticos de Rede & Virabilidade -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <div style="border-bottom: 1px solid #e2e8f0; padding-bottom: 8px; margin-bottom: 14px; display: flex; justify-content: space-between; align-items: center;">
            <h3 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">REGRAS DO PROGRAMA DE AFILIADOS & TRAVAS DE SEGURANÇA</h3>
            <span style="font-family: monospace; font-size: 0.65rem; color: #64748b;">SISTEMA VIRAL RESTRITO A OPERADORES QUALIFICADOS</span>
        </div>

        <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; margin-bottom: 12px;">
            <div>
                <label for="comm-rt-input" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 4px;">COMISSÃO POR INDICAÇÃO DIRETA (%):</label>
                <input id="comm-rt-input" type="number" step="0.5" bind:value={gestaoConfig.referralCommissionPercent} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.85rem; font-weight: 700; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="req-ref-cnt" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 4px;">MÍNIMO DE INDICADOS EXIGIDOS:</label>
                <input id="req-ref-cnt" type="number" min="0" max="10" bind:value={gestaoConfig.requiredReferralsCount} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.85rem; font-weight: 700; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="bns-mtr-amt" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 4px;">BÔNUS FIXO MUTIRÃO (R$):</label>
                <input id="bns-mtr-amt" type="number" step="10" bind:value={gestaoConfig.bonusPerMutiraoAmount} style="width: 100%; padding: 8px; font-family: monospace; font-size: 0.85rem; font-weight: 700; color: #065f46; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>
        </div>

        <div style="display: flex; justify-content: space-between; align-items: center; background: #f8fafc; border: 1px solid #e2e8f0; padding: 12px; margin-bottom: 14px; flex-wrap: wrap; gap: 12px;">
            <div style="display: flex; align-items: center; gap: 8px;">
                <input id="chk-3-ref" type="checkbox" bind:checked={gestaoConfig.requireReferralsForWithdrawal} style="width: 16px; height: 16px;" />
                <label for="chk-3-ref" style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #0f172a; cursor: pointer;">EXIGIR INDICADOS QUALIFICADOS PARA LIBERAR SAQUES DOS CLIENTES</label>
            </div>

            <div style="display: flex; align-items: center; gap: 8px;">
                <input id="chk-anti-self" type="checkbox" bind:checked={gestaoConfig.antiSelfReferralLock} style="width: 16px; height: 16px;" />
                <label for="chk-anti-self" style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #b91c1c; cursor: pointer;">TRAVA ANTI-MULTICONTA DE AFILIADO (IP/CHAVE PIX RESTRITOS)</label>
            </div>
        </div>

        <button on:click={onSaveCommission} style="width: 100%; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.75rem; font-weight: 700; text-transform: uppercase; padding: 12px; border: 1px solid #000; cursor: pointer;">
            SALVAR PARÂMETROS E REGRAS DA REDE DE AFILIADOS
        </button>
    </div>

    <!-- Tabela Densa B2G de Líderes de Rede e Desempenho de Afiliados -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 12px 0;">ÁRVORE DE AFILIADOS & DESEMPENHO DOS LÍDERES</h4>

        <div style="overflow-x: auto;">
            <table style="width: 100%; border-collapse: collapse; font-size: 0.75rem;">
                <thead>
                    <tr style="border-bottom: 1px solid #e2e8f0; background: #f8fafc; text-align: left; font-family: monospace; font-size: 0.6rem; color: #475569; text-transform: uppercase;">
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">LÍDER PROTOCOLO</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">NOME / CONTATO</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">CARREIRA</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">INDICADOS ATIVOS</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">RECEITA GERADA PARA BANCA</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">COMISSÃO ACUMULADA</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">STATUS DA MENSALIDADE</th>
                        <th style="padding: 8px 12px; text-align: right;">AÇÕES DE AUDITORIA DE REDE</th>
                    </tr>
                </thead>
                <tbody style="font-family: monospace;">
                    {#each networkMetrics.leadersList as leader}
                        <tr style="border-bottom: 1px solid #f1f5f9;">
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 700; color: #1e3a8a;">
                                #{leader.id}
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                                <strong style="font-family: sans-serif; display: block; color: #0f172a;">{leader.username.toUpperCase()}</strong>
                                <span style="font-size: 0.62rem; color: #64748b;">{leader.phone}</span>
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                                <span style="background: #f1f5f9; border: 1px solid #cbd5e1; padding: 2px 6px; font-size: 0.6rem; font-weight: 700;">NÍVEL {leader.nivel}</span>
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 700; color: #0f172a;">
                                {leader.directReferralsCount} Indicados
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 700; color: #065f46;">
                                R$ {leader.totalVolumeGenerated.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 900; color: #1e3a8a;">
                                R$ {leader.userCommissions.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                                <span style="font-size: 0.55rem; padding: 2px 6px; font-weight: 800; border: 1px solid {leader.referralBonusSuspended ? '#b45309' : '#047857'}; background: {leader.referralBonusSuspended ? '#fffbeb' : '#ecfdf5'}; color: {leader.referralBonusSuspended ? '#b45309' : '#065f46'};">
                                    {leader.referralBonusSuspended ? 'SUSPENSA POR FRAUDE' : 'REDE ATIVA'}
                                </span>
                            </td>
                            <td style="padding: 8px 12px; text-align: right;">
                                <button 
                                    on:click={() => onToggleReferralSuspension(leader)}
                                    style="padding: 4px 10px; font-size: 0.65rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: {leader.referralBonusSuspended ? '#047857' : '#b91c1c'}; color: #fff; cursor: pointer; text-transform: uppercase;"
                                >
                                    {leader.referralBonusSuspended ? 'REATIVAR COMISSÕES' : 'SUSPENDER BÔNUS DA REDE'}
                                </button>
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    </div>

</div>
