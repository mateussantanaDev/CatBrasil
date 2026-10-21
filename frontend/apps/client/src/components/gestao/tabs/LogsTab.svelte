<script>
    import { onMount } from 'svelte';
    import { api } from '../../../services/api.js';

    export let onShowToast = () => {};

    let securityLogs = [];
    let usageLogs = [];
    let activeLogType = 'security'; // 'security' | 'usage'
    let searchQuery = '';
    let isLoading = false;

    onMount(() => {
        loadLogs();
    });

    async function loadLogs() {
        isLoading = true;
        try {
            const [secRes, useRes] = await Promise.allSettled([
                api.getAdminSecurityLogs(200, 0),
                api.getAdminUsageLogs(200, 0)
            ]);

            if (secRes.status === 'fulfilled' && Array.isArray(secRes.value)) {
                securityLogs = secRes.value;
            }
            if (useRes.status === 'fulfilled' && Array.isArray(useRes.value)) {
                usageLogs = useRes.value;
            }
        } catch (e) {
            onShowToast(e.message || 'Erro ao carregar logs de auditoria', 'error');
        } finally {
            isLoading = false;
        }
    }

    $: filteredSecurityLogs = securityLogs.filter(log => {
        if (!searchQuery) return true;
        const q = searchQuery.toLowerCase();
        return (
            (log.event && log.event.toLowerCase().includes(q)) ||
            (log.userId && log.userId.toLowerCase().includes(q)) ||
            (log.ipAddress && log.ipAddress.toLowerCase().includes(q)) ||
            (log.details && log.details.toLowerCase().includes(q))
        );
    });

    $: filteredUsageLogs = usageLogs.filter(log => {
        if (!searchQuery) return true;
        const q = searchQuery.toLowerCase();
        return (
            (log.type && log.type.toLowerCase().includes(q)) ||
            (log.id && String(log.id).includes(q))
        );
    });

    function formatTime(isoStr) {
        if (!isoStr) return '--:--:--';
        const d = new Date(isoStr);
        return d.toLocaleDateString('pt-BR') + ' ' + d.toLocaleTimeString('pt-BR');
    }
</script>

