<script>
    import { onMount } from 'svelte';
    import { gestaoService } from '../../services/gestaoService.js';
    import { api } from '../../services/api.js';

    // Importação da Tela de Login do ERP
    import ErpLoginScreen from './ErpLoginScreen.svelte';

    // Importação dos Módulos Desacoplados por Aba (Arquitetura 100% Modular)
    import DreTab from './tabs/DreTab.svelte';
    import OperadoresTab from './tabs/OperadoresTab.svelte';
    import SaquesQueueTab from './tabs/SaquesQueueTab.svelte';
    import RoletaMatrixTab from './tabs/RoletaMatrixTab.svelte';
    import ComunicadosTab from './tabs/ComunicadosTab.svelte';
    import ComissoesTab from './tabs/ComissoesTab.svelte';
    import FrotaTab from './tabs/FrotaTab.svelte';

    export let users = [];
    export let transactions = [];
    export let onUpdateUser = () => {};
    export let onCloseErp = () => {};
    export let onShowToast = () => {};

    // Estado Autenticação do Módulo ERP
    let isAuthenticated = gestaoService.isErpAuthenticated();
    let activeTab = 'metrics'; // 'metrics' | 'users' | 'withdrawals' | 'roulette' | 'events' | 'commission' | 'frota'
    let gestaoConfig = gestaoService.getConfig();
    let events = gestaoService.getEvents();
    let rouletteSlices = gestaoService.getRouletteSlices();
    let machines = gestaoService.getMachinesCatalog();
    let bankMetrics = gestaoService.getBankMetrics(users, transactions);
    let queueMetrics = gestaoService.getWithdrawalQueueMetrics(transactions);
    let rouletteMath = gestaoService.getRouletteMathematicalMetrics(rouletteSlices);

    let clockDisplay = "";
    let searchQuery = "";
    let statusFilter = "todos"; 
    let withdrawQueueFilter = "pendentes";

    // Modal/Drawer de Gestão Individual do Operador
    let selectedUserForEdit = null;
    let balanceInputVal = "";
    let levelInputVal = 1;

    // Configuração de Gateway PIX
    let gwName = gestaoConfig.gateway ? gestaoConfig.gateway.name : "Gateway PIX Padrão";
    let gwDepositFee = gestaoConfig.gateway ? gestaoConfig.gateway.depositFeePercent : 1.5;
    let gwWithdrawFee = gestaoConfig.gateway ? gestaoConfig.gateway.withdrawFeeFixed : 1.00;
    let gwPixExpMinutes = gestaoConfig.gateway ? gestaoConfig.gateway.pixExpirationMinutes : 15;

    // Novo Comunicado Rich Media
    let newTitle = "";
    let newCategory = "CAMPANHA DA GESTÃO";
    let newContent = "";
    let newImageUrl = "";
    let newCtaLink = "";
    let newCtaText = "ACESSAR AGORA";
    let newShowInWelcome = false;

    onMount(() => {
        refreshData();
        updateClock();
        const interval = setInterval(updateClock, 1000);
        return () => clearInterval(interval);
    });

    function updateClock() {
        const now = new Date();
        clockDisplay = now.toLocaleDateString('pt-BR') + ' ' + now.toLocaleTimeString('pt-BR');
    }

    async function refreshData() {
        gestaoConfig = gestaoService.getConfig();
        events = gestaoService.getEvents();
        machines = gestaoService.getMachinesCatalog();

        try {
            const adminUsers = await api.getAdminUsers();
            if (adminUsers && adminUsers.length > 0) users = adminUsers;

            const adminTxs = await api.getAdminTransactions();
            if (adminTxs && adminTxs.length > 0) transactions = adminTxs;

            const rConfig = await api.getRouletteConfig();
            if (rConfig && rConfig.length > 0) rouletteSlices = rConfig;

            const serverDre = await api.getAdminDre();
            if (serverDre) bankMetrics = { ...gestaoService.getBankMetrics(users, transactions), ...serverDre };
        } catch (e) {
            console.warn("API de Gestão offline:", e.message);
            bankMetrics = gestaoService.getBankMetrics(users, transactions);
        }

        queueMetrics = gestaoService.getWithdrawalQueueMetrics(transactions);
        rouletteMath = gestaoService.getRouletteMathematicalMetrics(rouletteSlices);

        if (gestaoConfig.gateway) {
            gwName = gestaoConfig.gateway.name;
            gwDepositFee = gestaoConfig.gateway.depositFeePercent;
            gwWithdrawFee = gestaoConfig.gateway.withdrawFeeFixed;
            gwPixExpMinutes = gestaoConfig.gateway.pixExpirationMinutes;
        }
    }

    // Trava de Emergência Global
    function toggleGlobalLock() {
        const updated = gestaoService.updateConfig({ globalWithdrawalsLocked: !gestaoConfig.globalWithdrawalsLocked });
        gestaoConfig = updated;
        onShowToast(updated.globalWithdrawalsLocked ? "SAQUES GLOBAIS TRAVADOS EM MASSA!" : "SAQUES GLOBAIS LIBERADOS COM SUCESSO!", updated.globalWithdrawalsLocked ? "error" : "win");
    }

    function handleSaveGatewaySettings() {
        const updated = gestaoService.updateConfig({
            gateway: {
                name: gwName,
                depositFeePercent: parseFloat(gwDepositFee),
                withdrawFeeFixed: parseFloat(gwWithdrawFee),
                pixExpirationMinutes: parseInt(gwPixExpMinutes)
            },
            collectionGraceMinutes: parseInt(gestaoConfig.collectionGraceMinutes || 10)
        });
        gestaoConfig = updated;
        refreshData();
        onShowToast("Configurações do Gateway PIX salvas no DRE!", "win");
    }

    function handleSaveAutoApproveSettings() {
        const updated = gestaoService.updateConfig({
            autoApproveEnabled: gestaoConfig.autoApproveEnabled,
            autoApproveWithdrawalsUnder: parseFloat(gestaoConfig.autoApproveWithdrawalsUnder || 200)
        });
        gestaoConfig = updated;
        onShowToast("Regras de Aprovação Automática atualizadas!", "win");
    }

    // AÇÕES DA ROLETA DIÁRIA (SALVOS DIRETO NO BACKEND)
    async function handleSaveRouletteSettings() {
        try {
            await api.updateRouletteConfig(rouletteSlices);
            gestaoService.updateRouletteSlices(rouletteSlices);
            const updated = gestaoService.updateConfig({
                roulette: {
                    strictModalMode: gestaoConfig.roulette ? gestaoConfig.roulette.strictModalMode : true,
                    requireDepositToSpin: gestaoConfig.roulette ? gestaoConfig.roulette.requireDepositToSpin : false
                }
            });
            gestaoConfig = updated;
            await refreshData();
            onShowToast("Matriz de Probabilidades salva no servidor backend!", "win");
        } catch (e) {
            onShowToast(e.message || "Erro ao salvar regras da roleta", "error");
        }
    }

    function handleSaveMachinesCatalog() {
        gestaoService.updateMachinesCatalog(machines);
        refreshData();
        onShowToast("Catálogo de Frota salvo com sucesso!", "win");
    }

    function handleNormalizeProbabilities() {
        let currentSum = 0;
        rouletteSlices.forEach(s => currentSum += (parseFloat(s.probability) || 0));
        const diff = 100.0 - (currentSum - parseFloat(rouletteSlices[5].probability));
        rouletteSlices[5].probability = parseFloat(Math.max(0, diff).toFixed(1));
        refreshData();
        onShowToast("Probabilidades normalizadas para fechar em 100%!", "win");
    }

    function handleApplyPresetMaxProfit() {
        rouletteSlices = [
            { id: 1, label: "R$ 10", amount: 10.00, probability: 3.0, color: "#ffb200", textColor: "#111" },
            { id: 2, label: "R$ 50", amount: 50.00, probability: 1.0, color: "#111111", textColor: "#fff" },
            { id: 3, label: "R$ 100", amount: 100.00, probability: 0.5, color: "#2e7d32", textColor: "#fff" },
            { id: 4, label: "R$ 20", amount: 20.00, probability: 2.5, color: "#d32f2f", textColor: "#fff" },
            { id: 5, label: "R$ 500", amount: 500.00, probability: 0.05, color: "#ffb200", textColor: "#111" },
            { id: 6, label: "R$ 5", amount: 5.00, probability: 92.95, color: "#111111", textColor: "#fff" }
        ];
        refreshData();
        onShowToast("Preset LUCRO MÁXIMO (EV R$ 6.20) aplicado!", "win");
    }

    function handleApplyPresetViral() {
        rouletteSlices = [
            { id: 1, label: "R$ 10", amount: 10.00, probability: 15.0, color: "#ffb200", textColor: "#111" },
            { id: 2, label: "R$ 50", amount: 50.00, probability: 5.0, color: "#111111", textColor: "#fff" },
            { id: 3, label: "R$ 100", amount: 100.00, probability: 2.0, color: "#2e7d32", textColor: "#fff" },
            { id: 4, label: "R$ 20", amount: 20.00, probability: 10.0, color: "#d32f2f", textColor: "#fff" },
            { id: 5, label: "R$ 500", amount: 500.00, probability: 0.5, color: "#ffb200", textColor: "#111" },
            { id: 6, label: "R$ 5", amount: 5.00, probability: 67.5, color: "#111111", textColor: "#fff" }
        ];
        refreshData();
        onShowToast("Preset ATRAÇÃO VIRAL (EV R$ 14.50) aplicado!", "win");
    }

    // AÇÕES DA FILA DE SAQUES PIX NO SERVIDOR
    async function handleApproveWithdrawal(txId) {
        try {
            const res = await api.approveWithdrawal(txId);
            onShowToast(res.message || `Saque #${txId} APROVADO no servidor!`, "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao aprovar saque no servidor", "error");
        }
    }

    async function handleRejectWithdrawal(txId, lockAccount = false) {
        try {
            const res = await api.rejectWithdrawal(txId, lockAccount);
            onShowToast(res.message || `Saque #${txId} REJEITADO no servidor!`, "error");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao rejeitar saque no servidor", "error");
        }
    }

    // AÇÕES DE CONTROLE INDIVIDUAL DO OPERADOR NO SERVIDOR
    function openUserDrawer(userObj) {
        selectedUserForEdit = { ...userObj };
        balanceInputVal = "";
        levelInputVal = userObj.nivel;
    }

    async function handleAdjustBalance(isAdd) {
        const val = parseFloat(balanceInputVal);
        if (isNaN(val) || val <= 0) {
            onShowToast("Digite um valor válido!", "error");
            return;
        }
        const newBal = isAdd ? selectedUserForEdit.balance + val : Math.max(0, selectedUserForEdit.balance - val);
        try {
            await api.updateAdminUser(selectedUserForEdit.id, { balance: newBal });
            selectedUserForEdit.balance = newBal;
            onShowToast(`Saldo de ${selectedUserForEdit.username} alterado para R$ ${newBal.toFixed(2)}`, "win");
            balanceInputVal = "";
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao atualizar saldo", "error");
        }
    }

    async function toggleUserLock(userObj) {
        const target = userObj || selectedUserForEdit;
        const newLock = !target.withdrawLocked;
        try {
            await api.updateAdminUser(target.id, { withdrawLocked: newLock });
            target.withdrawLocked = newLock;
            onShowToast(newLock ? `Saques de ${target.username} TRAVADOS!` : `Saques de ${target.username} LIBERADOS!`, newLock ? "error" : "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar trava", "error");
        }
    }

    async function toggleReferralSuspension(userObj) {
        const target = userObj || selectedUserForEdit;
        const newSusp = !target.referralBonusSuspended;
        try {
            await api.updateAdminUser(target.id, { referralBonusSuspended: newSusp });
            target.referralBonusSuspended = newSusp;
            onShowToast(newSusp ? `Comissões de ${target.username} SUSPENSAS!` : `Comissões de ${target.username} REATIVADAS!`, newSusp ? "error" : "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar comissão", "error");
        }
    }

    async function handleSetLevel() {
        const lvl = parseInt(levelInputVal);
        if (isNaN(lvl) || lvl < 1) {
            onShowToast("Nível inválido!", "error");
            return;
        }
        try {
            await api.updateAdminUser(selectedUserForEdit.id, { nivel: lvl });
            selectedUserForEdit.nivel = lvl;
            onShowToast(`Nível de ${selectedUserForEdit.username} alterado para NÍVEL ${lvl}!`, "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar nível", "error");
        }
    }

    async function handleAddBonusMachine(machineModel) {
        if (!selectedUserForEdit) return;
        try {
            await api.rentMachine(selectedUserForEdit.id, machineModel);
            onShowToast(`Máquina bônus concedida à garagem de ${selectedUserForEdit.username}!`, "win");
            refreshData();
        } catch (e) {
            onShowToast("Erro ao conceder máquina: " + e.message, "error");
        }
    }

    function handleResetUserSpin() {
        if (!selectedUserForEdit) return;
        selectedUserForEdit.alreadySpunToday = false;
        onUpdateUser(selectedUserForEdit.id, { alreadySpunToday: false });
        onShowToast(`Giro Diário de ${selectedUserForEdit.username} liberado para novo uso!`, "win");
        refreshData();
    }

    // Publicar Aviso
    function handleCreateEvent() {
        if (!newTitle || !newContent) {
            onShowToast("Preencha o título e o conteúdo do comunicado!", "error");
            return;
        }
        events = gestaoService.addEvent({
            title: newTitle,
            category: newCategory,
            content: newContent,
            imageUrl: newImageUrl,
            ctaLink: newCtaLink,
            ctaText: newCtaText,
            showInWelcomeModal: newShowInWelcome,
            badgeColor: "#ffb200"
        });
        newTitle = "";
        newContent = "";
        newImageUrl = "";
        newCtaLink = "";
        newCtaText = "ACESSAR AGORA";
        newShowInWelcome = false;
        onShowToast("Comunicado Rich Media publicado instantaneamente!", "win");
    }

    function handleToggleEventActive(id) {
        events = gestaoService.toggleEventActive(id);
        onShowToast("Status do comunicado alterado com sucesso!", "info");
    }

    function handleDeleteEvent(id) {
        events = gestaoService.deleteEvent(id);
        onShowToast("Comunicado removido.", "info");
    }

    // Salvar Comissões
    function handleSaveCommission() {
        gestaoService.updateConfig({
            referralCommissionPercent: parseFloat(gestaoConfig.referralCommissionPercent),
            requireReferralsForWithdrawal: gestaoConfig.requireReferralsForWithdrawal
        });
        onShowToast("Regras de comissão salvas com sucesso!", "win");
    }

    function handleLogoutErp() {
        gestaoService.logoutErp();
        isAuthenticated = false;
        onCloseErp();
    }
