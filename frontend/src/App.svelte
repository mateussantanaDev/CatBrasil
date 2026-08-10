<script>
    import { onMount } from 'svelte';
    import { api } from './services/api.js';

    import { websocketService } from './services/websocketService.js';

    // Componentes Modulares
    import LoginScreen from './components/LoginScreen.svelte';
    import Header from './components/Header.svelte';
    import BottomNav from './components/BottomNav.svelte';
    import GaragemTab from './components/GaragemTab.svelte';
    import LojaTab from './components/LojaTab.svelte';
    import RankingTab from './components/RankingTab.svelte';
    import ConfiguracoesTab from './components/ConfiguracoesTab.svelte';
    import ToastContainer from './components/ToastContainer.svelte';
    
    // Lazy Loading Dinâmico do Módulo ERP de Gestão (Code Splitting)
    let GestaoErpComponent = null;

    async function toggleGestaoErpMode() {
        if (!showGestaoErp) {
            if (!GestaoErpComponent) {
                const module = await import('./components/gestao/GestaoErp.svelte');
                GestaoErpComponent = module.default;
            }
            showGestaoErp = true;
        } else {
            showGestaoErp = false;
        }
    }

    // Modais
    import PixModal from './components/modals/PixModal.svelte';
    import SaqueModal from './components/modals/SaqueModal.svelte';
    import RoletaModal from './components/modals/RoletaModal.svelte';
    import EventsModal from './components/modals/EventsModal.svelte';
    import InviteModal from './components/modals/InviteModal.svelte';
    import AnnouncementsModal from './components/modals/AnnouncementsModal.svelte';
    import RankingModal from './components/modals/RankingModal.svelte';
    import PecasModal from './components/modals/PecasModal.svelte';

    // Estado de Autenticação Real do Backend
    let token = localStorage.getItem('cb_token') || "";
    let isLoggedIn = false;
    let user = {
        id: "",
        username: "",
        phone: "",
        balance: 0.00,
        nivel: 1,
        xp: 0,
        securityPassword: "",
        coletasCompleted: 0,
        tratorRented: false,
        alreadySpunToday: false,
        referralBonusSuspended: false,
        withdrawLocked: false,
        yieldBalanceAvailable: 0.00,
        requiredRollover: 0.00,
        currentRollover: 0.00,
        partsInventory: { motorC15: 0, esteiraReforcada: 0, filtroOleo: 0, turboGarrett: 0 }
    };

    let users = [];
    let garage = [];
    let referrals = [];
    let transactions = [];

    // Controladores de Abas e Modais
    let currentTab = 'garagem';
    let txFilter = 'todos';
    let inputPassword = '';

    // Modais Visibilidade
    let showPixModal = false;
    let showWithdrawModal = false;
    let showRouletteModal = false;
    let showEventsModal = false;
    let showInviteModal = false;
    let showAnnouncementsModal = true;
    let showRankingModal = false;
    let showPecasModal = false;
    let showGestaoErp = false;

    // Dados Ranking
    let rankingLeaderboard = [];
    let rankingUserRank = null;
    let rankingSeasonEndsInHours = 48;

    function handleOpenRanking() {
        currentTab = 'ranking';
    }

    async function handleAssembleMachine() {
        try {
            const data = await api.assembleMachineFromParts(token);
            if (data && data.success) {
                showToast(data.message, "win");
                showPecasModal = false;
                await loadUserData();
            }
        } catch (e) {
            showToast(e.message, "error");
        }
    }

    // Dados PIX & Saque
    let currentPixValue = 0;
    let pixCodeString = "";
    let pixTimerSeconds = 900;
    let withdrawAmount = "";
    let withdrawPixKey = "";
    let withdrawSecurityPass = "";

    // Roleta
    let isSpinning = false;
    let rouletteRotation = 0;
    let roulettePrizeMessage = "";

    // Timers
    let countdownDisplay = "10:00";
    let countdownSeconds = 600;

    // Toast System
    let toasts = [];
    function showToast(message, type = '') {
        const id = Date.now() + Math.random();
        toasts = [...toasts, { id, message, type }];
        setTimeout(() => removeToast(id), 3500);
    }
    function removeToast(id) {
        toasts = toasts.filter(t => t.id !== id);
    }

    // Sanfonas
    let submenuOpen = {
        niveis: false,
        conta: true,
        convidados: false,
        historico: false,
        eventos: false
    };

    function toggleSubmenu(key) {
        submenuOpen[key] = !submenuOpen[key];
    }

    // Login Handler
    async function handleLoginSuccess(userToken, userData) {
        token = userToken;
        localStorage.setItem('cb_token', userToken);
        user = userData;
        isLoggedIn = true;
        showAnnouncementsModal = true;
        await loadUserData();
    }

    async function loadUserData() {
        if (!token) {
            isLoggedIn = false;
            return;
        }
        try {
            const loggedUser = await api.getProfile(token);
            if (loggedUser) {
                user = loggedUser;
                isLoggedIn = true;
            }

            const machs = await api.getGarage(token);
            if (machs) garage = machs;

            const txs = await api.getTransactions(token);
            if (txs) transactions = txs;

            const refs = await api.getReferrals(token);
            if (refs) referrals = refs;

            // Conecta ao servidor WebSocket em tempo real para escutar depósitos PIX e coletas
            websocketService.connect(user.id, (event) => {
                loadUserData();
                if (event.type === 'PIX_CONFIRMED') {
                    showToast(`⚡ Depósito PIX de R$ ${parseFloat(event.paidAmount).toFixed(2)} confirmado em tempo real!`, "win");
                }
            });
        } catch (e) {
            console.warn("Falha ao sincronizar com backend:", e.message);
            isLoggedIn = false;
        }
    }

    function handleUpdateUserFromGestao(userId, updates) {
        users = users.map(u => u.id === userId ? { ...u, ...updates } : u);
        if (user.id === userId) {
            user = { ...user, ...updates };
        }
    }

    // Ciclo de Vida
    onMount(() => {
        loadUserData();

        // Verificar acesso direto via URL (?erp=true ou #erp)
        if (window.location.search.includes('erp') || window.location.hash.includes('erp')) {
            toggleGestaoErpMode();
        }

        // Atalho de teclado para gestores (Ctrl+Shift+E ou Cmd+Shift+E)
        const handleKeyDown = (e) => {
            if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'E' || e.key === 'e')) {
                e.preventDefault();
                toggleGestaoErpMode();
            }
        };
        window.addEventListener('keydown', handleKeyDown);

        // Timer de Coleta
        const timer = setInterval(() => {
            if (countdownSeconds > 0) {
                countdownSeconds--;
                const m = Math.floor(countdownSeconds / 60);
                const s = countdownSeconds % 60;
                countdownDisplay = `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
            }
        }, 1000);

        return () => {
            window.removeEventListener('keydown', handleKeyDown);
            clearInterval(timer);
        };
    });

    // Ações de Negócio
    async function handleCollectYield(machineId) {
        try {
            const res = await api.collectYield(token, machineId);
            if (res.isExpired) {
                showToast("Janela de coleta expirada! Ciclo reiniciado sem prêmio.", "error");
            } else {
                showToast(res.message || "Coleta efetuada com sucesso!", "win");
            }
            await loadUserData();
        } catch (e) {
            showToast(e.message || "Erro ao coletar rendimento", "error");
        }
    }

    async function handleRentMachine(machineModel) {
        try {
            const res = await api.rentMachine(token, machineModel);
            showToast(res.message || "Novo contrato ativado com sucesso!", "win");
            await loadUserData();
        } catch (e) {
            showToast(`${e.message}`, "error");
        }
    }

    async function handleGeneratePix(amount = 500) {
        try {
            currentPixValue = amount;
            const res = await api.generatePix(token, amount);
            pixCodeString = res.pixCode;
            showPixModal = true;
        } catch (e) {
            showToast(e.message || "Erro ao gerar PIX", "error");
        }
    }

    function copyPixCode() {
        if (navigator.clipboard) navigator.clipboard.writeText(pixCodeString);
        showToast("Código PIX copiado para a área de transferência!", "win");
    }

    async function handleSaveSecurityPassword() {
        if (!inputPassword || inputPassword.length !== 6) {
            showToast("A senha financeira deve conter exatamente 6 dígitos!", "error");
            return;
        }
        try {
            await api.saveSecurityPassword(token, inputPassword);
            inputPassword = "";
            showToast("Senha financeira salva com sucesso!", "win");
            await loadUserData();
        } catch (e) {
            showToast(e.message || "Erro ao salvar senha financeira", "error");
        }
    }

    async function handleSubmitWithdrawal() {
        if (user.withdrawLocked) {
            showToast("Saques bloqueados para a sua conta! Entre em contato com a gestão.", "error");
            return;
        }
        if (!withdrawAmount || parseFloat(withdrawAmount) <= 0) {
            showToast("Informe um valor de saque válido!", "error");
            return;
        }
        if (!withdrawPixKey) {
            showToast("Digite uma chave PIX válida!", "error");
            return;
        }
        try {
            const res = await api.requestWithdrawal(token, parseFloat(withdrawAmount), withdrawPixKey, withdrawSecurityPass);
            showWithdrawModal = false;
            withdrawAmount = "";
            withdrawPixKey = "";
            withdrawSecurityPass = "";
            showToast(res.message || "Saque solicitado com sucesso!", "win");
            await loadUserData();
        } catch (e) {
            showToast(`${e.message}`, "error");
        }
    }

    async function handleSpinRoulette() {
        if (user.alreadySpunToday) {
            showToast("Você já girou a roleta hoje! Tente novamente amanhã.", "error");
            return;
        }
        isSpinning = true;
        try {
            const res = await api.spinRoulette(token);
            const sectorIdx = res.sectorIdx !== undefined ? res.sectorIdx : 0;
            // Ângulos das fatias no SVG: 0: R$ 10, 1: R$ 50, 2: R$ 100, 3: R$ 20, 4: R$ 500, 5: R$ 5
            const sliceTargetAngles = [330, 270, 210, 150, 90, 30];
            const targetAngle = sliceTargetAngles[sectorIdx % 6] || 330;

            const currentMod = rouletteRotation % 360;
            rouletteRotation = rouletteRotation - currentMod + 1800 + targetAngle;

            setTimeout(() => {
                isSpinning = false;
                if (res.user) user = res.user;
                const prizeLabel = res.win ? res.win.label : "Giro realizado";
                roulettePrizeMessage = prizeLabel;
                showToast(`Parabéns! Prêmio conquistado: ${prizeLabel}`, "win");
            }, 3200);
        } catch (e) {
            isSpinning = false;
            showToast(e.message || "Erro ao girar roleta", "error");
        }
    }

    function handleCloseAnnouncements() {
        showAnnouncementsModal = false;
    }

    function copyInviteLink() {
        const link = `https://catbrasil.app/ref/${user.id}`;
        if (navigator.clipboard) navigator.clipboard.writeText(link);
        showToast("Link de convite copiado com sucesso!", "win");
    }

    async function levelUp() {
        try {
            const res = await api.levelUp(token);
            showToast(res.message || `Parabéns! Subiu de nível!`, "win");
            await loadUserData();
        } catch (e) {
            showToast(e.message || "Requisitos incompletos para subir de nível", "error");
        }
    }

    function handleLogout() {
        token = "";
        localStorage.removeItem('cb_token');
        isLoggedIn = false;
        showToast("Turno finalizado com segurança.", "info");
    }

    function simulateReferral(activate) {
        referrals = referrals.map(r => r.nome.includes('Maria') ? { ...r, status: activate ? 'ativo' : 'inativo' } : r);
        user.referralBonusSuspended = !activate;
        showToast(activate ? "✅ Todos os convidados estão ATIVOS!" : "⚠️ Maria Inativada! Bônus de 5% Suspenso.", activate ? "win" : "error");
    }
</script>

<div class="app-container">
    {#if !isLoggedIn}
        <!-- TELA DE LOGIN / REGISTRO BRUTALISTA -->
        <LoginScreen 
            onLoginSuccess={handleLoginSuccess} 
            onShowToast={showToast} 
            onOpenGestao={() => showGestaoErp = true}
        />
    {:else}
        <!-- HEADER -->
        <Header 
            {user} 
            onOpenDeposit={() => handleGeneratePix(500)} 
            onOpenWithdraw={() => showWithdrawModal = true} 
            onOpenRanking={handleOpenRanking} 
            onOpenPecas={() => showPecasModal = true} 
        />

        <!-- CONTEÚDO PRINCIPAL -->
        <main class="content">
            {#if currentTab === 'garagem'}
                <GaragemTab 
                    {user} 
                    {garage} 
                    {countdownDisplay} 
                    onCollectYield={handleCollectYield} 
                />
            {:else if currentTab === 'loja'}
                <LojaTab 
                    {user} 
                    onRentMachine={handleRentMachine} 
                />
            {:else if currentTab === 'ranking'}
                <RankingTab 
                    {user} 
                />
            {:else if currentTab === 'config'}
                <ConfiguracoesTab 
                    {user} 
                    {referrals} 
                    {transactions} 
                    {submenuOpen} 
                    {txFilter} 
                    {inputPassword} 
                    onToggleSubmenu={toggleSubmenu} 
                    onSaveSecurityPassword={handleSaveSecurityPassword} 
                    onCopyInviteLink={copyInviteLink} 
                    onLevelUp={levelUp} 
                    onLogout={handleLogout} 
                    onSetTxFilter={(f) => txFilter = f} 
                    onSimulateReferral={simulateReferral} 
                    onResetRoulette={() => { user.alreadySpunToday = false; showToast("Roleta resetada!", "win"); }} 
                    onFullReset={() => { localStorage.clear(); window.location.reload(); }} 
                />
            {/if}
        </main>

        <!-- FABs Flutuantes (Apenas na Garagem) -->
        {#if currentTab === 'garagem'}
            <button class="fab" on:click={() => showInviteModal = true} aria-label="Abrir Convites" title="Indicações & Convites">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
                    <circle cx="8.5" cy="7" r="4"/>
                    <line x1="20" y1="8" x2="20" y2="14"/>
                    <line x1="17" y1="11" x2="23" y2="11"/>
                </svg>
            </button>
            <button class="floating-events-btn" on:click={() => showEventsModal = true} aria-label="Comunicados e Eventos da Gestão" title="Avisos & Eventos">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/>
                    <path d="M13.73 21a2 2 0 0 1-3.46 0"/>
                </svg>
                {#if referrals.filter(r => r.status === 'ativo').length < 3}
                    <span class="fab-badge">!</span>
                {/if}
            </button>
        {/if}

        <!-- BARRA INFERIOR DE NAVEGAÇÃO -->
        <BottomNav 
            {currentTab} 
            garageCount={garage.length} 
            onSelectTab={(tab) => currentTab = tab} 
        />

        <!-- MODAIS DO SISTEMA -->
        <AnnouncementsModal 
            show={showAnnouncementsModal} 
            {user} 
            {isSpinning} 
            {rouletteRotation} 
            {roulettePrizeMessage} 
            onCloseModal={handleCloseAnnouncements} 
            onSpinRoulette={handleSpinRoulette} 
        />

        <PixModal 
            show={showPixModal} 
            {currentPixValue} 
            {pixCodeString} 
            {pixTimerSeconds} 
            onClose={() => showPixModal = false} 
            onCopyCode={copyPixCode} 
        />

        <SaqueModal 
            show={showWithdrawModal} 
            {user} 
            bind:withdrawAmount 
            bind:withdrawPixKey 
            bind:withdrawSecurityPass 
            onClose={() => showWithdrawModal = false} 
            onSubmitWithdrawal={handleSubmitWithdrawal} 
        />

        <InviteModal 
            show={showInviteModal} 
            {user} 
            {referrals} 
            onClose={() => showInviteModal = false} 
            onCopyLink={copyInviteLink} 
        />

        <EventsModal 
            show={showEventsModal} 
            {referrals} 
            onClose={() => showEventsModal = false} 
            onInviteClick={() => { showEventsModal = false; showInviteModal = true; }} 
        />

        <RankingModal 
            show={showRankingModal} 
            leaderboard={rankingLeaderboard} 
            userRank={rankingUserRank} 
            seasonEndsInHours={rankingSeasonEndsInHours} 
            onClose={() => showRankingModal = false} 
        />

        <PecasModal 
            show={showPecasModal} 
            {user} 
            onClose={() => showPecasModal = false} 
            onAssembleMachine={handleAssembleMachine} 
        />
    {/if}

    <!-- PAINEL STANDALONE DESKTOP DE GESTÃO ERP (LAZY LOADED) -->
    {#if showGestaoErp && GestaoErpComponent}
        <svelte:component
            this={GestaoErpComponent}
            {users} 
            {transactions} 
            onUpdateUser={handleUpdateUserFromGestao} 
            onCloseErp={() => showGestaoErp = false} 
            onShowToast={showToast} 
        />
    {/if}

    <!-- NOTIFICAÇÕES TOAST -->
    <ToastContainer 
        {toasts} 
        onRemoveToast={removeToast} 
    />
</div>
