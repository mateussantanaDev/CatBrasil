<script>
    import { gestaoService } from '../../../services/gestaoService.js';

    export let users = [];
    export let transactions = [];
    export let bankMetrics = {};
    export let searchQuery = "";
    export let statusFilter = "todos";

    export let onOpenUserDrawer = () => {};

    $: filteredUsers = users.filter(u => {
        const matchesQuery = u.username.toLowerCase().includes(searchQuery.toLowerCase()) || 
                             u.phone.includes(searchQuery) || 
                             u.id.toLowerCase().includes(searchQuery.toLowerCase());
        if (!matchesQuery) return false;

        if (statusFilter === 'travados') return u.withdrawLocked;
        if (statusFilter === 'suspensos') return u.referralBonusSuspended;
        if (statusFilter === 'vip') return u.nivel >= 10;
        return true;
    });
</script>

<!-- ABA 2: BASE DE OPERADORES & CLIENTES DESACOPLADA -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- PanelHeader com KPIs da Base -->
    <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px;">
        <div style="background: #fff; border: 1px solid #e2e8f0; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">TOTAL DE OPERADORES</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #0f172a; margin-top: 2px;">{users.length} Clientes</div>
        </div>
        <div style="background: #fff; border: 1px solid #e2e8f0; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">CUSTÓDIA PASSIVA TOTAL</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #065f46; margin-top: 2px;">
                R$ {bankMetrics.totalUserBalances ? bankMetrics.totalUserBalances.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
        </div>
        <div style="background: #fff; border: 1px solid #e2e8f0; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">CONTAS COM SAQUE TRAVADO</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b91c1c; margin-top: 2px;">{users.filter(u => u.withdrawLocked).length} Restritos</div>
        </div>
        <div style="background: #fff; border: 1px solid #e2e8f0; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">COMISSÕES SUSPENSAS</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b45309; margin-top: 2px;">{users.filter(u => u.referralBonusSuspended).length} Contas</div>
        </div>
    </div>

    <!-- Barra de Busca & Filtros Inteligentes -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 14px; display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap;">
        <div style="flex: 1; min-width: 280px;">
            <label for="search-usr-id" style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 4px;">BUSCAR POR NOME, CELULAR OU ID PROTOCOLO:</label>
            <input 
                id="search-usr-id"
                type="text" 
                placeholder="Digite ex: Operador 458F ou 11999..." 
                bind:value={searchQuery}
                style="width: 100%; background: #fff; border: 1px solid #cbd5e1; padding: 8px 12px; font-family: monospace; font-size: 0.75rem; color: #0f172a; box-sizing: border-box;"
            />
        </div>

        <!-- Filtros Rápidos -->
        <div style="display: flex; gap: 6px; align-items: center;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; margin-right: 4px;">FILTRAR:</span>
            <button 
                on:click={() => statusFilter = 'todos'}
                style="padding: 6px 10px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {statusFilter === 'todos' ? '#1e3a8a' : '#fff'}; color: {statusFilter === 'todos' ? '#fff' : '#0f172a'}; cursor: pointer;"
            >
                TODOS ({users.length})
            </button>
            <button 
                on:click={() => statusFilter = 'travados'}
                style="padding: 6px 10px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {statusFilter === 'travados' ? '#b91c1c' : '#fff'}; color: {statusFilter === 'travados' ? '#fff' : '#0f172a'}; cursor: pointer;"
            >
                SAQUES TRAVADOS
            </button>
            <button 
                on:click={() => statusFilter = 'suspensos'}
                style="padding: 6px 10px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {statusFilter === 'suspensos' ? '#b45309' : '#fff'}; color: {statusFilter === 'suspensos' ? '#fff' : '#0f172a'}; cursor: pointer;"
            >
                COMISSÕES SUSPENSAS
            </button>
            <button 
                on:click={() => statusFilter = 'vip'}
                style="padding: 6px 10px; font-size: 0.62rem; font-family: monospace; font-weight: 700; border: 1px solid #cbd5e1; background: {statusFilter === 'vip' ? '#ffb200' : '#fff'}; color: #000; cursor: pointer;"
            >
                OPERADORES NÍVEL 10+
            </button>
        </div>
    </div>

    <!-- Tabela Densa B2G com Auditoria e LTV por Cliente -->
    <div style="background: #fff; border: 1px solid #e2e8f0; overflow-x: auto;">
        <table style="width: 100%; border-collapse: collapse; font-size: 0.75rem;">
            <thead>
                <tr style="border-bottom: 1px solid #e2e8f0; background: #f8fafc; text-align: left; font-family: monospace; font-size: 0.6rem; color: #475569; text-transform: uppercase;">
                    <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">PROTOCOLO ID</th>
                    <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">NOME OPERADOR / CONTATO</th>
                    <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">CARREIRA NÍVEL</th>
                    <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">SALDO CUSTODIADO</th>
                    <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">DEPÓSITOS vs SAQUES</th>
                    <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">LUCRO BANCA (LTV)</th>
                    <th style="border-right: 1px solid #e2e8f0; padding: 8px 12px;">STATUS PERMISSÃO</th>
                    <th style="padding: 8px 12px; text-align: right;">GESTÃO DE COMANDO</th>
                </tr>
            </thead>
            <tbody style="font-family: monospace;">
                {#each filteredUsers as u (u.id)}
                    {@const stats = gestaoService.getUserFinancialStats(u.id, transactions)}
                    <tr style="border-bottom: 1px solid #f1f5f9;">
                        <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 700; color: #1e3a8a;">#{u.id}</td>
                        <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                            <strong style="font-family: sans-serif; display: block; color: #0f172a;">{u.username.toUpperCase()}</strong>
                            <span style="font-size: 0.62rem; color: #64748b;">{u.phone}</span>
                        </td>
                        <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; color: #0f172a;">
                            <span style="background: #f1f5f9; border: 1px solid #cbd5e1; padding: 2px 6px; font-size: 0.6rem; font-weight: 700;">NÍVEL {u.nivel}</span>
                        </td>
                        <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 700; color: #065f46;">
                            R$ {u.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                        </td>
                        <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-size: 0.68rem;">
                            <span style="color: #065f46;">+ R$ {stats.totalDeposited.toFixed(2)}</span> / 
                            <span style="color: #b91c1c;">- R$ {stats.totalWithdrawn.toFixed(2)}</span>
                        </td>
                        <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px; font-weight: 700; color: {stats.netLtvProfit >= 0 ? '#065f46' : '#b91c1c'};">
                            R$ {stats.netLtvProfit.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                        </td>
                        <td style="border-right: 1px solid #f1f5f9; padding: 8px 12px;">
                            <div style="display: flex; flex-direction: column; gap: 2px;">
                                <span style="font-size: 0.55rem; padding: 1px 4px; font-weight: 700; border: 1px solid {u.withdrawLocked ? '#b91c1c' : '#047857'}; background: {u.withdrawLocked ? '#fef2f2' : '#ecfdf5'}; color: {u.withdrawLocked ? '#991b1b' : '#065f46'}; width: fit-content;">
                                    SAQUE: {u.withdrawLocked ? 'TRAVADO' : 'LIBERADO'}
                                </span>
                                {#if u.referralBonusSuspended}
                                    <span style="font-size: 0.55rem; padding: 1px 4px; font-weight: 700; border: 1px solid #b45309; background: #fffbeb; color: #b45309; width: fit-content;">
                                        REDE: SUSPENSA
                                    </span>
                                {/if}
                            </div>
                        </td>
                        <td style="padding: 8px 12px; text-align: right;">
                            <button 
                                on:click={() => onOpenUserDrawer(u)}
                                style="padding: 4px 10px; font-size: 0.65rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #ffb200; color: #000; cursor: pointer; text-transform: uppercase;"
                            >
                                INSPECIONAR / GERENCIAR
                            </button>
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>

</div>