</script>

{#if !isAuthenticated}
    <ErpLoginScreen 
        onLoginSuccess={() => { isAuthenticated = true; refreshData(); }} 
        onCancel={onCloseErp} 
        {onShowToast} 
    />
{:else}
<!-- STANDALONE DESKTOP SOFTWARE SHELL (DESIGN SYSTEM UNISISM/CATBRASIL B2G) -->
<div style="position: fixed; inset: 0; z-index: 999999; background: #0f172a; display: flex; font-family: 'Inter', system-ui, sans-serif; color: #0f172a; width: 100vw; height: 100vh; overflow: hidden;">
    
    <!-- SIDEBAR ESQUERDA FIXA (w-64) -->
    <aside style="width: 256px; background: #0f172a; color: #ffffff; display: flex; flex-direction: column; border-right: 1px solid #1e293b; flex-shrink: 0; user-select: none;">
        
        <!-- Topo Branding -->
        <div style="padding: 16px; border-bottom: 1px solid #1e293b; display: flex; align-items: center; gap: 12px;">
            <div style="width: 36px; height: 36px; background: #ffb200; border: 1px solid #000; display: flex; align-items: center; justify-content: center; font-family: monospace; font-weight: 900; color: #000; font-size: 0.95rem;">
                CAT
            </div>
            <div>
                <h1 style="font-family: monospace; font-size: 0.85rem; font-weight: 700; color: #fff; text-transform: uppercase; margin: 0; line-height: 1;">CatBrasil</h1>
                <span style="font-size: 0.58rem; font-family: monospace; color: #ffb200; text-transform: uppercase;">MÓDULO DE GESTÃO ERP</span>
            </div>
        </div>

        <!-- Escopo Operacional -->
        <div style="padding: 10px 16px; background: #020617; border-bottom: 1px solid #1e293b;">
            <span style="font-size: 0.52rem; font-family: monospace; color: #94a3b8; text-transform: uppercase; display: block;">ESCOPO DO CAIXA</span>
            <strong style="font-size: 0.68rem; font-family: monospace; color: #e2e8f0; display: block; margin-top: 2px;">CENTRAL DE LIQUIDEZ DA BANCA</strong>
        </div>

        <!-- Menu de Abas Vertical -->
        <nav style="flex: 1; padding: 12px 8px; display: flex; flex-direction: column; gap: 4px; overflow-y: auto;">
            
            <button 
                on:click={() => { activeTab = 'metrics'; refreshData(); }}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'metrics' ? '#ffb200' : 'transparent'}; background: {activeTab === 'metrics' ? '#1e293b' : 'transparent'}; color: {activeTab === 'metrics' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>DRE & MARGEM BANCA</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">1</kbd>
            </button>

            <button 
                on:click={() => activeTab = 'users'}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'users' ? '#ffb200' : 'transparent'}; background: {activeTab === 'users' ? '#1e293b' : 'transparent'}; color: {activeTab === 'users' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>OPERADORES / CLIENTES</span>
                <span style="font-size: 0.6rem; font-family: monospace; background: #334155; padding: 1px 5px; color: #fff;">{users.length}</span>
            </button>

            <button 
                on:click={() => { activeTab = 'withdrawals'; refreshData(); }}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'withdrawals' ? '#ffb200' : 'transparent'}; background: {activeTab === 'withdrawals' ? '#1e293b' : 'transparent'}; color: {activeTab === 'withdrawals' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>FILA DE SAQUES</span>
                {#if queueMetrics.pendingCount > 0}
                    <span style="font-size: 0.6rem; font-family: monospace; background: #b91c1c; color: #fff; padding: 1px 5px; font-weight: 900;">{queueMetrics.pendingCount}</span>
                {/if}
            </button>

            <button 
                on:click={() => { activeTab = 'roulette'; refreshData(); }}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'roulette' ? '#ffb200' : 'transparent'}; background: {activeTab === 'roulette' ? '#1e293b' : 'transparent'}; color: {activeTab === 'roulette' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>ROLETA & PROBABILIDADES</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">4</kbd>
            </button>

            <button 
                on:click={() => activeTab = 'events'}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'events' ? '#ffb200' : 'transparent'}; background: {activeTab === 'events' ? '#1e293b' : 'transparent'}; color: {activeTab === 'events' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>PUBLICAR COMUNICADOS</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">5</kbd>
            </button>

            <button 
                on:click={() => activeTab = 'commission'}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'commission' ? '#ffb200' : 'transparent'}; background: {activeTab === 'commission' ? '#1e293b' : 'transparent'}; color: {activeTab === 'commission' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>COMISSÕES & REDE</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">6</kbd>
            </button>

            <button 
                on:click={() => { activeTab = 'frota'; refreshData(); }}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'frota' ? '#ffb200' : 'transparent'}; background: {activeTab === 'frota' ? '#1e293b' : 'transparent'}; color: {activeTab === 'frota' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>FROTA & MÁQUINAS</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">7</kbd>
            </button>
        </nav>

        <!-- Rodapé do Usuário da Gestão -->
        <div style="padding: 12px; background: #020617; border-top: 1px solid #1e293b; display: flex; align-items: center; justify-content: space-between;">
            <div style="display: flex; align-items: center; gap: 8px;">
                <div style="width: 28px; height: 28px; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.7rem; font-weight: 700; display: flex; align-items: center; justify-content: center; border: 1px solid #334155;">
                    GS
                </div>
                <div>
                    <strong style="font-size: 0.7rem; font-family: monospace; color: #f8fafc; display: block; line-height: 1;">GESTOR CAIXA</strong>
                    <span style="font-size: 0.52rem; font-family: monospace; color: #94a3b8; display: block; text-transform: uppercase;">CONEXÃO AUTENTICADA</span>
                </div>
            </div>
            <button 
                on:click={handleLogoutErp}
                style="background: #ffb200; color: #000; font-family: monospace; font-size: 0.65rem; font-weight: 900; padding: 4px 10px; border: 1px solid #000; cursor: pointer; text-transform: uppercase;"
            >
                SAIR
            </button>
        </div>
    </aside>

    <!-- PAINEL PRINCIPAL DESKTOP -->
    <div style="flex: 1; display: flex; flex-direction: column; min-width: 0; background: #f8fafc; overflow: hidden;">
        
        <!-- Header Superior -->
        <header style="height: 52px; background: #ffffff; border-bottom: 1px solid #e2e8f0; padding: 0 24px; display: flex; align-items: center; justify-content: space-between; flex-shrink: 0;">
            <div style="display: flex; align-items: center; gap: 8px;">
                <span style="font-family: monospace; font-size: 0.72rem; color: #64748b; text-transform: uppercase;">CATBRASIL / GESTÃO /</span>
                <h2 style="font-family: monospace; font-size: 0.8rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">
                    {activeTab === 'metrics' ? 'DRE AUTOMÁTICO & AUDITORIA DE USO DOS OPERADORES' : 
                     activeTab === 'users' ? 'GESTÃO & AUDITORIA AVANÇADA DE OPERADORES' : 
                     activeTab === 'withdrawals' ? 'FILA DE APROVAÇÃO E AUDITORIA DE SAQUES PIX' : 
                     activeTab === 'roulette' ? 'ROLETA & MATRIZ DE PROBABILIDADES MATEMÁTICAS' : 
                     activeTab === 'events' ? 'PUBLICAR COMUNICADOS DA GESTÃO' : 
                     activeTab === 'commission' ? 'REGRAS DE COMISSÃO & TRAVAS' : 'CATÁLOGO DE FROTA DE MÁQUINAS'}
                </h2>
            </div>

            <div style="display: flex; align-items: center; gap: 16px;">
                <span style="font-family: monospace; font-size: 0.62rem; font-weight: 700; padding: 3px 8px; border: 1px solid {gestaoConfig.globalWithdrawalsLocked ? '#b91c1c' : '#047857'}; background: {gestaoConfig.globalWithdrawalsLocked ? '#fef2f2' : '#ecfdf5'}; color: {gestaoConfig.globalWithdrawalsLocked ? '#991b1b' : '#065f46'}; text-transform: uppercase;">
                    {gestaoConfig.globalWithdrawalsLocked ? 'SAQUES GLOBAIS TRAVADOS' : 'SAQUES GLOBAIS LIBERADOS'}
                </span>
                <span style="font-family: monospace; font-size: 0.72rem; color: #475569; border-left: 1px solid #e2e8f0; padding-left: 16px;">
                    {clockDisplay}
                </span>
            </div>
        </header>

        <!-- Conteúdo do ERP Renderizado via Módulos Desacoplados -->
        <main style="flex: 1; overflow-y: auto; padding: 20px 24px; max-width: 1600px; width: 100%; margin: 0 auto; box-sizing: border-box; display: flex; flex-direction: column; gap: 16px;">
            
            <!-- ABA 1: DRE AUTOMÁTICO DESACOPLADO -->
            {#if activeTab === 'metrics'}
                <DreTab 
                    {bankMetrics}
                    {gestaoConfig}
                    {gwName}
                    {gwDepositFee}
                    {gwWithdrawFee}
                    {gwPixExpMinutes}
                    onSaveGatewaySettings={handleSaveGatewaySettings}
                    onToggleGlobalLock={toggleGlobalLock}
                />
            {/if}

            <!-- ABA 2: BASE DE OPERADORES DESACOPLADA -->
            {#if activeTab === 'users'}
                <OperadoresTab 
                    {users}
                    {transactions}
                    {bankMetrics}
                    bind:searchQuery
                    bind:statusFilter
                    onOpenUserDrawer={openUserDrawer}
                />
            {/if}

            <!-- ABA 3: FILA DE SAQUES PIX DESACOPLADA -->
            {#if activeTab === 'withdrawals'}
                <SaquesQueueTab 
                    {users}
                    {transactions}
                    {gestaoConfig}
                    {queueMetrics}
                    bind:withdrawQueueFilter
                    onSaveAutoApproveSettings={handleSaveAutoApproveSettings}
                    onApproveWithdrawal={handleApproveWithdrawal}
                    onRejectWithdrawal={handleRejectWithdrawal}
                />
            {/if}

            <!-- ABA 4: ROLETA & MATRIZ DE PROBABILIDADES DESACOPLADA -->
            {#if activeTab === 'roulette'}
                <RoletaMatrixTab 
                    {rouletteSlices}
                    {gestaoConfig}
                    {bankMetrics}
                    {rouletteMath}
                    onSaveRouletteSettings={handleSaveRouletteSettings}
                    onNormalizeProbabilities={handleNormalizeProbabilities}
                    onApplyPresetMaxProfit={handleApplyPresetMaxProfit}
                    onApplyPresetViral={handleApplyPresetViral}
                />
            {/if}

            <!-- ABA 5: PUBLICAR COMUNICADOS DESACOPLADA -->
            {#if activeTab === 'events'}
                <ComunicadosTab 
                    {events}
                    bind:newTitle
                    bind:newCategory
                    bind:newContent
                    bind:newImageUrl
                    bind:newCtaLink
                    bind:newCtaText
                    bind:newShowInWelcome
                    onCreateEvent={handleCreateEvent}
                    onToggleEventActive={handleToggleEventActive}
                    onDeleteEvent={handleDeleteEvent}
                />
            {/if}

            <!-- ABA 6: REGRAS DE COMISSÃO & AFILIADOS DESACOPLADA (APROFUNDADA) -->
            {#if activeTab === 'commission'}
                <ComissoesTab 
                    {users}
                    {transactions}
                    {gestaoConfig}
                    onSaveCommission={handleSaveCommission}
                    onToggleReferralSuspension={toggleReferralSuspension}
                />
            {/if}

            <!-- ABA 7: CATÁLOGO DE FROTA DE MÁQUINAS DESACOPLADO -->
            {#if activeTab === 'frota'}
                <FrotaTab 
                    bind:machines
                    {users}
                    onSaveMachinesCatalog={handleSaveMachinesCatalog}
                    {onShowToast}
                />
            {/if}

        </main>

        <!-- MODAL / DRAWER DE GESTÃO INDIVIDUAL DO OPERADOR -->
        {#if selectedUserForEdit}
            <div style="position: fixed; inset: 0; background: rgba(15,23,42,0.7); backdrop-filter: blur(2px); z-index: 1000000; display: flex; align-items: center; justify-content: center; padding: 20px;">
                <div style="background: #fff; border: 2px solid #0f172a; box-shadow: 6px 6px 0px #0f172a; width: 100%; max-width: 680px; max-height: 90vh; overflow-y: auto; padding: 20px; font-family: monospace;">
                    
                    <div style="border-bottom: 2px solid #0f172a; padding-bottom: 10px; margin-bottom: 14px; display: flex; justify-content: space-between; align-items: center;">
                        <div>
                            <span style="font-size: 0.6rem; font-weight: 700; color: #1e3a8a; text-transform: uppercase; display: block;">INSPEÇÃO INDIVIDUAL PROTOCOLO #{selectedUserForEdit.id}</span>
                            <h3 style="font-family: sans-serif; font-size: 1.1rem; font-weight: 900; color: #0f172a; margin: 2px 0 0 0;">{selectedUserForEdit.username.toUpperCase()} ({selectedUserForEdit.phone})</h3>
                        </div>
                        <button on:click={() => selectedUserForEdit = null} style="background: #0f172a; color: #fff; border: none; font-size: 1rem; padding: 4px 10px; cursor: pointer; font-weight: 700;">✕</button>
                    </div>

                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 14px;">
                        <div style="background: #ecfdf5; border: 1px solid #047857; padding: 10px;">
                            <span style="font-size: 0.58rem; color: #047857; font-weight: 700; display: block;">SALDO LIVRE CUSTODIADO</span>
                            <strong style="font-size: 1.2rem; color: #065f46;">R$ {selectedUserForEdit.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong>
                        </div>
                        <div style="background: #f8fafc; border: 1px solid #cbd5e1; padding: 10px;">
                            <span style="font-size: 0.58rem; color: #475569; font-weight: 700; display: block;">NÍVEL ATUAL DE CARREIRA</span>
                            <strong style="font-size: 1.2rem; color: #0f172a;">NÍVEL {selectedUserForEdit.nivel} (+{((selectedUserForEdit.nivel - 1) * 5)}% BÔNUS)</strong>
                        </div>
                    </div>

                    <div style="border: 1px solid #cbd5e1; padding: 12px; background: #f8fafc; margin-bottom: 10px;">
                        <label for="draw-bal-inp" style="font-size: 0.62rem; font-weight: 700; color: #0f172a; display: block; margin-bottom: 4px;">INJETAR OU REMOVER SALDO EM R$:</label>
                        <div style="display: flex; gap: 8px;">
                            <input id="draw-bal-inp" type="number" placeholder="Valor R$" bind:value={balanceInputVal} style="flex: 1; padding: 6px 8px; font-family: monospace; font-size: 0.8rem; border: 1px solid #94a3b8;" />
                            <button on:click={() => handleAdjustBalance(true)} style="background: #047857; color: #fff; font-size: 0.7rem; font-weight: 700; padding: 6px 12px; border: 1px solid #000; cursor: pointer;">+ CRÉDITO</button>
                            <button on:click={() => handleAdjustBalance(false)} style="background: #b91c1c; color: #fff; font-size: 0.7rem; font-weight: 700; padding: 6px 12px; border: 1px solid #000; cursor: pointer;">- DÉBITO</button>
                        </div>
                    </div>

                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px;">
                        <div style="border: 1px solid #cbd5e1; padding: 12px; background: #fff;">
                            <label for="draw-lvl-inp" style="font-size: 0.6rem; font-weight: 700; color: #0f172a; display: block; margin-bottom: 4px;">DEFINIR NÍVEL DE CARREIRA DIRETO:</label>
                            <div style="display: flex; gap: 6px;">
                                <input id="draw-lvl-inp" type="number" min="1" max="50" bind:value={levelInputVal} style="width: 80px; padding: 4px; font-size: 0.8rem; border: 1px solid #94a3b8;" />
                                <button on:click={handleSetLevel} style="background: #1e3a8a; color: #fff; font-size: 0.65rem; font-weight: 700; padding: 6px; border: 1px solid #000; cursor: pointer; flex: 1;">ALTERAR NÍVEL</button>
                            </div>
                        </div>

                        <div style="border: 1px solid #cbd5e1; padding: 12px; background: #fff;">
                            <span style="font-size: 0.6rem; font-weight: 700; color: #0f172a; display: block; margin-bottom: 4px;">TRAVA INDIVIDUAL DE RETIRADA PIX:</span>
                            <button 
                                on:click={() => toggleUserLock(selectedUserForEdit)}
                                style="width: 100%; padding: 6px; font-size: 0.65rem; font-weight: 700; border: 1px solid #000; background: {selectedUserForEdit.withdrawLocked ? '#047857' : '#b91c1c'}; color: #fff; cursor: pointer;"
                            >
                                {selectedUserForEdit.withdrawLocked ? 'LIBERAR SAQUE DO CLIENTE' : 'TRAVAR SAQUE DO CLIENTE'}
                            </button>
                        </div>
                    </div>

                    <div style="border: 1px solid #cbd5e1; padding: 12px; background: #fff; margin-bottom: 14px;">
                        <h4 style="font-size: 0.68rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 8px 0;">AÇÕES ESPECIAIS & CONCESSÃO DE FROTA BÔNUS:</h4>
                        
                        <div style="display: flex; gap: 8px; flex-wrap: wrap;">
                            <button on:click={() => handleAddBonusMachine('trator_d8t')} style="flex: 1; padding: 8px; font-size: 0.62rem; font-weight: 700; background: #ffb200; color: #000; border: 1px solid #000; cursor: pointer;">
                                + INJETAR TRATOR D8T NA GARAGEM
                            </button>
                            <button on:click={() => handleAddBonusMachine('cat_320')} style="flex: 1; padding: 8px; font-size: 0.62rem; font-weight: 700; background: #1e3a8a; color: #fff; border: 1px solid #000; cursor: pointer;">
                                + INJETAR ESCAVADEIRA CAT 320
                            </button>
                        </div>

                        <div style="display: flex; gap: 8px; margin-top: 8px; flex-wrap: wrap;">
                            <button on:click={handleResetUserSpin} style="flex: 1; padding: 6px; font-size: 0.62rem; font-weight: 700; background: #f8fafc; color: #0f172a; border: 1px solid #cbd5e1; cursor: pointer;">
                                RESETAR GIRO DA ROLETA HOJE
                            </button>
                            <button on:click={() => toggleReferralSuspension(selectedUserForEdit)} style="flex: 1; padding: 6px; font-size: 0.62rem; font-weight: 700; background: {selectedUserForEdit.referralBonusSuspended ? '#047857' : '#b45309'}; color: #fff; border: 1px solid #000; cursor: pointer;">
                                {selectedUserForEdit.referralBonusSuspended ? 'REATIVAR COMISSÃO 5%' : 'SUSPENDER COMISSÃO DE AFILIADO'}
                            </button>
                        </div>
                    </div>

                    <button on:click={() => selectedUserForEdit = null} style="width: 100%; background: #0f172a; color: #fff; font-size: 0.75rem; font-weight: 700; padding: 10px; border: none; cursor: pointer; text-transform: uppercase;">
                        CONCLUIR INSPEÇÃO E SALVAR
                    </button>

                </div>
            </div>
        {/if}

        <!-- Rodapé do ERP -->
        <footer style="height: 32px; background: #ffffff; border-top: 1px solid #e2e8f0; padding: 0 24px; display: flex; align-items: center; justify-content: space-between; font-family: monospace; font-size: 0.62rem; color: #64748b; flex-shrink: 0;">
            <div>CATBRASIL ERP v3.0 · ARQUITETURA 100% MODULAR E DESACOPLADA · CONEXÃO CRIPTOGRAFADA SSL</div>
            <div>AUTENTICADO COMO GESTOR DA BANCA</div>
        </footer>
    </div>
</div>
{/if}
