<script>
    import { onMount } from 'svelte';
    import { gestaoService } from '../../services/gestaoService.js';
    import { api } from '../../services/api.js';
    import TOTPSetup from '@catbrasil/ui/components/TOTPSetup.svelte';

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
    import ConfiguracoesTab from './tabs/ConfiguracoesTab.svelte';
    import LogsTab from './tabs/LogsTab.svelte';

    export let users = [];
    export let transactions = [];
    export let onCloseErp = () => {};
    export let onShowToast = () => {};

    // Estado Autenticação do Módulo ERP
    let isAuthenticated = gestaoService.isErpAuthenticated();
    let showERP2FASetup = false;
    let erpTwoFactorEnabled = false;
    let activeTab = 'metrics'; // 'metrics' | 'users' | 'withdrawals' | 'roulette' | 'events' | 'commission' | 'frota' | 'config'
    let isMobileMenuOpen = false;
    let gestaoConfig = {
        gateway: { name: "Gateway PIX Padrão", depositFeePercent: 1.5, withdrawFeeFixed: 1.00, pixExpirationMinutes: 15 },
        autoApproveEnabled: false,
        autoApproveWithdrawalsUnder: 200,
        globalWithdrawalsLocked: false
    };
    let events = [];
    let rouletteSlices = [];
    let machines = [];
    let bankMetrics = {};
    let queueMetrics = {};
    let rouletteMath = { totalProb: 100, expectedValueEV: 0, isProbValid: true };

    let clockDisplay = "";
    let searchQuery = "";
    let statusFilter = "todos"; 
    let withdrawQueueFilter = "pendentes";

    // Modal/Drawer de Gestão Individual do Operador
    let selectedUserForEdit = null;
    let balanceInputVal = "";
    let levelInputVal = 1;

    // Configuração de Gateway PIX & SuitPay
    let gwName = "SuitPay Oficial";
    let gwDepositFee = 1.5;
    let gwWithdrawFee = 1.00;
    let gwPixExpMinutes = 15;
    let suitpayClientId = "";
    let suitpayClientSecret = "";
    let suitpaySandbox = false;
    let suitpayCallbackUrl = "";
    let suitpayEnabled = false;

    // Novo Comunicado Rich Media
    let newTitle = "";
    let newCategory = "CAMPANHA DA GESTÃO";
    let newContent = "";
    let newImageUrl = "";
    let newCtaLink = "";
    let newCtaText = "ACESSAR AGORA";
    let newShowInWelcome = false;

    onMount(() => {
        if (isAuthenticated) { refreshData(); loadERP2FAStatus(); }
        updateClock();
        const interval = setInterval(updateClock, 1000);
        return () => clearInterval(interval);
    });

    async function loadERP2FAStatus() {
        try {
            const profile = await api.get('/admin/profile');
            erpTwoFactorEnabled = Boolean(profile.twoFactorEnabled);
        } catch (error) {
            if (error.status === 401 || error.status === 403) { gestaoService.logoutErp(); isAuthenticated = false; return; }
            onShowToast(error.message || 'Não foi possível validar o 2FA do gestor.', 'error');
        }
    }

    function updateClock() {
        const now = new Date();
        clockDisplay = now.toLocaleDateString('pt-BR') + ' ' + now.toLocaleTimeString('pt-BR');
    }

    async function refreshData() {
        try {
            const [
                usersRes,
                txsRes,
                rouletteRes,
                dreRes,
                annRes,
                cfgRes,
                catalogRes
            ] = await Promise.allSettled([
                api.getAdminUsers(),
                api.getAdminTransactions(),
                api.getRouletteConfig(),
                api.getAdminDre(),
                api.getAdminAnnouncements(),
                api.getSystemConfig(),
                api.getStoreMachines()
            ]);

            if (usersRes.status === 'fulfilled' && Array.isArray(usersRes.value)) {
                users = usersRes.value;
            }
            if (txsRes.status === 'fulfilled' && Array.isArray(txsRes.value)) {
                transactions = txsRes.value;
            }
            if (rouletteRes.status === 'fulfilled' && Array.isArray(rouletteRes.value) && rouletteRes.value.length > 0) {
                rouletteSlices = rouletteRes.value;
            }
            if (annRes.status === 'fulfilled' && Array.isArray(annRes.value)) {
                events = annRes.value;
            }
            if (catalogRes.status === 'fulfilled' && Array.isArray(catalogRes.value) && catalogRes.value.length > 0) {
                machines = catalogRes.value;
            }
            if (cfgRes.status === 'fulfilled' && cfgRes.value && typeof cfgRes.value === 'object') {
                gestaoConfig = { ...gestaoConfig, ...cfgRes.value };
                if (cfgRes.value.gateway) {
                    gwName = cfgRes.value.gateway.name || gwName;
                    gwDepositFee = cfgRes.value.gateway.depositFeePercent ?? gwDepositFee;
                    gwWithdrawFee = cfgRes.value.gateway.withdrawFeeFixed ?? gwWithdrawFee;
                    gwPixExpMinutes = cfgRes.value.gateway.pixExpirationMinutes ?? gwPixExpMinutes;
                    suitpayClientId = cfgRes.value.gateway.suitpayClientId || "";
                    suitpayClientSecret = cfgRes.value.gateway.suitpayClientSecret || "";
                    suitpaySandbox = !!cfgRes.value.gateway.suitpaySandbox;
                    suitpayCallbackUrl = cfgRes.value.gateway.suitpayCallbackUrl || "";
                    suitpayEnabled = !!cfgRes.value.gateway.suitpayEnabled;
                }
            }

            const serverDre = dreRes.status === 'fulfilled' ? dreRes.value : null;
            const dreData = serverDre?.data || serverDre;
            if (dreData && Object.keys(dreData).length > 0) {
                bankMetrics = { ...gestaoService.getBankMetrics(users, transactions), ...dreData };
            } else {
                bankMetrics = gestaoService.getBankMetrics(users, transactions);
            }
        } catch (e) {
            console.warn("Erro ao sincronizar dados com PostgreSQL:", e.message);
            bankMetrics = gestaoService.getBankMetrics(users, transactions);
        }

        queueMetrics = gestaoService.getWithdrawalQueueMetrics(transactions);
        rouletteMath = gestaoService.getRouletteMathematicalMetrics(rouletteSlices);

        if (gestaoConfig.gateway) {
            gwName = gestaoConfig.gateway.name || "Gateway PIX Padrão";
            gwDepositFee = gestaoConfig.gateway.depositFeePercent || 1.5;
            gwWithdrawFee = gestaoConfig.gateway.withdrawFeeFixed || 1.00;
            gwPixExpMinutes = gestaoConfig.gateway.pixExpirationMinutes || 15;
        }
    }

    // Trava de Emergência Global
    async function toggleGlobalLock() {
        const newLockState = !gestaoConfig.globalWithdrawalsLocked;
        try {
            await api.toggleGlobalWithdrawLock(newLockState);
            gestaoConfig = { ...gestaoConfig, globalWithdrawalsLocked: newLockState };
            onShowToast(newLockState ? "SAQUES GLOBAIS TRAVADOS EM MASSA!" : "SAQUES GLOBAIS LIBERADOS COM SUCESSO!", newLockState ? "error" : "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar trava de emergência", "error");
        }
    }

    async function handleSaveGatewaySettings() {
        try {
            await api.updateGatewaySettings({
                name: gwName,
                depositFeePercent: parseFloat(gwDepositFee),
                withdrawFeeFixed: parseFloat(gwWithdrawFee),
                pixExpirationMinutes: parseInt(gwPixExpMinutes),
                collectionGraceMinutes: parseInt(gestaoConfig.collectionGraceMinutes || 10),
                suitpayClientId: (suitpayClientId || '').trim(),
                suitpayClientSecret: (suitpayClientSecret || '').trim(),
                suitpaySandbox: !!suitpaySandbox,
                suitpayCallbackUrl: (suitpayCallbackUrl || '').trim(),
                suitpayEnabled: !!suitpayEnabled
            });
            onShowToast("Configurações do Gateway PIX / SuitPay salvas no PostgreSQL!", "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao salvar configurações do Gateway", "error");
        }
    }

    async function handleSaveAutoApproveSettings() {
        try {
            await api.updateAutoApproveSettings(
                gestaoConfig.autoApproveEnabled,
                parseFloat(gestaoConfig.autoApproveWithdrawalsUnder || 200)
            );
            onShowToast("Regras de Aprovação Automática atualizadas no banco!", "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao salvar aprovação automática", "error");
        }
    }

    // AÇÕES DA ROLETA DIÁRIA (SALVOS DIRETO NO BACKEND)
    async function handleSaveRouletteSettings() {
        try {
            await api.updateRouletteConfig(rouletteSlices);
            onShowToast("Matriz de Probabilidades salva no PostgreSQL!", "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao salvar regras da roleta", "error");
        }
    }

    async function handleSaveMachinesCatalog() {
        try {
            await api.saveFleetCatalog(machines);
            onShowToast("Catálogo de Frota salvo no PostgreSQL com sucesso!", "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao salvar catálogo de frotas", "error");
        }
    }

    function handleNormalizeProbabilities() {
        let currentSum = 0;
        rouletteSlices.forEach(s => currentSum += (parseFloat(s.probability) || 0));
        const diff = 100.0 - (currentSum - parseFloat(rouletteSlices[rouletteSlices.length - 1].probability));
        rouletteSlices[rouletteSlices.length - 1].probability = parseFloat(Math.max(0, diff).toFixed(1));
        rouletteMath = gestaoService.getRouletteMathematicalMetrics(rouletteSlices);
        onShowToast("Probabilidades normalizadas para fechar em 100%!", "win");
    }

    function handleApplyPresetMaxProfit() {
        rouletteSlices = [
            { id: 1, label: "🧩 Motor C15", amount: 0, probability: 10.0, color: "#ffb200", textColor: "#111" },
            { id: 2, label: "🧩 Esteira CAT", amount: 0, probability: 10.0, color: "#111111", textColor: "#fff" },
            { id: 3, label: "🧩 Filtro Óleo", amount: 0, probability: 10.0, color: "#ffb200", textColor: "#111" },
            { id: 4, label: "🧩 Turbo Garrett", amount: 0, probability: 5.0, color: "#111111", textColor: "#fff" },
            { id: 5, label: "📦 Caixa CAT", amount: 0, probability: 5.0, color: "#ffb200", textColor: "#111" },
            { id: 6, label: "💰 R$ 5,00", amount: 5.00, probability: 60.0, color: "#2e7d32", textColor: "#fff" }
        ];
        rouletteMath = gestaoService.getRouletteMathematicalMetrics(rouletteSlices);
        onShowToast("Preset LUCRO MÁXIMO (EV R$ 3.00) aplicado! Clique em salvar.", "win");
    }

    function handleApplyPresetViral() {
        rouletteSlices = [
            { id: 1, label: "🧩 Motor C15", amount: 0, probability: 25.0, color: "#ffb200", textColor: "#111" },
            { id: 2, label: "🧩 Esteira CAT", amount: 0, probability: 25.0, color: "#111111", textColor: "#fff" },
            { id: 3, label: "🧩 Filtro Óleo", amount: 0, probability: 25.0, color: "#ffb200", textColor: "#111" },
            { id: 4, label: "🧩 Turbo Garrett", amount: 0, probability: 15.0, color: "#111111", textColor: "#fff" },
            { id: 5, label: "📦 Caixa CAT", amount: 0, probability: 5.0, color: "#ffb200", textColor: "#111" },
            { id: 6, label: "💰 R$ 5,00", amount: 5.00, probability: 5.0, color: "#2e7d32", textColor: "#fff" }
        ];
        rouletteMath = gestaoService.getRouletteMathematicalMetrics(rouletteSlices);
        onShowToast("Preset ATRAÇÃO VIRAL aplicado! Clique em salvar.", "win");
    }

    // AÇÕES DA FILA DE SAQUES PIX NO SERVIDOR
    async function handleApproveWithdrawal(txId) {
        try {
            const res = await api.approveWithdrawal(txId);
            onShowToast(res.message || `Saque #${txId} APROVADO no PostgreSQL!`, "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao aprovar saque no servidor", "error");
        }
    }

    async function handleRejectWithdrawal(txId, lockAccount = false) {
        try {
            const res = await api.rejectWithdrawal(txId, lockAccount ? "Bloqueado por suspeita de fraude" : "Rejeitado pela diretoria", lockAccount);
            onShowToast(res.message || `Saque #${txId} REJEITADO e estornado!`, "error");
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
        try {
            await api.adjustUserBalance(selectedUserForEdit.id, val, isAdd ? 'add' : 'subtract', 'Ajuste Manual Gestão ERP');
            onShowToast(`Saldo de ${selectedUserForEdit.username} alterado com sucesso!`, "win");
            balanceInputVal = "";
            await refreshData();
            if (selectedUserForEdit) {
                const refreshed = users.find(u => u.id === selectedUserForEdit.id);
                if (refreshed) selectedUserForEdit = { ...refreshed };
            }
        } catch (e) {
            onShowToast(e.message || "Erro ao atualizar saldo no PostgreSQL", "error");
        }
    }

    async function toggleUserLock(userObj) {
        const target = userObj || selectedUserForEdit;
        try {
            await api.toggleWithdrawLock(target.id);
            onShowToast(`Status de trava de ${target.username} alterado com sucesso!`, "win");
            await refreshData();
            if (selectedUserForEdit && selectedUserForEdit.id === target.id) {
                const refreshed = users.find(u => u.id === target.id);
                if (refreshed) selectedUserForEdit = { ...refreshed };
            }
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar trava no PostgreSQL", "error");
        }
    }

    async function toggleReferralSuspension(userObj) {
        const target = userObj || selectedUserForEdit;
        try {
            await api.toggleCommissionLock(target.id);
            onShowToast(`Comissões de ${target.username} atualizadas com sucesso!`, "win");
            await refreshData();
            if (selectedUserForEdit && selectedUserForEdit.id === target.id) {
                const refreshed = users.find(u => u.id === target.id);
                if (refreshed) selectedUserForEdit = { ...refreshed };
            }
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar comissão no PostgreSQL", "error");
        }
    }

    async function handleSetLevel() {
        const lvl = parseInt(levelInputVal);
        if (isNaN(lvl) || lvl < 1) {
            onShowToast("Nível inválido!", "error");
            return;
        }
        try {
            await api.updateUserLevel(selectedUserForEdit.id, lvl);
            selectedUserForEdit.nivel = lvl;
            onShowToast(`Nível de ${selectedUserForEdit.username} alterado para NÍVEL ${lvl}!`, "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar nível no PostgreSQL", "error");
        }
    }

    async function handleAddBonusMachine(machineModel) {
        if (!selectedUserForEdit) return;
        try {
            await api.injectMachineBonus(selectedUserForEdit.id, machineModel);
            onShowToast(`Máquina bônus concedida à garagem de ${selectedUserForEdit.username}!`, "win");
            await refreshData();
        } catch (e) {
            onShowToast("Erro ao conceder máquina: " + e.message, "error");
        }
    }

    // Publicar Aviso
    async function handleCreateEvent() {
        if (!newTitle || !newContent) {
            onShowToast("Preencha o título e o conteúdo do comunicado!", "error");
            return;
        }
        try {
            await api.createAnnouncement({
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
            onShowToast("Comunicado Rich Media publicado no PostgreSQL!", "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao publicar comunicado", "error");
        }
    }

    async function handleToggleEventActive(id) {
        try {
            await api.toggleAnnouncement(id);
            onShowToast("Status do comunicado alterado no PostgreSQL!", "info");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao alterar comunicado", "error");
        }
    }

    async function handleDeleteEvent(id) {
        try {
            await api.deleteAnnouncement(id);
            onShowToast("Comunicado removido do PostgreSQL.", "info");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao deletar comunicado", "error");
        }
    }

    // Salvar Comissões
    async function handleSaveCommission() {
        try {
            await api.updateReferralsSettings({
                referralCommissionPercent: parseFloat(gestaoConfig.referralCommissionPercent || 5.0),
                requireReferralsForWithdrawal: gestaoConfig.requireReferralsForWithdrawal,
                requiredReferralsCount: parseInt(gestaoConfig.requiredReferralsCount || 3),
                bonusPerMutiraoAmount: parseFloat(gestaoConfig.bonusPerMutiraoAmount || 50.0),
                antiSelfReferralLock: gestaoConfig.antiSelfReferralLock !== false
            });
            onShowToast("Regras de comissão salvas no PostgreSQL!", "win");
            await refreshData();
        } catch (e) {
            onShowToast(e.message || "Erro ao salvar comissões", "error");
        }
    }

    function handleLogoutErp() {
        showERP2FASetup = false;
        gestaoService.logoutErp();
        isAuthenticated = false;
        onCloseErp();
    }
</script>

{#if !isAuthenticated}
    <ErpLoginScreen 
        onLoginSuccess={() => { isAuthenticated = true; refreshData(); loadERP2FAStatus(); }} 
        onCancel={onCloseErp} 
        {onShowToast} 
    />
{:else}
<!-- STANDALONE DESKTOP SOFTWARE SHELL (DESIGN SYSTEM UNISISM/CATBRASIL B2G) -->
<div style="position: fixed; inset: 0; z-index: 999999; background: #0f172a; display: flex; font-family: 'Inter', system-ui, sans-serif; color: #0f172a; width: 100vw; height: 100vh; overflow: hidden;">
    
    <!-- SIDEBAR ESQUERDA FIXA (w-64) -->
    <aside class="sidebar-desktop {isMobileMenuOpen ? 'sidebar-mobile-open' : ''}" style="width: 256px; background: #0f172a; color: #ffffff; display: flex; flex-direction: column; border-right: 1px solid #1e293b; flex-shrink: 0; user-select: none;">
        
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
            {#if !erpTwoFactorEnabled}
                <button type="button" on:click={() => showERP2FASetup = true} style="background:#1e293b; color:#ffb200; border:1px solid #ffb200; padding:4px 8px; font-size:.62rem; cursor:pointer;">ATIVAR 2FA</button>
            {/if}
            
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
                on:click={() => { activeTab = 'frota'; isMobileMenuOpen = false; refreshData(); }}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'frota' ? '#ffb200' : 'transparent'}; background: {activeTab === 'frota' ? '#1e293b' : 'transparent'}; color: {activeTab === 'frota' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>FROTA & MÁQUINAS</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">7</kbd>
            </button>

            <button 
                on:click={() => { activeTab = 'config'; isMobileMenuOpen = false; refreshData(); }}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'config' ? '#ffb200' : 'transparent'}; background: {activeTab === 'config' ? '#1e293b' : 'transparent'}; color: {activeTab === 'config' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>CONFIGURAÇÕES & TRAVAS</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">8</kbd>
            </button>

            <button 
                on:click={() => { activeTab = 'logs'; isMobileMenuOpen = false; refreshData(); }}
                style="width: 100%; display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; text-align: left; font-family: monospace; font-size: 0.72rem; font-weight: 700; border: none; border-left: 3px solid {activeTab === 'logs' ? '#ffb200' : 'transparent'}; background: {activeTab === 'logs' ? '#1e293b' : 'transparent'}; color: {activeTab === 'logs' ? '#ffb200' : '#cbd5e1'}; cursor: pointer;"
            >
                <span>AUDITORIA & SEGURANÇA</span>
                <kbd style="font-size: 0.55rem; padding: 1px 4px; background: #0f172a; border: 1px solid #334155; color: #94a3b8;">9</kbd>
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

    <!-- PAINEL PRINCIPAL DESKTOP / MOBILE -->
    <div style="flex: 1; display: flex; flex-direction: column; min-width: 0; background: #f8fafc; overflow: hidden; position: relative;">
        
        <!-- Header Superior Responsivo -->
        <header style="min-height: 52px; background: #ffffff; border-bottom: 1px solid #e2e8f0; padding: 8px 16px; display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px; flex-shrink: 0;">
            <div style="display: flex; align-items: center; gap: 8px;">
                <button 
                    class="mobile-menu-btn"
                    on:click={() => isMobileMenuOpen = !isMobileMenuOpen}
                    style="background: #0f172a; color: #ffb200; border: 1px solid #000; padding: 5px 10px; font-family: monospace; font-size: 0.75rem; font-weight: 900; cursor: pointer; display: none;"
                >
                    {isMobileMenuOpen ? '✕ FECHAR' : '☰ MENU'}
                </button>
                <span style="font-family: monospace; font-size: 0.72rem; color: #64748b; text-transform: uppercase;">CATBRASIL / GESTÃO /</span>
                <h2 style="font-family: monospace; font-size: 0.8rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">
                    {activeTab === 'metrics' ? 'DRE AUTOMÁTICO & AUDITORIA DE USO DOS OPERADORES' : 
                     activeTab === 'users' ? 'GESTÃO & AUDITORIA AVANÇADA DE OPERADORES' : 
                     activeTab === 'withdrawals' ? 'FILA DE APROVAÇÃO E AUDITORIA DE SAQUES PIX' : 
                     activeTab === 'roulette' ? 'ROLETA & MATRIZ DE PROBABILIDADES MATEMÁTICAS' : 
                     activeTab === 'events' ? 'PUBLICAR COMUNICADOS DA GESTÃO' : 
                     activeTab === 'commission' ? 'REGRAS DE COMISSÃO & TRAVAS' :
                     activeTab === 'frota' ? 'CATÁLOGO DE FROTA DE MÁQUINAS' : 'CONFIGURAÇÕES GLOBAIS DA BANCA & KILL SWITCH'}
                </h2>
            </div>

            <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">
                <span style="font-family: monospace; font-size: 0.62rem; font-weight: 700; padding: 3px 8px; border: 1px solid {gestaoConfig.globalWithdrawalsLocked ? '#b91c1c' : '#047857'}; background: {gestaoConfig.globalWithdrawalsLocked ? '#fef2f2' : '#ecfdf5'}; color: {gestaoConfig.globalWithdrawalsLocked ? '#991b1b' : '#065f46'}; text-transform: uppercase;">
                    {gestaoConfig.globalWithdrawalsLocked ? 'SAQUES GLOBAIS TRAVADOS' : 'SAQUES GLOBAIS LIBERADOS'}
                </span>
                <span style="font-family: monospace; font-size: 0.72rem; color: #475569; border-left: 1px solid #e2e8f0; padding-left: 12px;">
                    {clockDisplay}
                </span>
            </div>
        </header>

        <!-- Barra de Navegação Rápida Horizontal (Mobile Friendly) -->
        <nav class="mobile-tabs-bar" style="background: #0f172a; padding: 6px 8px; display: flex; gap: 6px; overflow-x: auto; border-bottom: 1px solid #334155; flex-shrink: 0; white-space: nowrap;">
            <button on:click={() => { activeTab = 'metrics'; refreshData(); }} class="tab-chip {activeTab === 'metrics' ? 'active' : ''}">📊 DRE</button>
            <button on:click={() => activeTab = 'users'} class="tab-chip {activeTab === 'users' ? 'active' : ''}">👥 OPERADORES ({users.length})</button>
            <button on:click={() => { activeTab = 'withdrawals'; refreshData(); }} class="tab-chip {activeTab === 'withdrawals' ? 'active' : ''}">💸 SAQUES {#if queueMetrics.pendingCount > 0}<span style="background: #b91c1c; color:#fff; padding:0 4px; border-radius:2px;">{queueMetrics.pendingCount}</span>{/if}</button>
            <button on:click={() => { activeTab = 'roulette'; refreshData(); }} class="tab-chip {activeTab === 'roulette' ? 'active' : ''}">🎰 ROLETA</button>
            <button on:click={() => activeTab = 'events'} class="tab-chip {activeTab === 'events' ? 'active' : ''}">📢 COMUNICADOS</button>
            <button on:click={() => activeTab = 'commission'} class="tab-chip {activeTab === 'commission' ? 'active' : ''}">🤝 COMISSÕES</button>
            <button on:click={() => { activeTab = 'frota'; refreshData(); }} class="tab-chip {activeTab === 'frota' ? 'active' : ''}">🚜 FROTA</button>
            <button on:click={() => activeTab = 'config'} class="tab-chip {activeTab === 'config' ? 'active' : ''}">⚙️ CONFIG</button>
        </nav>

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

            <!-- ABA 8: CONFIGURAÇÕES GLOBAIS DA BANCA & KILL SWITCH -->
            {#if activeTab === 'config'}
                <ConfiguracoesTab 
                    {gestaoConfig}
                    bind:gwDepositFee
                    bind:gwWithdrawFee
                    bind:gwPixExpMinutes
                    bind:suitpayClientId
                    bind:suitpayClientSecret
                    bind:suitpaySandbox
                    bind:suitpayCallbackUrl
                    bind:suitpayEnabled
                    onToggleGlobalLock={toggleGlobalLock}
                    onSaveAutoApproveSettings={handleSaveAutoApproveSettings}
                    onSaveGatewaySettings={handleSaveGatewaySettings}
                />
            {/if}

            <!-- ABA 9: TRILHA DE AUDITORIA & SEGURANÇA IMUTÁVEL -->
            {#if activeTab === 'logs'}
                <LogsTab {onShowToast} />
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
<TOTPSetup
    isOpen={showERP2FASetup}
    setup={() => api.post('/auth/2fa/setup', {}, { token: api.getERPToken() })}
    verify={(code) => api.post('/auth/2fa/verify', { code }, { token: api.getERPToken() })}
    onEnabled={() => { erpTwoFactorEnabled = true; onShowToast('2FA do gestor ativado.', 'win'); }}
    onclose={() => showERP2FASetup = false}
/>
{/if}

<style>
    .tab-chip {
        background: #1e293b;
        color: #cbd5e1;
        font-family: monospace;
        font-size: 0.65rem;
        font-weight: 700;
        padding: 6px 12px;
        border: 1px solid #334155;
        cursor: pointer;
        text-transform: uppercase;
        flex-shrink: 0;
        transition: all 0.15s;
    }
    .tab-chip:hover {
        background: #334155;
        color: #fff;
    }
    .tab-chip.active {
        background: #ffb200;
        color: #000;
        border-color: #000;
        font-weight: 900;
    }
    @media (max-width: 768px) {
        .mobile-menu-btn {
            display: inline-block !important;
        }
        .sidebar-desktop {
            display: none !important;
        }
        .sidebar-mobile-open {
            display: flex !important;
            position: fixed;
            inset: 0;
            z-index: 1000000;
            width: 280px;
            max-width: 85vw;
        }
    }
    @media (min-width: 769px) {
        .mobile-tabs-bar {
            display: none !important;
        }
    }
</style>