<!-- ABA AUDITORIA, SEGURANÇA & LOGS IMUTÁVEIS -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">

    <!-- Header do Painel -->
    <div style="border-bottom: 1px solid #e2e8f0; background: linear-gradient(to right, #f8fafc, #ffffff); padding: 12px 16px; display: flex; align-items: center; justify-content: space-between;">
        <div style="display: flex; align-items: center; gap: 12px;">
            <span style="width: 24px; height: 24px; background: #0f172a; color: #fff; font-family: monospace; font-size: 0.7rem; font-weight: 700; display: flex; align-items: center; justify-content: center;">🛡️</span>
            <div>
                <h3 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">TRILHA DE AUDITORIA & SEGURANÇA IMUTÁVEL</h3>
                <p style="font-size: 0.68rem; color: #64748b; margin: 2px 0 0 0;">Histórico criptográfico de acessos, ações de gestores, aprovações de saques e coletas</p>
            </div>
        </div>

        <button 
            on:click={loadLogs}
            disabled={isLoading}
            style="background: #0f172a; color: #ffb200; font-family: monospace; font-size: 0.7rem; font-weight: 700; padding: 6px 12px; border: 1px solid #000; cursor: pointer;"
        >
            {isLoading ? '⏳ SINCRONIZANDO...' : '🔄 ATUALIZAR LOGS'}
        </button>
    </div>

    <!-- Filtros de Tipo de Log e Barra de Busca -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 12px; display: flex; flex-wrap: wrap; gap: 10px; align-items: center; justify-content: space-between;">
        <div style="display: flex; gap: 8px;">
            <button 
                on:click={() => activeLogType = 'security'}
                style="font-family: monospace; font-size: 0.72rem; font-weight: 700; padding: 6px 12px; border: 1px solid #000; background: {activeLogType === 'security' ? '#0f172a' : '#f8fafc'}; color: {activeLogType === 'security' ? '#ffb200' : '#475569'}; cursor: pointer;"
            >
                🛡️ LOGS DE SEGURANÇA & GESTÃO ({securityLogs.length})
            </button>
            <button 
                on:click={() => activeLogType = 'usage'}
                style="font-family: monospace; font-size: 0.72rem; font-weight: 700; padding: 6px 12px; border: 1px solid #000; background: {activeLogType === 'usage' ? '#0f172a' : '#f8fafc'}; color: {activeLogType === 'usage' ? '#ffb200' : '#475569'}; cursor: pointer;"
            >
                ⚙️ LOGS DE USO & COLETAS ({usageLogs.length})
            </button>
        </div>

        <div style="flex: 1; max-width: 320px;">
            <input 
                type="text" 
                placeholder="Pesquisar por evento, usuário, IP..." 
                bind:value={searchQuery}
                style="width: 100%; padding: 6px 10px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;"
            />
        </div>
    </div>

    <!-- TABELA DE LOGS DE SEGURANÇA -->
    {#if activeLogType === 'security'}
        <div style="background: #fff; border: 1px solid #e2e8f0; overflow-x: auto;">
            <table style="width: 100%; border-collapse: collapse; font-family: monospace; font-size: 0.7rem;">
                <thead>
                    <tr style="background: #0f172a; color: #ffb200; text-align: left;">
                        <th style="padding: 10px 12px;">ID</th>
                        <th style="padding: 10px 12px;">DATA/HORA</th>
                        <th style="padding: 10px 12px;">OPERADOR</th>
                        <th style="padding: 10px 12px;">EVENTO</th>
                        <th style="padding: 10px 12px;">IP / DISPOSITIVO</th>
                        <th style="padding: 10px 12px;">DETALHES</th>
                    </tr>
                </thead>
                <tbody>
                    {#if filteredSecurityLogs.length === 0}
                        <tr>
                            <td colspan="6" style="padding: 24px; text-align: center; color: #94a3b8;">
                                NENHUM REGISTRO DE AUDITORIA LOCALIZADO
                            </td>
                        </tr>
                    {:else}
                        {#each filteredSecurityLogs as log (log.id)}
                            <tr style="border-bottom: 1px solid #e2e8f0; background: {log.event.includes('GESTOR') ? '#fefce8' : log.event.includes('WITHDRAW') ? '#f0fdf4' : '#fff'};">
                                <td style="padding: 8px 12px; font-weight: 700; color: #64748b;">#{log.id}</td>
                                <td style="padding: 8px 12px; color: #0f172a; white-space: nowrap;">{formatTime(log.createdAt)}</td>
                                <td style="padding: 8px 12px; font-weight: 700; color: #1e3a8a;">{log.userId || 'SISTEMA'}</td>
                                <td style="padding: 8px 12px;">
                                    <span style="display: inline-block; padding: 2px 6px; font-weight: 700; font-size: 0.65rem; border: 1px solid {log.event.includes('GESTOR') ? '#ca8a04' : '#334155'}; background: {log.event.includes('GESTOR') ? '#fef08a' : '#f1f5f9'}; color: {log.event.includes('GESTOR') ? '#854d0e' : '#0f172a'};">
                                        {log.event}
                                    </span>
                                </td>
                                <td style="padding: 8px 12px; color: #64748b; font-size: 0.62rem;">{log.ipAddress || '127.0.0.1'}</td>
                                <td style="padding: 8px 12px; color: #475569; font-size: 0.65rem;">{log.details || '—'}</td>
                            </tr>
                        {/each}
                    {/if}
                </tbody>
            </table>
        </div>
    {:else}
        <!-- TABELA DE LOGS DE USO E COLETAS -->
        <div style="background: #fff; border: 1px solid #e2e8f0; overflow-x: auto;">
            <table style="width: 100%; border-collapse: collapse; font-family: monospace; font-size: 0.7rem;">
                <thead>
                    <tr style="background: #0f172a; color: #ffb200; text-align: left;">
                        <th style="padding: 10px 12px;">ID</th>
                        <th style="padding: 10px 12px;">DATA/HORA</th>
                        <th style="padding: 10px 12px;">TIPO DE OPERAÇÃO</th>
                        <th style="padding: 10px 12px;">VALOR (R$)</th>
                    </tr>
                </thead>
                <tbody>
                    {#if filteredUsageLogs.length === 0}
                        <tr>
                            <td colspan="4" style="padding: 24px; text-align: center; color: #94a3b8;">
                                NENHUM REGISTRO DE USO LOCALIZADO
                            </td>
                        </tr>
                    {:else}
                        {#each filteredUsageLogs as log (log.id)}
                            <tr style="border-bottom: 1px solid #e2e8f0;">
                                <td style="padding: 8px 12px; font-weight: 700; color: #64748b;">#{log.id}</td>
                                <td style="padding: 8px 12px; color: #0f172a; white-space: nowrap;">{formatTime(log.createdAt)}</td>
                                <td style="padding: 8px 12px; font-weight: 700; color: #0f172a;">{log.type}</td>
                                <td style="padding: 8px 12px; font-weight: 900; color: #047857;">R$ {log.amount ? log.amount.toFixed(2) : '0.00'}</td>
                            </tr>
                        {/each}
                    {/if}
                </tbody>
            </table>
        </div>
    {/if}

</div>
