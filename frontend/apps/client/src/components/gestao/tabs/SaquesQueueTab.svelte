<script>
    import { gestaoService } from '../../../services/gestaoService.js';

    export let users = [];
    export let transactions = [];
    export let gestaoConfig = {};
    export let queueMetrics = {};
    export let withdrawQueueFilter = "pendentes";

    export let onSaveAutoApproveSettings = () => {};
    export let onApproveWithdrawal = () => {};
    export let onRejectWithdrawal = () => {};

    $: withdrawalTxs = transactions.filter(t => t.tipo.toLowerCase().includes('saque'));

    $: filteredWithdrawals = withdrawalTxs.filter(t => {
        if (withdrawQueueFilter === 'pendentes') return t.status === 'pendente';
        if (withdrawQueueFilter === 'aprovados') return t.status === 'sucesso' || t.status === 'aprovado';
        if (withdrawQueueFilter === 'rejeitados') return t.status === 'rejeitado' || t.status === 'recusado';
        if (withdrawQueueFilter === 'alto_valor') return Math.abs(t.valor) >= 500;
        return true;
    });
</script>

<!-- ABA 3: FILA DE SAQUES PIX DESACOPLADA -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- PanelHeader com KPIs de Saques -->
    <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px;">
        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #b91c1c; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">SAQUES PENDENTES ({queueMetrics.pendingCount || 0})</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b91c1c; margin-top: 2px;">
                R$ {queueMetrics.pendingTotalVal ? queueMetrics.pendingTotalVal.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
        </div>
        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #059669; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">TOTAL APROVADO & PAGO ({queueMetrics.approvedCount || 0})</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #065f46; margin-top: 2px;">
                R$ {queueMetrics.approvedTotalVal ? queueMetrics.approvedTotalVal.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
        </div>
        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #b45309; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">TOTAL REJEITADO ({queueMetrics.rejectedCount || 0})</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b45309; margin-top: 2px;">
                R$ {queueMetrics.rejectedTotalVal ? queueMetrics.rejectedTotalVal.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
        </div>
        <div style="background: #0f172a; color: #fff; border: 1px solid #0f172a; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #ffb200; text-transform: uppercase; display: block;">MODO DE APROVAÇÃO PIX</span>
            <div style="font-family: monospace; font-size: 0.9rem; font-weight: 700; color: #fff; margin-top: 4px;">
                {gestaoConfig.autoApproveEnabled ? `AUTO-APROVAR < R$ ${gestaoConfig.autoApproveWithdrawalsUnder || 200}` : 'APROVAÇÃO 100% MANUAL'}
            </div>
        </div>
    </div>

    <!-- Painel de Regras de Auto-Aprovação PIX -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 14px; display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap;">
        <div style="display: flex; align-items: center; gap: 10px;">
            <input id="chk-auto-appr" type="checkbox" bind:checked={gestaoConfig.autoApproveEnabled} style="width: 18px; height: 18px;" />
            <label for="chk-auto-appr" style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #0f172a; cursor: pointer;">HABILITAR APROVAÇÃO AUTOMÁTICA VIA GATEWAY PIX PARA VALORES BAIXOS</label>
        </div>

        <div style="display: flex; align-items: center; gap: 8px;">
            <label for="auto-lim-inp" style="font-family: monospace; font-size: 0.65rem; font-weight: 700; color: #64748b;">LIMITE MÁXIMO AUTO (R$):</label>
            <input id="auto-lim-inp" type="number" step="10" bind:value={gestaoConfig.autoApproveWithdrawalsUnder} style="width: 100px; padding: 4px 8px; font-family: monospace; font-size: 0.75rem; border: 1px solid #94a3b8;" />
            <button on:click={onSaveAutoApproveSettings} style="background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.68rem; font-weight: 700; padding: 6px 12px; border: 1px solid #000; cursor: pointer;">SALVAR REGRA</button>
        </div>
    </div>

    <!-- Filtros da Fila -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 12px; display: flex; gap: 6px; align-items: center;">
        <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; margin-right: 4px;">VER FILA:</span>
        <button 
            on:click={() => withdrawQueueFilter = 'pendentes'}
            style="padding: 6px 12px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {withdrawQueueFilter === 'pendentes' ? '#b91c1c' : '#fff'}; color: {withdrawQueueFilter === 'pendentes' ? '#fff' : '#0f172a'}; cursor: pointer;"
        >
            PENDENTES DE APROVAÇÃO ({queueMetrics.pendingCount || 0})
        </button>
        <button 
            on:click={() => withdrawQueueFilter = 'aprovados'}
            style="padding: 6px 12px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {withdrawQueueFilter === 'aprovados' ? '#047857' : '#fff'}; color: {withdrawQueueFilter === 'aprovados' ? '#fff' : '#0f172a'}; cursor: pointer;"
        >
            HISTÓRICO APROVADOS ({queueMetrics.approvedCount || 0})
        </button>
        <button 
            on:click={() => withdrawQueueFilter = 'rejeitados'}
            style="padding: 6px 12px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {withdrawQueueFilter === 'rejeitados' ? '#b45309' : '#fff'}; color: {withdrawQueueFilter === 'rejeitados' ? '#fff' : '#0f172a'}; cursor: pointer;"
        >
            REJEITADOS / ESTORNADOS ({queueMetrics.rejectedCount || 0})
        </button>
        <button 
            on:click={() => withdrawQueueFilter = 'alto_valor'}
            style="padding: 6px 12px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {withdrawQueueFilter === 'alto_valor' ? '#ffb200' : '#fff'}; color: #000; cursor: pointer;"
        >
            ALTO VALOR (>= R$ 500)
        </button>
    </div>

    <!-- Tabela Densa de Auditoria de Saques -->
    <div style="background: #fff; border: 1px solid #e2e8f0; overflow-x: auto;">
        {#if filteredWithdrawals.length === 0}
            <div style="padding: 32px; text-align: center; font-family: monospace; font-size: 0.75rem; color: #64748b;">
                NENHUMA SOLICITAÇÃO ENCONTRADA COM ESTES CRITÉRIOS DE FILTRO.
            </div>
        {:else}
            <table style="width: 100%; border-collapse: collapse; font-size: 0.75rem;">
                <thead>
                    <tr style="border-bottom: 1px solid #e2e8f0; background: #f8fafc; text-align: left; font-family: monospace; font-size: 0.6rem; color: #475569; text-transform: uppercase;">
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">PROTOCOLO SAQUE / DATA</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">OPERADOR / CONTATO</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">CHAVE PIX DESTINO</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">VALOR PIX (R$)</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">AUDITORIA DE RISCO (LTV)</th>
                        <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">STATUS</th>
                        <th style="padding: 8px 12px; text-align: right;">AÇÕES DE DECISÃO DA GESTÃO</th>
                    </tr>
                </thead>
                <tbody style="font-family: monospace;">
                    {#each filteredWithdrawals as tx (tx.id)}
                        {@const userObj = users.find(u => u.id === tx.userId)}
                        {@const stats = userObj ? gestaoService.getUserFinancialStats(userObj.id, transactions) : { totalDeposited: 0, totalWithdrawn: 0, netLtvProfit: 0 }}
                        <tr style="border-bottom: 1px solid #f1f5f9;">
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                                <strong style="color: #1e3a8a; display: block;">#{tx.id}</strong>
                                <span style="font-size: 0.62rem; color: #64748b;">{tx.data}</span>
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                                {#if userObj}
                                    <strong style="font-family: sans-serif; display: block; color: #0f172a;">{userObj.username.toUpperCase()}</strong>
                                    <span style="font-size: 0.62rem; color: #64748b;">NÍVEL {userObj.nivel} · {userObj.phone}</span>
                                {:else}
                                    <span style="color: #991b1b;">#{tx.userId} (NÃO LOCALIZADO)</span>
                                {/if}
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 700; color: #0f172a;">
                                {tx.pixKey || 'CHAVE-PIX-PADRÃO'}
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 900; font-size: 0.9rem; color: #b91c1c;">
                                R$ {Math.abs(tx.valor).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-size: 0.65rem;">
                                <div style="display: flex; flex-direction: column; gap: 2px;">
                                    <span>Depósitos: <strong style="color: #065f46;">R$ {stats.totalDeposited.toFixed(2)}</strong></span>
                                    <span>LTV Banca: <strong style="color: {stats.netLtvProfit >= 0 ? '#065f46' : '#b91c1c'};">R$ {stats.netLtvProfit.toFixed(2)}</strong></span>
                                    {#if userObj && userObj.withdrawLocked}
                                        <span style="color: #b91c1c; font-weight: 700;">CONTA COM SAQUE TRAVADO</span>
                                    {/if}
                                </div>
                            </td>
                            <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                                <span style="font-size: 0.58rem; padding: 2px 6px; font-weight: 800; text-transform: uppercase; border: 1px solid {tx.status === 'pendente' ? '#b91c1c' : tx.status === 'sucesso' ? '#047857' : '#b45309'}; background: {tx.status === 'pendente' ? '#fef2f2' : tx.status === 'sucesso' ? '#ecfdf5' : '#fffbeb'}; color: {tx.status === 'pendente' ? '#991b1b' : tx.status === 'sucesso' ? '#065f46' : '#b45309'};">
                                    {tx.status}
                                </span>
                            </td>
                            <td style="padding: 8px 12px; text-align: right;">
                                {#if tx.status === 'pendente'}
                                    <button 
                                        on:click={() => onApproveWithdrawal(tx.id)}
                                        style="padding: 4px 8px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #047857; color: #fff; cursor: pointer; margin-right: 4px;"
                                    >
                                        APROVAR PIX
                                    </button>
                                    <button 
                                        on:click={() => onRejectWithdrawal(tx.id, false)}
                                        style="padding: 4px 8px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: #fff; color: #b45309; cursor: pointer; margin-right: 4px;"
                                    >
                                        REJEITAR & ESTORNAR
                                    </button>
                                    <button 
                                        on:click={() => onRejectWithdrawal(tx.id, true)}
                                        style="padding: 4px 8px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #b91c1c; color: #fff; cursor: pointer;"
                                    >
                                        ESTORNAR & TRAVAR CONTA
                                    </button>
                                {:else}
                                    <span style="font-size: 0.6rem; color: #64748b; font-weight: 700;">DECISÃO PROCESSADA</span>
                                {/if}
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        {/if}
    </div>

</div>
