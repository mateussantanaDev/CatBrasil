<script>
    import { onMount } from 'svelte';

    // Banco de dados local em localStorage para simular o backend offline
    const LOCAL_USERS_KEY = 'cb_users';
    const LOCAL_MACHINES_KEY = 'cb_machines';
    const LOCAL_REFERRALS_KEY = 'cb_referrals';
    const LOCAL_TRANSACTIONS_KEY = 'cb_transactions';

    // Token e Autenticação
    let token = localStorage.getItem('token') || '';
    let authMode = 'login'; // 'login' ou 'register'
    let loginUsername = '';
    let loginPassword = '';
    
    // Estado de input do Registro
    let regName = '';
    let regPhone = '';
    let regPassword = '';
    let regConfirmPassword = '';
    let inviteCode = '';
    let verificationCodeInput = '';
    let verificationCodeSent = false;
    let generatedVerificationCode = '';
    let isSendingCode = false;

    // Estado Geral da Aplicação
    let user = {
        id: "",
        username: "",
        phone: "",
        balance: 0.00,
        xp: 0,
        nivel: 1,
        securityPassword: "",
        promoUnlocked: false,
        freeWithdrawActive: false,
        coletasCompleted: 0,
        tratorRented: false,
        alreadySpunToday: false,
        referralBonusSuspended: false
    };
    let garage = [];
    let referrals = [];
    let transactions = [];
    let txFilter = 'todos';
    let configSubtab = 'perfil'; // perfil, seguranca, rede, historico

    
    function getFilteredTransactions(txs, filter) {
        if (filter === 'depositos') {
            return txs.filter(t => t.tipo.toLowerCase().includes('depósito') || t.tipo.toLowerCase().includes('deposito'));
        }
        if (filter === 'saques') {
            return txs.filter(t => t.tipo.toLowerCase().includes('saque'));
        }
        if (filter === 'rendimentos') {
            return txs.filter(t => t.tipo.toLowerCase().includes('rendimento') || t.tipo.toLowerCase().includes('aluguel'));
        }
        return txs;
    }
    
    // Controle de Abas
    let currentTab = 'garagem';

    // Modais Adicionais
    let showPixPayModal = false;
    let currentPixValue = 0;
    let pixCodeString = "";
    let pixTimerSeconds = 900;
    let pixTimerInterval = null;


    // Toast Notifications & Efeitos Visuais
    let toasts = [];
    function showToast(message, type = '') {
        const id = Date.now() + Math.random();
        toasts = [...toasts, { id, message, type }];
        setTimeout(() => {
            removeToast(id);
        }, 3500);
    }
    function removeToast(id) {
        toasts = toasts.filter(t => t.id !== id);
    }

    function launchConfetti() {
        if (typeof document === 'undefined') return;
        const colors = ['#FFD700', '#FF4500', '#00E676', '#29B6F6', '#FF007F'];
        for (let i = 0; i < 35; i++) {
            const particle = document.createElement('div');
            particle.className = 'confetti-particle';
            particle.style.left = `${Math.random() * 100}vw`;
            particle.style.backgroundColor = colors[Math.floor(Math.random() * colors.length)];
            particle.style.animationDuration = `${1.2 + Math.random() * 1.5}s`;
            particle.style.transform = `rotate(${Math.random() * 360}deg)`;
            document.body.appendChild(particle);
            setTimeout(() => particle.remove(), 2800);
        }
    }

    // Controle de Modais
    let showDepositModal = false;
    let showWithdrawModal = false;
    let showInviteModal = false;
    let showAnnouncementsModal = false;
    let showLevelUpModal = false;
    let showSmsModal = false;
    let showEventsModal = false;

    // Carrossel e Roleta (Slide de Anúncios)
    let announceSlideIdx = 0;
    let allowAnnounceClose = false;
    let qaRouletteMode = 'normal';
    let rouletteSpinning = false;
    let rouletteTargetDegrees = 0;
    let spinBtnText = "🎡 GIRAR GRÁTIS";

    // Timer de Coleta
    let countdownDisplay = "10:00";
    let timerInterval = null;

    // Sanfonas (Accordions) da aba Ajustes
    let submenuOpen = {
        niveis: true,
        conta: false,
        bonificacoes: false,
        convidados: false,
        eventos: false,
        historico: false
    };

    function toggleSubmenu(key) {
        submenuOpen[key] = !submenuOpen[key];
    }

    // Seeding de dados padrão (Mateus) para o banco do LocalStorage
    function initLocalStorageDb() {
        if (!localStorage.getItem(LOCAL_USERS_KEY)) {
            const defaultUsers = [
                {
                    id: "CB-458F",
                    username: "mateus",
                    phone: "11999999999",
                    password: "admin", 
                    balance: 2450.00,
                    xp: 240,
                    nivel: 8,
                    securityPassword: "",
                    promoUnlocked: false,
                    freeWithdrawActive: false,
                    coletasCompleted: 4,
                    tratorRented: false,
                    alreadySpunToday: false,
                    referralBonusSuspended: false
                }
            ];
            localStorage.setItem(LOCAL_USERS_KEY, JSON.stringify(defaultUsers));
        }

        if (!localStorage.getItem(LOCAL_MACHINES_KEY)) {
            const defaultMachines = [
                {
                    id: "mach-1",
                    userId: "CB-458F",
                    name: "Escavadeira CAT 320",
                    cost: 2500,
                    yieldAmount: 110.00,
                    lastCollectedAt: new Date(Date.now() - 23.9 * 3600 * 1000).toISOString(),
                    state: "aguardando",
                    secondsRemaining: 360
                }
            ];
            localStorage.setItem(LOCAL_MACHINES_KEY, JSON.stringify(defaultMachines));
        }

        if (!localStorage.getItem(LOCAL_REFERRALS_KEY)) {
            const defaultReferrals = [
                { id: "ref-1", referrerId: "CB-458F", nome: "João Silva", status: "ativo", deposito: 500.00, data: "18/07/2026" },
                { id: "ref-2", referrerId: "CB-458F", nome: "Maria Oliveira", status: "ativo", deposito: 350.00, data: "19/07/2026" },
                { id: "ref-3", referrerId: "CB-458F", nome: "Carlos Pereira", status: "ativo", deposito: 1200.00, data: "20/07/2026" }
            ];
            localStorage.setItem(LOCAL_REFERRALS_KEY, JSON.stringify(defaultReferrals));
        }

        if (!localStorage.getItem(LOCAL_TRANSACTIONS_KEY)) {
            const defaultTransactions = [
                { id: "tx-1", userId: "CB-458F", data: "20/07/2026", tipo: "Depósito", valor: 300.00, status: "sucesso" },
                { id: "tx-2", userId: "CB-458F", data: "18/07/2026", tipo: "Saque", valor: -150.00, status: "sucesso" },
                { id: "tx-3", userId: "CB-458F", data: "15/07/2026", tipo: "Depósito", valor: 1000.00, status: "sucesso" }
            ];
            localStorage.setItem(LOCAL_TRANSACTIONS_KEY, JSON.stringify(defaultTransactions));
        }
    }

    // Getters e Setters Auxiliares do LocalStorage
    function getLocalUsers() { return JSON.parse(localStorage.getItem(LOCAL_USERS_KEY) || '[]'); }
    function setLocalUsers(users) { localStorage.setItem(LOCAL_USERS_KEY, JSON.stringify(users)); }

    function getLocalMachines() { return JSON.parse(localStorage.getItem(LOCAL_MACHINES_KEY) || '[]'); }
    function setLocalMachines(machs) { localStorage.setItem(LOCAL_MACHINES_KEY, JSON.stringify(machs)); }

    function getLocalReferrals() { return JSON.parse(localStorage.getItem(LOCAL_REFERRALS_KEY) || '[]'); }
    function setLocalReferrals(refs) { localStorage.setItem(LOCAL_REFERRALS_KEY, JSON.stringify(refs)); }

    function getLocalTransactions() { return JSON.parse(localStorage.getItem(LOCAL_TRANSACTIONS_KEY) || '[]'); }
    function setLocalTransactions(txs) { localStorage.setItem(LOCAL_TRANSACTIONS_KEY, JSON.stringify(txs)); }

    function updateUserInDb(id, updates) {
        const users = getLocalUsers();
        const idx = users.findIndex(u => u.id === id);
        if (idx !== -1) {
            users[idx] = { ...users[idx], ...updates };
            setLocalUsers(users);
        }
    }

    // Simulador de Métodos Reativos do App Local
    function fetchUserData() {
        const users = getLocalUsers();
        const found = users.find(u => u.id === token);
        if (found) {
            user = { ...found };
            allowAnnounceClose = user.alreadySpunToday;
        }
    }

    function fetchGarage() {
        const allMachs = getLocalMachines();
        const userMachs = allMachs.filter(m => m.userId === token);
        const now = Date.now();
        const cycle = 24 * 3600 * 1000;
        const grace = 10 * 60 * 1000;

        garage = userMachs.map(m => {
            if (m.overrideState) {
                return { ...m, state: m.overrideState, secondsRemaining: m.overrideSecondsRemaining };
            }
            
            const lastCollect = new Date(m.lastCollectedAt).getTime();
            const nextRelease = lastCollect + cycle;
            const limitTime = nextRelease + grace;
            
            let state = "aguardando";
            let secondsRemaining = 0;

            if (now < nextRelease) {
                state = "aguardando";
                secondsRemaining = Math.max(0, Math.floor((nextRelease - now) / 1000));
            } else if (now >= nextRelease && now <= limitTime) {
                state = "disponivel";
                secondsRemaining = Math.max(0, Math.floor((limitTime - now) / 1000));
            } else {
                state = "expirado";
                secondsRemaining = 0;
            }

            return {
                ...m,
                state,
                secondsRemaining
            };
        });

        if (garage.length > 0) {
            updateCountdown(garage[0]);
        }
    }

    function fetchReferrals() {
        const allRefs = getLocalReferrals();
        referrals = allRefs.filter(r => r.referrerId === token);
        
        // Verifica bônus de indicação suspenso
        const hasInactive = referrals.some(r => r.status === 'inativo');
        if (user.id && user.referralBonusSuspended !== hasInactive) {
            user.referralBonusSuspended = hasInactive;
            updateUserInDb(user.id, { referralBonusSuspended: hasInactive });
        }
    }

    function fetchTransactions() {
        const allTxs = getLocalTransactions();
        transactions = allTxs.filter(t => t.userId === token);
    }

    function refreshAll() {
        if (!token) return;
        fetchUserData();
        fetchGarage();
        fetchReferrals();
        fetchTransactions();
    }

    // Ações de Autenticação com Validação de Celular Único
    function isValidPhone(phone) {
        const cleaned = phone.replace(/\D/g, '');
        return cleaned.length === 11 && cleaned.startsWith('9', 2);
    }

    function isPasswordSecure(password) {
        if (password.length < 6) return { valid: false, error: "A senha deve conter no mínimo 6 caracteres!" };
        
        const allowedRegex = /^[a-zA-Z0-9@$!%*?&_#-]+$/;
        if (!allowedRegex.test(password)) {
            return { valid: false, error: "A senha contém caracteres inválidos! Use apenas letras, números e @$!%*?&_#-." };
        }
        
        const hasUppercase = /[A-Z]/.test(password);
        const hasLowercase = /[a-z]/.test(password);
        const hasNumber = /[0-9]/.test(password);
        const hasSpecial = /[@$!%*?&_#-]/.test(password);
        
        if (!hasUppercase || !hasLowercase || !hasNumber || !hasSpecial) {
            return {
                valid: false,
                error: "Senha fraca! Requer: 1 letra maiúscula, 1 minúscula, 1 número e 1 símbolo (@$!%*?&_#-)."
            };
        }
        return { valid: true };
    }

    function handleSendVerificationCode() {
        if (!regName) {
            showToast("❌ Digite o nome do operador!", "error");
            return;
        }
        if (!regPhone) {
            showToast("❌ Digite seu número de telefone!", "error");
            return;
        }
        
        const cleanedPhone = regPhone.replace(/\D/g, '');
        if (!isValidPhone(cleanedPhone)) {
            showToast("❌ Número de celular com DDD inválido! Ex: 11999999999", "error");
            return;
        }

        // Validação da senha antes de abrir o modal SMS
        const pwCheck = isPasswordSecure(regPassword);
        if (!pwCheck.valid) {
            showToast(`❌ ${pwCheck.error}`, "error");
            return;
        }

        if (regPassword !== regConfirmPassword) {
            showToast("❌ As senhas não conferem!", "error");
            return;
        }

        // Garante que o telefone está vinculado a apenas uma conta
        const users = getLocalUsers();
        const phoneExists = users.some(u => u.phone === cleanedPhone);
        if (phoneExists) {
            showToast("❌ Este telefone já está cadastrado em outra conta!", "error");
            return;
        }

        isSendingCode = true;
        showToast("⏳ Enviando SMS com código de verificação...", "");

        setTimeout(() => {
            generatedVerificationCode = String(Math.floor(100000 + Math.random() * 900000));
            verificationCodeSent = true;
            isSendingCode = false;
            showSmsModal = true;
            showToast(`✨ Código enviado! Verifique seu celular.`, "win");
        }, 1000);
    }

    function handleLogin() {
        if (!loginUsername || !loginPassword) {
            showToast("❌ Preencha usuário/telefone e senha!", "error");
            return;
        }
        const users = getLocalUsers();
        let found = users.find(u => (u.username.toLowerCase() === loginUsername.toLowerCase() || u.phone === loginUsername) && u.password === loginPassword);
        
        // Login livre sem restrições de autenticação (cria operador se não existir)
        if (!found) {
            const newId = `CB-${Math.floor(1000 + Math.random() * 9000)}`;
            const isNumeric = /^\d+$/.test(loginUsername);
            found = {
                id: newId,
                username: isNumeric ? `Op. ${loginUsername.substring(loginUsername.length - 4)}` : loginUsername,
                phone: isNumeric ? loginUsername : "11999999999",
                password: loginPassword,
                balance: 2450.00, // Saldo de testes inicial
                xp: 240,
                nivel: 8,
                securityPassword: "",
                promoUnlocked: false,
                freeWithdrawActive: false,
                coletasCompleted: 4,
                tratorRented: false,
                alreadySpunToday: false,
                referralBonusSuspended: false
            };
            users.push(found);
            setLocalUsers(users);

            // Garante uma escavadeira para o novo operador
            const machs = getLocalMachines();
            machs.push({
                id: `mach-${Date.now()}`,
                userId: newId,
                name: "Escavadeira CAT 320",
                cost: 2500,
                yieldAmount: 110.00,
                lastCollectedAt: new Date(Date.now() - 23.9 * 3600 * 1000).toISOString(),
                state: "aguardando",
                secondsRemaining: 360
            });
            setLocalMachines(machs);
        }

        token = found.id;
        localStorage.setItem('token', token);
        showToast(`⚡ Bem-vindo, ${found.username}!`, "win");
        loginUsername = "";
        loginPassword = "";
        refreshAll();
        
        if (!user.alreadySpunToday) {
            showAnnouncementsModal = true;
            announceSlideIdx = 0;
            allowAnnounceClose = false;
        }
    }

    function handleRegister() {
        if (!regName) {
            showToast("❌ Informe o nome do operador!", "error");
            return;
        }

        if (!regPhone || !regPassword || !regConfirmPassword) {
            showToast("❌ Preencha todos os campos obrigatórios!", "error");
            return;
        }

        const cleanedPhone = regPhone.replace(/\D/g, '');
        if (!verificationCodeSent) {
            showToast("❌ Solicite o código de verificação primeiro!", "error");
            return;
        }

        if (verificationCodeInput !== generatedVerificationCode) {
            showToast("❌ Código de verificação incorreto!", "error");
            return;
        }

        if (regPassword.length < 5) {
            showToast("❌ A senha deve conter pelo menos 5 caracteres!", "error");
            return;
        }

        if (regPassword !== regConfirmPassword) {
            showToast("❌ As senhas não conferem!", "error");
            return;
        }

        // Cria o novo operador
        const users = getLocalUsers();
        const newId = `CB-${Math.floor(1000 + Math.random() * 9000)}`;
        
        const newUser = {
            id: newId,
            username: regName,
            phone: cleanedPhone,
            password: regPassword,
            balance: 0.00,
            xp: 0,
            nivel: 1,
            securityPassword: "",
            promoUnlocked: false,
            freeWithdrawActive: false,
            coletasCompleted: 0,
            tratorRented: false,
            alreadySpunToday: false,
            referralBonusSuspended: false
        };

        // Escavadeira gratuita de boas-vindas
        const welcomeMachine = {
            id: `mach-${Date.now()}`,
            userId: newId,
            name: "Escavadeira CAT 320",
            cost: 2500,
            yieldAmount: 110.00,
            lastCollectedAt: new Date(Date.now() - 24 * 3600 * 1000).toISOString(),
            state: "disponivel",
            secondsRemaining: 600
        };

        // Associa código de convite
        if (inviteCode) {
            const referrerId = inviteCode.toUpperCase();
            const referrer = users.find(u => u.id === referrerId);
            if (referrer) {
                const refs = getLocalReferrals();
                refs.push({
                    id: `ref-${Date.now()}`,
                    referrerId: referrerId,
                    nome: `Op. ${cleanedPhone.substring(cleanedPhone.length - 4)}`,
                    status: "inativo",
                    deposito: 0.00,
                    data: new Date().toLocaleDateString('pt-BR')
                });
                setLocalReferrals(refs);
            }
        }

        users.push(newUser);
        setLocalUsers(users);

        const machs = getLocalMachines();
        machs.push(welcomeMachine);
        setLocalMachines(machs);

        // Login automático
        token = newId;
        localStorage.setItem('token', token);
        showToast("📝 Cadastro concluído com sucesso!", "win");

        // Limpa campos
        regName = "";
        regPhone = "";
        regPassword = "";
        regConfirmPassword = "";
        verificationCodeInput = "";
        verificationCodeSent = false;
        generatedVerificationCode = "";
        inviteCode = "";
        showSmsModal = false; // Fecha modal SMS!

        refreshAll();
        showAnnouncementsModal = true;
        announceSlideIdx = 0;
        allowAnnounceClose = false;
    }

    function handleLogout() {
        token = '';
        localStorage.removeItem('token');
        user = { id: "", username: "", balance: 0, xp: 0, nivel: 1 };
        garage = [];
        referrals = [];
        transactions = [];
        showAnnouncementsModal = false;
        showDepositModal = false;
        showWithdrawModal = false;
        showInviteModal = false;
        showLevelUpModal = false;
        showSmsModal = false;
        showEventsModal = false;
    }

    onMount(() => {
        initLocalStorageDb();
        
        // Verifica se há convite na URL (?ref=CB-XXXX)
        const urlParams = new URLSearchParams(window.location.search);
        const refParam = urlParams.get('ref');
        if (refParam) {
            inviteCode = refParam.toUpperCase();
            authMode = 'register';
            showToast(`👥 Indicado pelo link: ${inviteCode}`, "win");
        }

        if (token) {
            refreshAll();
        }

        // Loop de Atualização do Timer
        timerInterval = setInterval(() => {
            if (token && garage.length > 0) {
                const m = garage[0];
                if (m.secondsRemaining > 0 && m.state !== 'coletado' && m.state !== 'expirado') {
                    m.secondsRemaining--;
                    updateCountdown(m);
                } else if (m.secondsRemaining === 0 && m.state === 'disponivel') {
                    fetchGarage();
                }
            }
        }, 1000);

        return () => {
            if (timerInterval) clearInterval(timerInterval);
        };
    });

    function updateCountdown(m) {
        if (m.state === 'expirado') {
            countdownDisplay = "Expirado";
        } else {
            const hours = Math.floor(m.secondsRemaining / 3600);
            const mins = Math.floor((m.secondsRemaining % 3600) / 60);
            const secs = m.secondsRemaining % 60;
            
            if (hours >= 1) {
                if (mins === 0) {
                    countdownDisplay = `${hours}h`;
                } else {
                    countdownDisplay = `${hours}h ${String(mins).padStart(2, '0')}m`;
                }
            } else {
                countdownDisplay = `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
            }
        }
    }

    // Coletar produção
    function collectReward(machineId) {
        const machs = getLocalMachines();
        const idx = machs.findIndex(m => m.id === machineId);
        if (idx === -1) return;
        
        const machine = machs[idx];
        user.balance += machine.yieldAmount;
        user.coletasCompleted = Math.min(5, user.coletasCompleted + 1);
        updateUserInDb(user.id, { balance: user.balance, coletasCompleted: user.coletasCompleted });

        machs[idx].lastCollectedAt = new Date().toISOString();
        machs[idx].overrideState = null;
        machs[idx].overrideSecondsRemaining = null;
        setLocalMachines(machs);

        const txs = getLocalTransactions();
        txs.unshift({
            id: `tx-${Date.now()}`,
            userId: user.id,
            data: new Date().toLocaleDateString('pt-BR'),
            tipo: "Rendimento",
            valor: machine.yieldAmount,
            status: "sucesso"
        });
        setLocalTransactions(txs);

        // Se houver indicação correspondente desse usuário, ativa ela
        const refs = getLocalReferrals();
        const refIdx = refs.findIndex(r => r.nome === `Op. ${user.phone.substring(user.phone.length - 4)}`);
        if (refIdx !== -1 && refs[refIdx].status !== 'ativo') {
            refs[refIdx].status = 'ativo';
            refs[refIdx].deposito = 350.00;
            setLocalReferrals(refs);
        }

        showToast("💰 Coleta de rendimentos efetuada com sucesso!", "win");
        refreshAll();
    }

    // Simular status da garagem
    function simulateGarageState(state) {
        const machs = getLocalMachines();
        const userMachs = machs.filter(m => m.userId === token);
        if (userMachs.length === 0) return;
        
        const machineId = userMachs[0].id;
        const idx = machs.findIndex(m => m.id === machineId);
        
        if (state === 'aguardando') {
            machs[idx].overrideState = 'aguardando';
            machs[idx].overrideSecondsRemaining = 15;
            machs[idx].lastCollectedAt = new Date(Date.now() - 23.996 * 3600 * 1000).toISOString();
        } else if (state === 'disponivel') {
            machs[idx].overrideState = 'disponivel';
            machs[idx].overrideSecondsRemaining = 600;
            machs[idx].lastCollectedAt = new Date(Date.now() - 24.0 * 3600 * 1000).toISOString();
            user.coletasCompleted = 4;
            updateUserInDb(user.id, { coletasCompleted: 4 });
        } else if (state === 'expirado') {
            machs[idx].overrideState = 'expirado';
            machs[idx].overrideSecondsRemaining = 0;
            machs[idx].lastCollectedAt = new Date(Date.now() - 24.5 * 3600 * 1000).toISOString();
        } else {
            machs[idx].overrideState = null;
            machs[idx].overrideSecondsRemaining = null;
            machs[idx].lastCollectedAt = new Date().toISOString();
        }
        
        setLocalMachines(machs);
        showToast(`⚙️ Simulação ativada: ${state}`, "win");
        refreshAll();
    }

    // Alugar máquina na loja (Dicionário de Preços simulado do Servidor)
    const SHOP_MACHINE_PRICES = {
        'CAT 301.5 (Promo)': { cost: 300, yield: 22.00 },
        'Trator D8T': { cost: 10000, yield: 440.00 }
    };

    function rentMachine(name) {
        const item = SHOP_MACHINE_PRICES[name];
        if (!item) {
            showToast("❌ Produto de aluguel inválido!", "error");
            return;
        }

        const cost = item.cost;
        if (user.balance < cost) {
            showToast("❌ Saldo insuficiente na CatBrasil", "error");
            return;
        }
        
        user.balance -= cost;
        const xpGained = Math.floor(cost * 0.02);
        user.xp = Math.min(1000, user.xp + xpGained);
        if (name === 'Trator D8T') {
            user.tratorRented = true;
        }
        
        updateUserInDb(user.id, {
            balance: user.balance,
            xp: user.xp,
            tratorRented: user.tratorRented
        });

        const machs = getLocalMachines();
        machs.push({
            id: `mach-${Date.now()}`,
            userId: user.id,
            name: name,
            cost: cost,
            yieldAmount: item.yield,
            lastCollectedAt: new Date().toISOString(),
            state: "aguardando",
            secondsRemaining: 24 * 3600
        });
        setLocalMachines(machs);

        const txs = getLocalTransactions();
        txs.unshift({
            id: `tx-${Date.now()}`,
            userId: user.id,
            data: new Date().toLocaleDateString('pt-BR'),
            tipo: `Aluguel ${name}`,
            valor: -cost,
            status: "sucesso"
        });
        setLocalTransactions(txs);

        showToast(`🔨 Contrato de ${name} ativado!`, "win");
        refreshAll();
    }

    // Simular ativação de indicação (Maria Oliveira)
    function simulateReferral(active) {
        const refs = getLocalReferrals();
        const userRefs = refs.filter(r => r.referrerId === token);
        
        if (userRefs.length > 0) {
            const targetId = userRefs[0].id;
            const idx = refs.findIndex(r => r.id === targetId);
            refs[idx].status = active ? "ativo" : "inativo";
            refs[idx].deposito = active ? 350.00 : 0.00;
            setLocalReferrals(refs);
        } else {
            refs.push({
                id: `ref-${Date.now()}`,
                referrerId: user.id,
                nome: "Maria Oliveira (Simulado)",
                status: active ? "ativo" : "inativo",
                deposito: active ? 350.00 : 0.00,
                data: new Date().toLocaleDateString('pt-BR')
            });
            setLocalReferrals(refs);
        }
        showToast(`👥 Simulação de indicação atualizada!`, "win");
        refreshAll();
    }

    // Salvar senha de saque
    let inputPassword = "";
    function saveSecurityPassword() {
        if (inputPassword.length !== 6) {
            showToast("❌ A senha deve conter exatamente 6 dígitos!", "error");
            return;
        }
        user.securityPassword = inputPassword;
        updateUserInDb(user.id, { securityPassword: inputPassword });
        showToast("🔒 Senha de retirada salva com sucesso!", "win");
        inputPassword = "";
        refreshAll();
    }

    // Subir nível
    function levelUp() {
        const activeCount = referrals.filter(r => r.status === 'ativo').length;
        if (user.coletasCompleted < 5 || !user.tratorRented || activeCount < 3) {
            showToast("❌ Requisitos de promoção pendentes!", "error");
            return;
        }
        
        user.nivel += 1;
        user.coletasCompleted = 0;
        user.tratorRented = false;
        updateUserInDb(user.id, {
            nivel: user.nivel,
            coletasCompleted: 0,
            tratorRented: false
        });

        const txs = getLocalTransactions();
        txs.unshift({
            id: `tx-${Date.now()}`,
            userId: user.id,
            data: new Date().toLocaleDateString('pt-BR'),
            tipo: "Promoção Nível",
            valor: 0.00,
            status: "sucesso"
        });
        setLocalTransactions(txs);

        showToast("⭐ Parabéns! Você subiu de nível!", "win");
        showLevelUpModal = true;
        refreshAll();
    }

    // Saque
    let withdrawAmount = "";
    let withdrawPassword = "";
    function processWithdraw() {
        const val = parseFloat(withdrawAmount);
        if (isNaN(val) || val < 50) {
            showToast("❌ Valor mínimo para saque é R$ 50,00!", "error");
            return;
        }
        if (val > user.balance) {
            showToast("❌ Saldo insuficiente na carteira!", "error");
            return;
        }
        if (!user.securityPassword) {
            showToast("❌ Cadastre sua senha de saque nos Ajustes!", "error");
            return;
        }
        if (withdrawPassword !== user.securityPassword) {
            showToast("❌ Senha de segurança incorreta!", "error");
            return;
        }
        
        const feePercent = user.freeWithdrawActive ? 0.00 : 0.05;
        const fee = val * feePercent;
        const net = val - fee;

        user.balance -= val;
        user.freeWithdrawActive = false;
        updateUserInDb(user.id, { balance: user.balance, freeWithdrawActive: false });

        const txs = getLocalTransactions();
        txs.unshift({
            id: `tx-${Date.now()}`,
            userId: user.id,
            data: new Date().toLocaleDateString('pt-BR'),
            tipo: "Saque",
            valor: -val,
            status: "pendente"
        });
        setLocalTransactions(txs);

        showToast(`💰 Saque de R$ ${val.toFixed(2)} solicitado com sucesso!`, "win");
        withdrawAmount = "";
        withdrawPassword = "";
        showWithdrawModal = false;
        refreshAll();
    }

    // Depósito PIX com QR Code
    let depositAmount = "";
    function processDeposit() {
        const val = parseFloat(depositAmount);
        if (isNaN(val) || val < 100) {
            showToast("❌ Mínimo para depósito é R$ 100,00!", "error");
            return;
        }
        
        currentPixValue = val;
        pixCodeString = `00020126580014BR.GOV.BCB.PIX0136catbrasil-deposit-${user.id || 'CB-458F'}-${Date.now()}5204000053039865405${val.toFixed(2)}5802BR5909CATBRASIL6009SAO_PAULO62070503***6304`;
        pixTimerSeconds = 900;
        showDepositModal = false;
        showPixPayModal = true;

        if (pixTimerInterval) clearInterval(pixTimerInterval);
        pixTimerInterval = setInterval(() => {
            if (pixTimerSeconds > 0) {
                pixTimerSeconds--;
            } else {
                clearInterval(pixTimerInterval);
            }
        }, 1000);
    }

    function copyPixCode() {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(pixCodeString);
        }
        showToast("📋 Código PIX Copia e Cola copiado com sucesso!", "win");
    }

    function confirmPixPayment() {
        const val = currentPixValue;
        user.balance += val;
        updateUserInDb(user.id, { balance: user.balance });

        const txs = getLocalTransactions();
        txs.unshift({
            id: `tx-${Date.now()}`,
            userId: user.id,
            data: new Date().toLocaleDateString('pt-BR'),
            tipo: "Depósito PIX",
            valor: val,
            status: "sucesso"
        });
        setLocalTransactions(txs);

        launchConfetti();
        showToast(`⚡ Depósito de R$ ${val.toFixed(2)} aprovado via PIX!`, "win");
        if (pixTimerInterval) clearInterval(pixTimerInterval);
        showPixPayModal = false;
        depositAmount = "";
        refreshAll();
    }

    // Giro da roleta
    function spinRoulette() {
        if (user.alreadySpunToday || rouletteSpinning) {
            showToast("❌ Você já realizou seu giro diário hoje!", "error");
            return;
        }
        rouletteSpinning = true;
        spinBtnText = "⚙️ GIRANDO...";

        let sectorIdx = 0;
        if (qaRouletteMode === 'promo') {
            sectorIdx = 3;
        } else if (qaRouletteMode === 'free') {
            sectorIdx = 4;
        } else if (qaRouletteMode === 'empty') {
            sectorIdx = 0;
        } else {
            const roll = Math.random() * 100;
            if (roll < 2.5) {
                sectorIdx = 3; 
            } else if (roll < 5.0) {
                sectorIdx = 4;
            } else if (roll < 45.0) {
                sectorIdx = 1;
            } else if (roll < 80.0) {
                sectorIdx = 2;
            } else {
                sectorIdx = Math.random() < 0.5 ? 0 : 5;
            }
        }

        const prizes = [
            { type: 'NONE', amount: 0, label: 'Tente amanhã 💀' },
            { type: 'XP', amount: 10, label: '⚡ 10 XP' },
            { type: 'XP', amount: 20, label: '⚡ 20 XP' },
            { type: 'PROMO', amount: 300, label: '🎁 Contrato Promo R$ 300 (CAT 301.5)' },
            { type: 'FREE_WITHDRAW', amount: 0, label: '🎫 Saque Grátis (Taxa Zero!)' },
            { type: 'NONE', amount: 0, label: 'Tente amanhã 💀' }
        ];

        const win = prizes[sectorIdx];
        user.alreadySpunToday = true;

        if (win.type === 'XP') {
            user.xp = Math.min(1000, user.xp + win.amount);
        } else if (win.type === 'PROMO') {
            user.promoUnlocked = true;
        } else if (win.type === 'FREE_WITHDRAW') {
            user.freeWithdrawActive = true;
        }

        const rotations = 8;
        rouletteTargetDegrees = (rotations * 360) - (30 + sectorIdx * 60);

        setTimeout(() => {
            spinBtnText = "✅ GIRO CONCLUÍDO";
            updateUserInDb(user.id, {
                alreadySpunToday: true,
                xp: user.xp,
                promoUnlocked: user.promoUnlocked,
                freeWithdrawActive: user.freeWithdrawActive
            });
            
            if (win.type === 'XP') {
                showToast(`🎉 Ganhou ${win.label}!`, "win");
            } else if (win.type === 'PROMO') {
                showToast(`🎉 Super Sorte! ${win.label}! Vá na Loja.`, "win");
            } else if (win.type === 'FREE_WITHDRAW') {
                showToast(`🎉 Super Sorte! ${win.label}! Taxa zero no saque.`, "win");
            } else {
                showToast("😢 Não foi dessa vez! Volte amanhã.", "");
            }

            setTimeout(() => {
                showAnnouncementsModal = false;
                refreshAll();
            }, 2500);
        }, 4000);
    }

    function resetRoulette() {
        user.alreadySpunToday = false;
        user.promoUnlocked = false;
        user.freeWithdrawActive = false;
        updateUserInDb(user.id, {
            alreadySpunToday: false,
            promoUnlocked: false,
            freeWithdrawActive: false
        });
        showToast("🎡 Roleta e bônus resetados para teste!", "win");
        rouletteSpinning = false;
        spinBtnText = "GIRAR GRÁTIS";
        rouletteTargetDegrees = 0;
        refreshAll();
    }

    function fullReset() {
        localStorage.removeItem(LOCAL_USERS_KEY);
        localStorage.removeItem(LOCAL_MACHINES_KEY);
        localStorage.removeItem(LOCAL_REFERRALS_KEY);
        localStorage.removeItem(LOCAL_TRANSACTIONS_KEY);
        initLocalStorageDb();
        showToast("🔄 Banco de dados local reiniciado!", "win");
        handleLogout();
    }

    // Scroll manual de carrossel de eventos
    let scrollContainer;
    function scrollToSlide(index) {
        if (scrollContainer) {
            const slides = scrollContainer.querySelectorAll('.evento-slide');
            if (slides[index]) {
                scrollContainer.scrollTo({
                    left: slides[index].offsetLeft - scrollContainer.offsetLeft,
                    behavior: 'smooth'
                });
            }
        }
    }

    function copyRefLink() {
        navigator.clipboard.writeText(`https://catbrasil.app/ref/${user.id}`);
        showToast("📋 Link de convite copiado!", "win");
    }

    function tryCloseAnnouncements() {
        if (allowAnnounceClose || user.alreadySpunToday) {
            showAnnouncementsModal = false;
        } else {
            showToast("🔒 Garanta seu Giro Grátis na Roleta (Slide 4) para liberar o fechamento!", "error");
        }
    }

    function nextAnnounceSlide() {
        if (announceSlideIdx < 3) {
            announceSlideIdx++;
            if (announceSlideIdx === 3) {
                allowAnnounceClose = true;
            }
        }
    }

    function prevAnnounceSlide() {
        if (announceSlideIdx > 0) {
            announceSlideIdx--;
        }
    }
</script>

<div class="app-container">
    {#if !token}
        <!-- TELA DE LOGIN / REGISTRO REAL (ESTILO BRUTALISTA CLARO) -->
        <div class="login-screen-wrapper">
            <div class="login-card-container">
                <div class="login-banner-stripes"></div>
                
                <div style="width: 52px; height: 52px; border-radius: 50%; background: var(--cat-yellow); border: 2.5px solid var(--cat-black); display: flex; align-items: center; justify-content: center; font-size: 1.6rem; box-shadow: 2px 2px 0px #000; margin: 0 auto 12px auto; animation: bounce 2s infinite;">🚜</div>
                
                <h1 style="font-family:'Oswald'; font-size:2.2rem; color:var(--cat-black); margin-bottom:2px; letter-spacing:1px; line-height: 1.1;">CatBrasil</h1>
                <p style="font-size:0.62rem; text-transform:uppercase; font-weight:800; color:#666; letter-spacing:1.5px; margin-bottom:24px;">Central Operacional de Mineração</p>

                {#if authMode === 'login'}
                    <!-- FORMULÁRIO DE LOGIN -->
                    <div class="login-input-group">
                        <label for="username">Usuário ou Telefone:</label>
                        <input type="text" placeholder="Digite seu usuário ou telefone" bind:value={loginUsername} id="username">
                    </div>

                    <div class="login-input-group">
                        <label for="password">Senha de Acesso:</label>
                        <input type="password" placeholder="Digite sua senha" bind:value={loginPassword} id="password">
                    </div>

                    <button class="login-btn-submit" on:click={handleLogin}>⚡ ENTRAR NO TURNO</button>
                    <p style="font-size:0.75rem; color:#555; margin-top:14px;">
                        Novo no pátio? 
                        <button class="login-toggle-link" on:click={() => authMode = 'register'}>Criar nova conta</button>
                    </p>
                {:else}
                    <!-- FORMULÁRIO DE CADASTRO COM VALIDAÇÃO CELULAR SMS -->
                    <div class="login-input-group">
                        <label for="regName">Nome do Operador:</label>
                        <input type="text" placeholder="Ex: Mateus Silva" bind:value={regName} id="regName">
                    </div>

                    <div class="login-input-group">
                        <label for="regPhone">Número de Celular (com DDD):</label>
                        <input type="text" placeholder="Ex: 11999999999" bind:value={regPhone} id="regPhone">
                    </div>

                    <div class="login-input-group">
                        <label for="regPassword">Senha do Operador:</label>
                        <input type="password" placeholder="Mínimo 6 caracteres" bind:value={regPassword} id="regPassword">
                    </div>

                    <div class="login-input-group">
                        <label for="regConfirmPassword">Confirmar Senha:</label>
                        <input type="password" placeholder="Repita a senha" bind:value={regConfirmPassword} id="regConfirmPassword">
                    </div>

                    <div class="login-input-group">
                        <label for="invite">Código de Convite (Opcional):</label>
                        <input type="text" placeholder="Ex: CB-458F" bind:value={inviteCode} id="invite" style="text-transform:uppercase;">
                    </div>

                    <button class="login-btn-submit" on:click={handleSendVerificationCode} disabled={isSendingCode}>
                        {isSendingCode ? '⏳ ENVIANDO...' : '📨 CADASTRAR OPERADOR'}
                    </button>

                    <p style="font-size:0.75rem; color:#555; margin-top:14px;">
                        Já possui conta? 
                        <button class="login-toggle-link" on:click={() => { authMode = 'login'; verificationCodeSent = false; }}>Fazer login</button>
                    </p>
                {/if}
                
                <p style="font-size: 0.58rem; color: #777; margin-top: 24px;">CatBrasil v2.0 · Conexão Criptografada SSL 🔐</p>
            </div>
        </div>
    {:else}
        <!-- INTERFACE LOGADA DO APLICATIVO -->
        <header>
            <div class="header-top">
                <div class="user-level mono-text">NÍVEL {user.nivel} · OPERADOR</div>
                <div class="balance">
                    <span>Saldo Reais</span>
                    <strong class="mono-text">R$ {user.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong>
                </div>
            </div>
            <div class="xp-container">
                <div class="xp-bar-bg"><div class="xp-bar-fill" style="width: {Math.min(100, (user.xp / 1000) * 100)}%;"></div></div>
                <div class="xp-info">
                    <span>{user.xp} / 1000 XP</span>
                    <span style="font-weight:700;color:var(--cat-yellow);">PROGRESSÃO DE CARREIRA</span>
                </div>
            </div>
            <div class="quick-actions">
                <button class="btn-action-small" on:click={() => showDepositModal = true}>💵 Depositar</button>
                <button class="btn-action-small" on:click={() => showWithdrawModal = true}>💰 Sacar</button>
            </div>
        </header>

        <!-- CONTEÚDO PRINCIPAL -->
        <main class="content">
            
            <!-- ABA: GARAGEM -->
            {#if currentTab === 'garagem'}
                <div class="tab-content active">
                    <h2 class="section-title">🚜 Minha Frota <span class="dot"></span></h2>
                    
                    {#each garage as machine}
                        <div class="alert-box" style="background: {machine.state === 'expirado' ? '#FFCDD2' : '#FFF3CD'}; border-color: {machine.state === 'expirado' ? 'var(--cat-red)' : '#E0C36A'};">
                            <span class="alert-icon">{machine.state === 'expirado' ? '❌' : '⚠️'}</span>
                            {#if machine.state === 'aguardando'}
                                <span>🔒 Produção bloqueada. Liberação em <strong>{countdownDisplay}</strong>.</span>
                            {:else if machine.state === 'disponivel'}
                                <span>⚠️ <strong>COLETA DISPONÍVEL!</strong> Expira em <strong class="highlight-red">{countdownDisplay}</strong>.</span>
                            {:else if machine.state === 'expirado'}
                                <span>❌ <strong>ATRASO > 10MIN!</strong> Você perdeu o prazo limite de coleta. Nova liberação amanhã.</span>
                            {/if}
                        </div>
                        
                        <div class="store-card" id="machineCard">
                            <div class="store-card-img" style="background-image: url('https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=400&q=80');"></div>
                            <div class="store-card-body">
                                <h3 class="store-card-title">{machine.name}</h3>
                                <div class="store-card-details">
                                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Produção hoje:</span><span class="mono-text highlight-green">+R$ {machine.yieldAmount.toFixed(2)}</span></div>
                                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;">
                                        <span>Status:</span>
                                        <span class="mono-text {machine.state === 'expirado' ? 'highlight-red' : 'highlight-yellow'}">
                                            {machine.state === 'aguardando' ? 'Aguardando Liberação' : machine.state === 'disponivel' ? 'Disponível' : 'Expirado'}
                                        </span>
                                    </div>
                                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;">
                                        <span>{machine.state === 'aguardando' ? 'Liberação em:' : 'Tolerância Limite:'}</span>
                                        <span class="mono-text">{countdownDisplay}</span>
                                    </div>
                                </div>
                                
                                <button 
                                    class="btn {machine.state === 'disponivel' ? 'btn-claim' : 'btn-primary'}" 
                                    disabled={machine.state !== 'disponivel'} 
                                    on:click={() => collectReward(machine.id)}
                                    style="padding: 6px; font-size: 0.75rem; margin-top: 4px; border-color: var(--cat-black);"
                                >
                                    {#if machine.state === 'aguardando'}
                                        🔒 Aguardando
                                    {:else if machine.state === 'disponivel'}
                                        💰 Coletar
                                    {:else}
                                        ❌ Expirado
                                    {/if}
                                </button>
                            </div>
                        </div>
                    {/each}


                </div>
            {/if}

            <!-- ABA: LOJA -->
            {#if currentTab === 'loja'}
                <div class="tab-content active">
                    <h2 class="section-title">🛒 Contratos de Aluguel</h2>


                    <!-- Card Promocional Desbloqueado via Roleta -->
                    {#if user.promoUnlocked}
                        <div class="store-card" style="border-color: var(--cat-green); background: #E8F5E9;">
                            <span class="badge-limit" style="background: var(--cat-green); color: white;">PROMO ROLETA</span>
                            <div class="store-card-img" style="background-image:url('https://images.unsplash.com/photo-1541625602330-2277a4c46182?w=400&q=80');"></div>
                            <div class="store-card-body">
                                <h3 class="store-card-title" style="color: var(--cat-green);">CAT 301.5 (Promo)</h3>
                                <div class="store-card-details">
                                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Custo:</span><strong class="mono-text">R$ 300,00</strong></div>
                                    <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Retorno (15d):</span><strong class="mono-text highlight-green">R$ 330,00</strong></div>
                                </div>
                                <button class="btn" on:click={() => rentMachine('CAT 301.5 (Promo)')} style="padding: 6px; font-size: 0.75rem; margin-top: 4px; background: var(--cat-green); color: white; border-color: var(--cat-black);">Alugar Promo</button>
                            </div>
                        </div>
                    {/if}

                    <!-- Card 1: Mini Carregadeira -->
                    <div class="store-card">
                        <span class="badge-limit">2/2</span>
                        <div class="store-card-img" style="background-image:url('https://images.unsplash.com/photo-1504307651254-35680f35aa9e?w=400&q=80'); filter:grayscale(1);"></div>
                        <div class="store-card-body">
                            <h3 class="store-card-title">Mini Carregadeira 236D3</h3>
                            <div class="store-card-details">
                                <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Custo:</span><span class="mono-text">R$ 1.000,00</span></div>
                                <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Retorno:</span><span class="mono-text highlight-green">R$ 1.100,00</span></div>
                            </div>
                            <button class="btn btn-primary" disabled style="padding: 6px; font-size: 0.75rem; margin-top: 4px;">Limite Atingido</button>
                        </div>
                    </div>

                    <!-- Card 2: Trator D8T -->
                    <div class="store-card">
                        <span class="badge-limit">{user.tratorRented ? '1/1' : '0/1'}</span>
                        <div class="store-card-img" style="background-image:url('https://images.unsplash.com/photo-1621932953936-056b1b4d0eb0?w=400&q=80');"></div>
                        <div class="store-card-body">
                            <h3 class="store-card-title">Trator D8T</h3>
                            <div class="store-card-details">
                                <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Custo:</span><span class="mono-text">R$ 10.000,00</span></div>
                                <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Retorno:</span><span class="mono-text highlight-green">R$ 11.000,00</span></div>
                            </div>
                            <button class="btn btn-primary" disabled={user.tratorRented} on:click={() => rentMachine('Trator D8T')} style="padding: 6px; font-size: 0.75rem; margin-top: 4px;">
                                {user.tratorRented ? '🔨 CONTRATO ATIVO' : '🔨 ALUGAR'}
                            </button>
                        </div>
                    </div>

                    <!-- Card 3: Caminhão 797F (Locked) -->
                    <div class="store-card" style="opacity:0.7;">
                        <div class="store-card-img" style="background-image:url('https://images.unsplash.com/photo-1581091226825-a6a2a5aee158?w=400&q=80'); filter:brightness(0.5);"></div>
                        <div class="store-card-body">
                            <h3 class="store-card-title">Caminhão 797F</h3>
                            <div class="store-card-details">
                                <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Custo:</span><span class="mono-text">R$ 50.000,00</span></div>
                                <div class="data-row" style="font-size:0.7rem; margin:0; padding:1px 0;"><span>Retorno:</span><span class="mono-text highlight-green">R$ 55.000,00</span></div>
                            </div>
                            <p style="color:var(--cat-red); font-size:0.65rem; font-weight:bold; margin-top: 4px; text-transform: uppercase;">🔒 Requer Nível 20</p>
                        </div>
                    </div>
                </div>
            {/if}



            <!-- ABA: AJUSTES / CONFIGURAÇÕES (SMART ACCORDION DASHBOARD - NO TRUNCATION) -->
            {#if currentTab === 'config'}
                <div class="tab-content active" style="padding-bottom: 50px; width: 100%; box-sizing: border-box;">
                    
                    <!-- Header do Perfil do Operador (High-Trust & Clean) -->
                    <div style="background: var(--cat-black); color: var(--cat-white); padding: 14px 16px; border: var(--brutal-border); border-radius: 4px; box-shadow: var(--brutal-shadow); margin-bottom: 12px; width: 100%; box-sizing: border-radius: 4px;">
                        <div style="display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap;">
                            <div style="display: flex; align-items: center; gap: 10px; min-width: 0;">
                                <div style="width: 44px; height: 44px; border-radius: 50%; background: var(--cat-yellow); border: 2px solid var(--cat-white); display: flex; align-items: center; justify-content: center; font-size: 1.4rem; flex-shrink: 0;">👷‍♂️</div>
                                <div style="min-width: 0;">
                                    <h3 style="font-size: 1.15rem; color: var(--cat-yellow); font-family:'Oswald'; margin: 0; line-height: 1.1; word-break: break-word;">{user.username.toUpperCase()}</h3>
                                    <span style="font-size: 0.58rem; color: #bbb; text-transform: uppercase; font-weight: 700;">OPERADOR · NÍVEL {user.nivel}</span>
                                </div>
                            </div>
                            <div style="display: flex; gap: 6px; align-items: center; flex-wrap: wrap;">
                                <span style="font-size: 0.58rem; background: #1b5e20; border: 1px solid #4caf50; color: #a5d6a7; padding: 2px 6px; font-weight: 800; border-radius: 2px; white-space: nowrap;">VERIFICADO ✅</span>
                                <button on:click={copyInviteLink} style="font-size: 0.58rem; background: var(--cat-gray); border: 1px solid var(--cat-yellow); color: var(--cat-yellow); padding: 2px 6px; font-weight: 800; border-radius: 2px; cursor: pointer; white-space: nowrap;">ID #{user.id} 📋</button>
                            </div>
                        </div>
                    </div>

                    <!-- Resumo Financeiro da Conta (2 Colunas Compactas) -->
                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px; width: 100%; box-sizing: border-box;">
                        <div style="background: #fff; border: 2px solid var(--cat-black); padding: 10px; border-radius: 4px; box-shadow: 2.5px 2.5px 0px var(--cat-black); min-width: 0;">
                            <span style="font-size: 0.58rem; color: #666; text-transform: uppercase; font-weight: 800; display: block;">💵 Saldo Livre</span>
                            <strong class="mono-text" style="font-size: 0.88rem; color: var(--cat-green); display: block; margin-top: 1px; word-break: break-word;">R$ {user.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong>
                        </div>
                        <div style="background: #fff; border: 2px solid var(--cat-black); padding: 10px; border-radius: 4px; box-shadow: 2.5px 2.5px 0px var(--cat-black); min-width: 0;">
                            <span style="font-size: 0.58rem; color: #666; text-transform: uppercase; font-weight: 800; display: block;">📈 Bônus Nível</span>
                            <strong class="mono-text highlight-yellow" style="font-size: 0.88rem; display: block; margin-top: 1px; word-break: break-word;">+{((user.nivel - 1) * 5).toFixed(1)}% Extra</strong>
                        </div>
                    </div>

                    <!-- SMART ACCORDION 1: SEGURANÇA & SENHA FINANCEIRA -->
                    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 10px; width: 100%; box-sizing: border-box;">
                        <button 
                            on:click={() => toggleSubmenu('conta')}
                            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
                        >
                            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
                                <span style="font-size: 1.1rem; flex-shrink: 0;">🔒</span>
                                <strong style="font-family:'Oswald'; font-size: 0.95rem; color: var(--cat-black); word-break: break-word;">Segurança & Senha de Saque</strong>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-left: auto;">
                                <span style="font-size: 0.58rem; background: #E8F5E9; border: 1px solid #2e7d32; color: #2e7d32; padding: 2px 6px; font-weight: 800; border-radius: 2px; white-space: nowrap;">
                                    {user.securityPassword ? 'DEFINIDA ✅' : 'PENDENTE ⚠️'}
                                </span>
                                <span style="font-size: 0.8rem; color: #555; flex-shrink: 0;">{submenuOpen.conta ? '▲' : '▼'}</span>
                            </div>
                        </button>

                        {#if submenuOpen.conta}
                            <div style="padding: 14px; border-top: 1.5px dashed var(--cat-black); background: #fafafa; width: 100%; box-sizing: border-box;">
                                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                                    <span>Status de Retiradas:</span>
                                    <span class="highlight-green" style="font-weight: 800;">LIBERADO PARA SAQUE ✅</span>
                                </div>
                                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                                    <span>Proteção SSL:</span>
                                    <span class="mono-text">256-BIT ENCRYPTION ATIVA 🔐</span>
                                </div>
                                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                                    <span>Senha de Retirada:</span>
                                    <span class={user.securityPassword ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                                        {user.securityPassword ? 'DEFINIDA ✅' : 'PENDENTE DE CADASTRO ⚠️'}
                                    </span>
                                </div>

                                <div style="margin-top: 12px; background: #fff; border: 1.5px solid var(--cat-black); padding: 12px; border-radius: 4px; box-shadow: 2px 2px 0px rgba(0,0,0,0.1); width: 100%; box-sizing: border-box;">
                                    <label class="input-field-label" for="sec-pass-input" style="font-size: 0.68rem; display: block; margin-bottom: 4px;">Cadastrar / Alterar Senha de Saque (6 dígitos):</label>
                                    <input id="sec-pass-input" type="password" maxlength="6" placeholder="******" class="input-field" bind:value={inputPassword} style="letter-spacing: 6px; font-weight: 800; font-size: 1.1rem; text-align: center; width: 100%; box-sizing: border-box; padding: 8px; border: 2px solid var(--cat-black); border-radius: 2px;">
                                    <button class="btn btn-primary" on:click={saveSecurityPassword} style="margin-top: 10px; font-size: 0.8rem; padding: 10px; width: 100%; box-sizing: border-box;">
                                        🔒 SALVAR SENHA FINANCEIRA
                                    </button>
                                </div>
                            </div>
                        {/if}
                    </div>

                    <!-- SMART ACCORDION 2: PROGRESSO DE CARREIRA -->
                    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 10px; width: 100%; box-sizing: border-box;">
                        <button 
                            on:click={() => toggleSubmenu('niveis')}
                            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
                        >
                            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
                                <span style="font-size: 1.1rem; flex-shrink: 0;">📈</span>
                                <strong style="font-family:'Oswald'; font-size: 0.95rem; color: var(--cat-black); word-break: break-word;">Progresso de Carreira</strong>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-left: auto;">
                                <span style="font-size: 0.58rem; background: #FFF8E1; border: 1px solid var(--cat-yellow); color: var(--cat-black); padding: 2px 6px; font-weight: 800; border-radius: 2px; white-space: nowrap;">
                                    NÍVEL {user.nivel} ({user.xp} XP)
                                </span>
                                <span style="font-size: 0.8rem; color: #555; flex-shrink: 0;">{submenuOpen.niveis ? '▲' : '▼'}</span>
                            </div>
                        </button>

                        {#if submenuOpen.niveis}
                            <div style="padding: 14px; border-top: 1.5px dashed var(--cat-black); background: #fafafa; width: 100%; box-sizing: border-box;">
                                <h5 style="font-family:'Oswald'; font-size: 0.85rem; margin-bottom: 8px; color: var(--cat-black);">
                                    📋 MISSÕES PARA ALCANÇAR NÍVEL {user.nivel + 1}:
                                </h5>

                                <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 6px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 0.75rem;">
                                    <span>🚜 1. Coletas na Garagem</span>
                                    <strong class={user.coletasCompleted >= 5 ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                                        {user.coletasCompleted} / 5 {user.coletasCompleted >= 5 ? '✅' : '⏳'}
                                    </strong>
                                </div>

                                <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 6px; border-radius: 2px; font-size: 0.75rem;">
                                    <div style="display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap;">
                                        <span>🛒 2. Contrato Pesado</span>
                                        <strong class={user.tratorRented ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                                            {user.tratorRented ? '1 / 1 ✅' : '0 / 1 ⏳'}
                                        </strong>
                                    </div>
                                    <p style="font-size: 0.62rem; color: #666; margin-top: 4px; line-height: 1.3;">Requisito: Alugue o Trator D8T na Loja de máquinas.</p>
                                </div>

                                <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 8px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 0.75rem;">
                                    <span>👥 3. Qualificação de Rede</span>
                                    <strong class={referrals.filter(r => r.status === 'ativo').length === 3 ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                                        {referrals.filter(r => r.status === 'ativo').length} / 3 {referrals.filter(r => r.status === 'ativo').length === 3 ? '✅' : '⏳'}
                                    </strong>
                                </div>

                                <button 
                                    class="btn" 
                                    disabled={!(user.coletasCompleted >= 5 && user.tratorRented && referrals.filter(r => r.status === 'ativo').length === 3)} 
                                    on:click={levelUp}
                                    style="margin-top: 8px; padding: 10px; font-size: 0.8rem; width: 100%; box-sizing: border-box; background: {user.coletasCompleted >= 5 && user.tratorRented && referrals.filter(r => r.status === 'ativo').length === 3 ? 'var(--cat-yellow)' : 'var(--cat-light-gray)'};"
                                >
                                    {user.coletasCompleted >= 5 && user.tratorRented && referrals.filter(r => r.status === 'ativo').length === 3 ? '⭐ PROMOVER CARREIRA DE OPERADOR!' : '🔒 MISSÕES PENDENTES'}
                                </button>
                            </div>
                        {/if}
                    </div>

                    <!-- SMART ACCORDION 3: REDE DE AFILIADOS -->
                    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 10px; width: 100%; box-sizing: border-box;">
                        <button 
                            on:click={() => toggleSubmenu('convidados')}
                            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
                        >
                            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
                                <span style="font-size: 1.1rem; flex-shrink: 0;">👥</span>
                                <strong style="font-family:'Oswald'; font-size: 0.95rem; color: var(--cat-black); word-break: break-word;">Rede & Indicação (5%)</strong>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-left: auto;">
                                <span style="font-size: 0.58rem; background: {user.referralBonusSuspended ? '#FFEBEE' : '#E8F5E9'}; border: 1px solid {user.referralBonusSuspended ? '#d32f2f' : '#2e7d32'}; color: {user.referralBonusSuspended ? '#d32f2f' : '#2e7d32'}; padding: 2px 6px; font-weight: 800; border-radius: 2px; white-space: nowrap;">
                                    {user.referralBonusSuspended ? 'SUSPENSO ⚠️' : '5% ATIVO ✅'}
                                </span>
                                <span style="font-size: 0.8rem; color: #555; flex-shrink: 0;">{submenuOpen.convidados ? '▲' : '▼'}</span>
                            </div>
                        </button>

                        {#if submenuOpen.convidados}
                            <div style="padding: 14px; border-top: 1.5px dashed var(--cat-black); background: #fafafa; width: 100%; box-sizing: border-box;">
                                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                                    <span>Operadores Diretos Ativos:</span>
                                    <span class="mono-text highlight-green" style="font-weight: 800;">{referrals.filter(r => r.status === 'ativo').length} / {referrals.length} Ativos</span>
                                </div>
                                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                                    <span>Comissão de Produção:</span>
                                    <span class={user.referralBonusSuspended ? 'highlight-red' : 'highlight-green'} style="font-weight: 800;">
                                        {user.referralBonusSuspended ? '0.0% (Suspenso)' : '5.0% Liberação Direta'}
                                    </span>
                                </div>

                                <div style="margin-top: 10px;">
                                    {#each referrals as c}
                                        <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 6px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 6px;">
                                            <div>
                                                <strong style="display:block; font-size:0.8rem; color: var(--cat-black);">{c.nome}</strong>
                                                <span style="font-size:0.62rem; color:#666;">Data: {c.data}</span>
                                            </div>
                                            <div style="text-align:right;">
                                                <span class={c.status === 'ativo' ? 'status-ativo' : 'status-inativo'} style="display:inline-block; font-size:0.65rem; font-weight: 800; padding: 1px 4px;">
                                                    {c.status.toUpperCase()} {c.status === 'ativo' ? '✅' : '⚠️'}
                                                </span>
                                            </div>
                                        </div>
                                    {/each}
                                </div>

                                <button class="btn btn-primary" on:click={copyInviteLink} style="margin-top: 10px; font-size: 0.8rem; padding: 10px; width: 100%; box-sizing: border-box;">
                                    📋 COPIAR LINK DE CONVITE
                                </button>
                            </div>
                        {/if}
                    </div>

                    <!-- SMART ACCORDION 4: HISTÓRICO FINANCEIRO -->
                    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 14px; width: 100%; box-sizing: border-box;">
                        <button 
                            on:click={() => toggleSubmenu('historico')}
                            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
                        >
                            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
                                <span style="font-size: 1.1rem; flex-shrink: 0;">📋</span>
                                <strong style="font-family:'Oswald'; font-size: 0.95rem; color: var(--cat-black); word-break: break-word;">Histórico Financeiro</strong>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-left: auto;">
                                <span style="font-size: 0.58rem; background: #E0F7FA; border: 1px solid #00838F; color: #00838F; padding: 2px 6px; font-weight: 800; border-radius: 2px; white-space: nowrap;">
                                    EXTRATO ({transactions.length})
                                </span>
                                <span style="font-size: 0.8rem; color: #555; flex-shrink: 0;">{submenuOpen.historico ? '▲' : '▼'}</span>
                            </div>
                        </button>

                        {#if submenuOpen.historico}
                            <div style="padding: 14px; border-top: 1.5px dashed var(--cat-black); background: #fafafa; width: 100%; box-sizing: border-box;">
                                <!-- Filtros Responsivos Flexíveis -->
                                <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px; width: 100%; box-sizing: border-box;">
                                    <button 
                                        class="btn" 
                                        on:click={() => txFilter = 'todos'}
                                        style="padding: 6px 10px; font-size: 0.65rem; margin: 0; flex: 1 1 auto; border-color: var(--cat-black); background: {txFilter === 'todos' ? 'var(--cat-yellow)' : '#eee'}; color: var(--cat-black); font-weight: 800; white-space: nowrap;"
                                    >
                                        Todos ({transactions.length})
                                    </button>
                                    <button 
                                        class="btn" 
                                        on:click={() => txFilter = 'depositos'}
                                        style="padding: 6px 10px; font-size: 0.65rem; margin: 0; flex: 1 1 auto; border-color: var(--cat-black); background: {txFilter === 'depositos' ? 'var(--cat-yellow)' : '#eee'}; color: var(--cat-black); font-weight: 800; white-space: nowrap;"
                                    >
                                        Depósitos
                                    </button>
                                    <button 
                                        class="btn" 
                                        on:click={() => txFilter = 'saques'}
                                        style="padding: 6px 10px; font-size: 0.65rem; margin: 0; flex: 1 1 auto; border-color: var(--cat-black); background: {txFilter === 'saques' ? 'var(--cat-yellow)' : '#eee'}; color: var(--cat-black); font-weight: 800; white-space: nowrap;"
                                    >
                                        Saques
                                    </button>
                                    <button 
                                        class="btn" 
                                        on:click={() => txFilter = 'rendimentos'}
                                        style="padding: 6px 10px; font-size: 0.65rem; margin: 0; flex: 1 1 auto; border-color: var(--cat-black); background: {txFilter === 'rendimentos' ? 'var(--cat-yellow)' : '#eee'}; color: var(--cat-black); font-weight: 800; white-space: nowrap;"
                                    >
                                        Coletas
                                    </button>
                                </div>

                                <div class="transaction-list" style="width: 100%; box-sizing: border-box;">
                                    {#if getFilteredTransactions(transactions, txFilter).length === 0}
                                        <div style="text-align: center; font-size: 0.75rem; color: #777; padding: 14px;">
                                            Nenhuma transação nesta categoria.
                                        </div>
                                    {:else}
                                        {#each getFilteredTransactions(transactions, txFilter) as tx}
                                            <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 6px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 6px; width: 100%; box-sizing: border-box;">
                                                <div style="min-width: 0;">
                                                    <span class="transaction-type" style="display:block; font-size: 0.75rem; font-weight: 700; word-break: break-word;">{tx.tipo}</span>
                                                    <span style="font-size:0.62rem; color:#666;">{tx.data}</span>
                                                </div>
                                                <div style="display:flex; align-items:center; gap:6px; flex-wrap: wrap;">
                                                    <span class="mono-text {tx.valor >= 0 ? 'highlight-green' : 'highlight-red'}" style="font-size: 0.78rem; font-weight: 800;">
                                                        {tx.valor >= 0 ? '+' : ''}R$ {Math.abs(tx.valor).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                                                    </span>
                                                    <span class="status-badge {tx.status === 'sucesso' ? 'sucesso' : 'pendente'}" style="font-size: 0.55rem; padding: 2px 4px; border: 1px solid var(--cat-black); font-weight: 800; text-transform: uppercase;">
                                                        {tx.status}
                                                    </span>
                                                </div>
                                            </div>
                                        {/each}
                                    {/if}
                                </div>
                            </div>
                        {/if}
                    </div>

                    <!-- Botão de Finalizar Turno (Logout) -->
                    <button class="btn btn-primary" on:click={handleLogout} style="background: var(--cat-black); color: var(--cat-yellow); border-color: var(--cat-black); width: 100%; box-sizing: border-box; font-family: 'Oswald', sans-serif; padding: 12px; font-size: 0.9rem;">
                        🚪 FINALIZAR TURNO (LOGOUT)
                    </button>

                    <!-- Área de Testes QA Discreta -->
                    <div style="margin-top: 12px; text-align: center; width: 100%; box-sizing: border-box;">
                        <button 
                            on:click={() => toggleSubmenu('eventos')}
                            style="background: transparent; border: none; font-size: 0.62rem; color: #999; text-decoration: underline; cursor: pointer;"
                        >
                            {submenuOpen.eventos ? '▲ Ocultar Testes' : '▼ Opções de Teste QA'}
                        </button>
                        
                        {#if submenuOpen.eventos}
                            <div style="margin-top: 6px; background: #F5F5F5; border: 1px solid #ccc; padding: 8px; border-radius: 2px; text-align: left; width: 100%; box-sizing: border-box;">
                                <div style="display: flex; gap: 4px; margin-bottom: 4px; flex-wrap: wrap;">
                                    <button class="btn" on:click={() => simulateReferral(false)} style="font-size:0.58rem; padding: 4px; margin:0; flex:1; background:#ef5350; color:white;">Inativar Maria</button>
                                    <button class="btn" on:click={() => simulateReferral(true)} style="font-size:0.58rem; padding: 4px; margin:0; flex:1; background:#66bb6a; color:white;">Ativar Todos</button>
                                </div>
                                <button class="btn" on:click={resetRoulette} style="font-size:0.58rem; padding: 4px; margin:0 0 4px 0; background:var(--cat-gray); color:white; width:100%;">🔄 Reset Roleta</button>
                                <button class="btn" on:click={fullReset} style="font-size:0.58rem; padding: 4px; margin:0; background:var(--cat-red); color:white; width:100%;">🔄 Reset Full</button>
                            </div>
                        {/if}
                    </div>

                </div>
            {/if}

        </main>

        {#if token && currentTab === 'garagem'}
            <!-- BOTÃO FLUTUANTE DE CONVITES (👥 FAB) -->
            <button class="fab" on:click={() => showInviteModal = true} style="border:none; cursor:pointer;">👥</button>

            <!-- BOTÃO FLUTUANTE DE EVENTOS (📢 FAB) -->
            <button class="floating-events-btn" on:click={() => showEventsModal = true} aria-label="Central de Eventos">
                📢
                {#if referrals.filter(r => r.status === 'ativo').length < 3}
                    <span class="fab-badge">!</span>
                {/if}
            </button>
        {/if}

        <!-- NAVBAR INFERIOR -->
        <nav class="bottom-nav">
            <button class="nav-item {currentTab === 'garagem' ? 'active' : ''}" on:click={() => currentTab = 'garagem'}>
                <span class="nav-icon">🚜</span> Garagem
                {#if garage.some(m => m.state === 'disponivel')}
                    <span class="nav-badge">!</span>
                {/if}
            </button>
            <button class="nav-item {currentTab === 'loja' ? 'active' : ''}" on:click={() => currentTab = 'loja'}>
                <span class="nav-icon">🛒</span> Loja
            </button>
            <button class="nav-item {currentTab === 'config' ? 'active' : ''}" on:click={() => currentTab = 'config'}>
                <span class="nav-icon">⚙️</span> Ajustes
            </button>
        </nav>
    {/if}
</div>

<!-- ============ POPUPS & MODAIS PROTEGIDOS ============ -->

{#if token}
    <!-- 1. Modal Convite -->
    {#if showInviteModal}
        <div class="modal-overlay open">
            <div class="modal-card">
                <div class="modal-header">
                    <h2>👥 Convidar Operadores</h2>
                    <button class="close-modal" on:click={() => showInviteModal = false}>✕</button>
                </div>
                <div class="modal-body">
                    <p style="font-size:0.75rem; color:var(--cat-gray); line-height:1.4; margin-bottom:12px;">
                        Compartilhe seu link de convite da CatBrasil. Cada operador indicado qualificado gera bônus adicionais. Lembre-se: indicados inativos suspendem seus rendimentos!
                    </p>
                    <div class="input-field-wrapper">
                        <label class="input-field-label" for="refLink">Seu Link de Afiliado:</label>
                        <input type="text" value="https://catbrasil.app/ref/{user.id}" readonly class="input-field" id="refLink" on:click={(e) => e.target.select()}>
                    </div>
                    <div class="evento-stats" style="padding:6px 12px; margin-top:8px; margin-bottom:12px; font-size:0.65rem; text-align: center; box-shadow: none;">
                        Código de Indicação: <strong style="color:var(--cat-red);">{user.id}</strong>
                    </div>
                    <button class="btn btn-primary" on:click={copyRefLink}>📋 COPIAR LINK DE CONVITE</button>
                </div>
            </div>
        </div>
    {/if}

    <!-- 2. Modal Depósito -->
    {#if showDepositModal}
        <div class="modal-overlay open">
            <div class="modal-card">
                <div class="modal-header">
                    <h2>💵 Realizar Depósito</h2>
                    <button class="close-modal" on:click={() => showDepositModal = false}>✕</button>
                </div>
                <div class="modal-body">
                    <p style="font-size:0.75rem; color:var(--cat-gray); margin-bottom:10px;">
                        Adicione fundos à sua carteira operária instantaneamente via PIX da CatBrasil.
                    </p>
                    <div class="input-field-wrapper">
                        <label class="input-field-label" for="depositoAmount">Valor do Depósito (R$):</label>
                        <input type="number" placeholder="Mínimo 100.00" class="input-field" id="depositoAmount" bind:value={depositAmount}>
                    </div>
                    <div class="evento-stats" style="padding:8px 12px; margin-top:8px; margin-bottom:12px; font-size:0.7rem; box-shadow: none;">
                        💡 Depósitos acima de R$ 1.000 recebem bônus de 10% da campanha.
                    </div>
                    <button class="btn btn-claim" on:click={processDeposit}>⚡ GERAR PIX / CONFIRMAR</button>
                </div>
            </div>
        </div>
    {/if}

    <!-- 2.5 Modal Pagamento PIX Copia e Cola -->
    {#if showPixPayModal}
        <div class="modal-overlay open">
            <div class="modal-card" style="max-width: 440px;">
                <div class="modal-header" style="background: var(--cat-black); color: var(--cat-yellow);">
                    <h2>⚡ Pagamento PIX CatBrasil</h2>
                    <button class="close-modal" on:click={() => showPixPayModal = false} style="color:white;">✕</button>
                </div>
                <div class="modal-body" style="text-align: center;">
                    <div style="background: var(--cat-yellow); color: var(--cat-black); padding: 8px; font-weight: bold; border-radius: 4px; margin-bottom: 12px; font-size: 0.85rem; border: 1.5px solid var(--cat-black);">
                        Valor do Depósito: R$ {currentPixValue.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}
                    </div>

                    <!-- QR Code Simulado Visual SVG -->
                    <div style="background: #fff; border: 3px solid var(--cat-black); padding: 12px; display: inline-block; border-radius: 8px; box-shadow: 4px 4px 0px var(--cat-black); margin-bottom: 12px;">
                        <svg width="140" height="140" viewBox="0 0 100 100" style="display:block;">
                            <rect width="100" height="100" fill="#ffffff" />
                            <rect x="5" y="5" width="30" height="30" fill="#111111" />
                            <rect x="10" y="10" width="20" height="20" fill="#ffffff" />
                            <rect x="15" y="15" width="10" height="10" fill="#111111" />

                            <rect x="65" y="5" width="30" height="30" fill="#111111" />
                            <rect x="70" y="10" width="20" height="20" fill="#ffffff" />
                            <rect x="75" y="15" width="10" height="10" fill="#111111" />

                            <rect x="5" y="65" width="30" height="30" fill="#111111" />
                            <rect x="10" y="70" width="20" height="20" fill="#ffffff" />
                            <rect x="15" y="75" width="10" height="10" fill="#111111" />

                            <rect x="42" y="10" width="8" height="8" fill="#111111" />
                            <rect x="52" y="18" width="8" height="8" fill="#111111" />
                            <rect x="42" y="42" width="16" height="16" fill="#111111" />
                            <rect x="65" y="42" width="8" height="8" fill="#111111" />
                            <rect x="80" y="52" width="10" height="10" fill="#111111" />
                            <rect x="42" y="65" width="12" height="12" fill="#111111" />
                            <rect x="60" y="70" width="15" height="15" fill="#111111" />
                            <rect x="80" y="80" width="15" height="15" fill="#111111" />
                        </svg>
                    </div>

                    <div style="font-size:0.75rem; color:#555; margin-bottom: 8px;">
                        Escaneie o QR Code no app do seu banco ou use o código PIX:
                    </div>

                    <div style="background:#f4f4f4; border: 1.5px solid #ccc; padding: 6px 10px; border-radius: 4px; font-size: 0.65rem; word-break: break-all; font-family: monospace; max-height: 48px; overflow-y: auto; margin-bottom: 10px;">
                        {pixCodeString}
                    </div>

                    <button class="btn btn-claim" on:click={copyPixCode} style="width: 100%; margin-bottom: 8px; font-size: 0.8rem; border-color: var(--cat-black);">
                        📋 COPIAR CÓDIGO PIX COPIA E COLA
                    </button>

                    <div style="font-size: 0.7rem; color: var(--cat-red); font-weight: bold; margin-bottom: 12px;">
                        ⏱️ QR Code expira em {Math.floor(pixTimerSeconds / 60)}m {pixTimerSeconds % 60}s
                    </div>

                    <button class="btn btn-primary" on:click={confirmPixPayment} style="width: 100%; background: var(--cat-black); color: var(--cat-yellow); font-size: 0.8rem; border-color: var(--cat-black);">
                        ✅ JÁ REALIZEI O PAGAMENTO (CONFIRMAR PIX)
                    </button>
                </div>
            </div>
        </div>
    {/if}

    <!-- 3. Modal Saque -->
    {#if showWithdrawModal}
        <div class="modal-overlay open">
            <div class="modal-card">
                <div class="modal-header">
                    <h2>💰 Retirada de Saldo</h2>
                    <button class="close-modal" on:click={() => showWithdrawModal = false}>✕</button>
                </div>
                <div class="modal-body">
                    <div class="data-row" style="margin-bottom:8px;"><span>Saldo disponível:</span><strong class="highlight-yellow">R$ {user.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</strong></div>
                    <div class="input-field-wrapper" style="margin-bottom:8px;">
                        <label class="input-field-label" for="saqueAmount">Valor do Saque (R$):</label>
                        <input type="number" placeholder="Mínimo 50.00" class="input-field" id="saqueAmount" bind:value={withdrawAmount}>
                    </div>
                    <div class="input-field-wrapper">
                        <label class="input-field-label" for="saqueSenha">Senha de Retirada (6 dígitos):</label>
                        <input type="password" maxlength="6" placeholder="Digite sua senha de segurança" class="input-field" id="saqueSenha" bind:value={withdrawPassword} style="letter-spacing:4px;">
                    </div>
                    <div style="font-size:0.65rem; color:#666; margin-bottom:12px;" id="withdrawFeeText">
                        {#if user.freeWithdrawActive}
                            🌟 <strong>Taxa de Saque: GRÁTIS (Giro Premiado Ativo! 🎁)</strong>
                        {:else}
                            * Taxa operacional de saque padrão: 5%.
                        {/if}
                    </div>
                    <button class="btn btn-primary" on:click={processWithdraw}>💰 SOLICITAR RETIRADA</button>
                </div>
            </div>
        </div>
    {/if}

    <!-- 4. Modal Novidades e Roleta Diária -->
    {#if showAnnouncementsModal}
        <div class="modal-overlay open">
            <div class="modal-card" style="max-width: 420px; width: 95%;">
                <div class="modal-header">
                    <h2>📢 Central de Novidades</h2>
                    <button class="close-modal" id="annCloseBtn" on:click={tryCloseAnnouncements} style="font-size: 0.8rem; padding: 4px 8px; background: #333; border-radius: 4px; display: flex; align-items: center; gap: 4px; border: 1.5px solid var(--cat-yellow); font-family: 'Oswald', sans-serif;">
                        <span>{allowAnnounceClose ? '✅' : '🔒'}</span> <span>{allowAnnounceClose ? 'Fechar' : 'Bloqueado'}</span>
                    </button>
                </div>
                
                <div class="ann-carousel-wrapper">
                    <div class="ann-carousel-track" style="transform: translateX(-{announceSlideIdx * 25}%); transition: transform 0.4s ease-in-out;">
                        
                        <!-- Slide 1 -->
                        <div class="ann-carousel-slide">
                            <div class="ann-slide-header" style="text-align: center; margin-bottom: 12px;">
                                <span style="font-size: 3rem; display: block; margin-bottom: 8px;">👋</span>
                                <h3 style="font-size: 1.3rem; color: var(--cat-black); margin-bottom:2px;">Bem-vindo ao Turno!</h3>
                                <p style="font-size: 0.65rem; text-transform: uppercase; font-weight: 700; color: #777; letter-spacing: 1px;">Aviso Operacional de Entrada</p>
                            </div>
                            <div class="ann-slide-body" style="font-size: 0.85rem; color: #333; line-height: 1.5; text-align: center; flex: 1; display: flex; align-items: center; justify-content: center; flex-direction: column;">
                                <p>Operador, novas diretrizes e atualizações foram implantadas na central CatBrasil.</p>
                            </div>
                        </div>

                        <!-- Slide 2 -->
                        <div class="ann-carousel-slide">
                            <div class="ann-slide-header" style="text-align: center; margin-bottom: 10px;">
                                <span style="font-size: 3rem; display: block; margin-bottom: 8px;">🛒</span>
                                <h3 style="font-size: 1.3rem; color: var(--cat-black); margin-bottom:2px;">Novidades na Loja</h3>
                                <p style="font-size: 0.65rem; text-transform: uppercase; font-weight: 700; color: #777; letter-spacing: 1px;">Novas Frotas de Aluguel</p>
                            </div>
                            <div class="ann-slide-body" style="flex: 1; display: flex; flex-direction: column; justify-content: center;">
                                <p style="font-size: 0.8rem; color: #444; line-height: 1.4; margin-bottom: 10px; text-align: center;">Um novo contrato de peso pesado acaba de chegar no pátio:</p>
                                <div class="store-card" style="margin-bottom:0; box-shadow: none; border-color: #555;">
                                    <div class="store-card-img" style="background-image:url('https://images.unsplash.com/photo-1621932953936-056b1b4d0eb0?w=200&q=80'); width: 90px;"></div>
                                    <div class="store-card-body" style="padding: 8px 10px;">
                                        <h4 class="store-card-title" style="font-size:0.85rem;">Trator D8T</h4>
                                        <div class="data-row" style="font-size:0.65rem; margin:2px 0 0; padding:0;"><span>Custo:</span><strong>R$ 10.000,00</strong></div>
                                        <div class="data-row" style="font-size:0.65rem; margin:0; padding:0;"><span>Retorno:</span><strong class="highlight-green">R$ 11.000,00</strong></div>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Slide 3 -->
                        <div class="ann-carousel-slide">
                            <div class="ann-slide-header" style="text-align: center; margin-bottom: 12px;">
                                <span style="font-size: 3rem; display: block; margin-bottom: 8px;">📢</span>
                                <h3 style="font-size: 1.3rem; color: var(--cat-black); margin-bottom:2px;">Eventos Ativos</h3>
                                <p style="font-size: 0.65rem; text-transform: uppercase; font-weight: 700; color: #777; letter-spacing: 1px;">Multiplicadores de Lucro</p>
                            </div>
                            <div class="ann-slide-body" style="font-size: 0.85rem; color: #333; line-height: 1.5; text-align: center; flex: 1; display: flex; flex-direction: column; justify-content: center;">
                                <p>🌴 <strong>Campanha de Verão:</strong> +20% de retorno em todas as coletas diárias durante o mês de julho!</p>
                                <div style="background: #FFE082; border: 1.5px solid var(--cat-black); padding: 8px; font-size: 0.72rem; margin-top: 12px; font-weight: 700; border-radius: 2px;">
                                    👥 Mutirão de Indicações ativo! Convide 3 operadores e resgate R$ 500,00 adicionais.
                                </div>
                            </div>
                        </div>

                        <!-- Slide 4 -->
                        <div class="ann-carousel-slide">
                            <div class="ann-slide-header" style="text-align: center; margin-bottom: 8px;">
                                <h3 style="font-size: 1.2rem; color: var(--cat-black); margin-bottom:2px;">🎡 Giro Diário Premiado</h3>
                                <p style="font-size: 0.65rem; text-transform: uppercase; font-weight: 700; color: #777; letter-spacing: 1px;">Resgate sua Recompensa Grátis</p>
                            </div>
                            <div class="ann-slide-body" style="display: flex; flex-direction: column; align-items: center; justify-content: center; flex: 1;">
                                <p style="font-size: 0.75rem; color: #555; text-align: center; margin-bottom: 6px;">Gire a roleta operária agora e acumule bônus em Reais ou XP!</p>
                                
                                <div class="roulette-container" style="margin: 0;">
                                    <div class="wheel-pointer">▼</div>
                                    <div class="wheel-outer" style="width: 140px; height: 140px; border-width: 3px; transform: rotate({rouletteTargetDegrees}deg); transition: transform 4s cubic-bezier(0.1, 0.8, 0.1, 1);">
                                        <div class="wheel-inner-line" style="transform: rotate(30deg);"></div>
                                        <div class="wheel-inner-line" style="transform: rotate(90deg);"></div>
                                        <div class="wheel-inner-line" style="transform: rotate(150deg);"></div>
                                        
                                        <div class="wheel-sector" style="transform: rotate(30deg); padding-top: 8px; font-size: 0.52rem;">💀 Vazio</div>
                                        <div class="wheel-sector" style="transform: rotate(90deg); padding-top: 8px; font-size: 0.52rem;">⚡ 10 XP</div>
                                        <div class="wheel-sector" style="transform: rotate(150deg); padding-top: 8px; font-size: 0.52rem;">⚡ 20 XP</div>
                                        <div class="wheel-sector" style="transform: rotate(210deg); padding-top: 8px; font-size: 0.52rem; font-weight:800; color:var(--cat-red);">🎁 Promo 300</div>
                                        <div class="wheel-sector" style="transform: rotate(270deg); padding-top: 8px; font-size: 0.52rem; font-weight:800; color:var(--cat-red);">🎫 S. Grátis</div>
                                        <div class="wheel-sector" style="transform: rotate(330deg); padding-top: 8px; font-size: 0.52rem;">💀 Vazio</div>
                                    </div>
                                    <div style="margin-top:6px; display:flex; align-items:center; gap:4px; font-size:0.62rem; background:#fff; border:1px solid var(--cat-black); padding:2px 4px; border-radius:2px; justify-content: center; width: 150px;">
                                        <span>⚙️ Probabilidade:</span>
                                        <select bind:value={qaRouletteMode} style="font-size:0.6rem; border:none; background:none; font-weight:800; outline:none; cursor:pointer;">
                                            <option value="normal">Normal (5% bom)</option>
                                            <option value="promo">Forçar Promo 300</option>
                                            <option value="free">Forçar Saque Grátis</option>
                                        </select>
                                    </div>
                                    <button class="btn btn-claim" disabled={user.alreadySpunToday || rouletteSpinning} on:click={spinRoulette} style="margin-top: 6px; padding: 6px; font-size: 0.75rem; width: 150px;">
                                        {user.alreadySpunToday ? '✅ GIRO CONCLUÍDO' : spinBtnText}
                                    </button>
                                </div>
                            </div>
                        </div>

                    </div>
                </div>
                
                <div class="ann-footer-nav">
                    <button class="btn-action-small" disabled={announceSlideIdx === 0} on:click={prevAnnounceSlide} style="margin: 0; width: 80px; font-family: 'Oswald', sans-serif;">⬅️ Voltar</button>
                    
                    <div class="carousel-dots-container">
                        <span class="carousel-dot-indicator {announceSlideIdx === 0 ? 'active' : ''}"></span>
                        <span class="carousel-dot-indicator {announceSlideIdx === 1 ? 'active' : ''}"></span>
                        <span class="carousel-dot-indicator {announceSlideIdx === 2 ? 'active' : ''}"></span>
                        <span class="carousel-dot-indicator {announceSlideIdx === 3 ? 'active' : ''}"></span>
                    </div>

                    <button class="btn-action-small" disabled={announceSlideIdx === 3} on:click={nextAnnounceSlide} style="margin: 0; width: 80px; font-family: 'Oswald', sans-serif;">Avançar ➔</button>
                </div>
            </div>
        </div>
    {/if}

    <!-- 5. Modal Level Up -->
    {#if showLevelUpModal}
        <div class="modal-overlay open">
            <div class="modal-card" style="max-width: 340px; text-align: center;">
                <div class="modal-header" style="background: var(--cat-yellow); color: var(--cat-black); border-bottom-color: var(--cat-black);">
                    <h2>⭐ CARREIRA PROMOVIDA</h2>
                    <button class="close-modal" on:click={() => showLevelUpModal = false} style="color: var(--cat-black);">✕</button>
                </div>
                <div class="modal-body" style="padding: 20px;">
                    <span style="font-size: 3.5rem; display: block; margin-bottom: 10px; animation: shake 1s infinite;">🏆</span>
                    <h3 style="font-size: 1.3rem; color: var(--cat-black);">NÍVEL {user.nivel} ALCANÇADO</h3>
                    <p style="font-size: 0.65rem; text-transform: uppercase; font-weight: 700; color: #777; letter-spacing: 1px; margin-top: 2px;">Categoria: Operador Master</p>
                    
                    <div class="evento-stats" style="margin-top: 14px; text-align: left; padding: 12px; font-size: 0.72rem; border-color: var(--cat-green); background: #E8F5E9; box-shadow: none;">
                        <strong style="color: var(--cat-green); display: block; margin-bottom: 4px;">🎁 Vantagem Desbloqueada:</strong>
                        Seu multiplicador de produção subiu! Bônus de +{((user.nivel - 1) * 5).toFixed(1)}% ativo na produção diária.
                    </div>
                    
                    <button class="btn btn-primary" on:click={() => showLevelUpModal = false} style="margin-top: 16px;">VOLTAR AO TRABALHO</button>
                </div>
            </div>
        </div>
    {/if}

    <!-- 6. Modal Central de Eventos -->
    {#if showEventsModal}
        <div class="modal-overlay open" style="z-index: 1500;">
            <div class="modal-card" style="max-width: 420px; width: 95%;">
                <div class="modal-header">
                    <h2>📢 Central de Eventos</h2>
                    <button class="close-modal" on:click={() => showEventsModal = false}>✕</button>
                </div>
                <div class="modal-body" style="padding: 16px 12px;">
                    <div class="eventos-carousel" style="overflow-x: auto; display: flex; gap: 12px; scroll-snap-type: x mandatory; padding-bottom: 8px;">
                        
                        <!-- Slide 1 -->
                        <div class="evento-slide" style="scroll-snap-align: start; flex-shrink: 0; width: 280px; background: #fff; border: 2.2px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); padding: 12px; display: flex; flex-direction: column; justify-content: space-between; border-radius: 2px; min-height: 230px;">
                            <div>
                                <span class="evento-tag" style="background: var(--cat-yellow); color: var(--cat-black); font-size: 0.55rem; padding: 2px 4px; border: 1.5px solid var(--cat-black); border-radius: 2px; font-weight:800; text-transform:uppercase; display:inline-block;">Campanha Sazonal</span>
                                <p style="font-size:0.6rem; color:#666; margin-top: 4px; font-weight:700;">⏱️ 12d 04h restantes</p>
                                <h3 style="font-size:0.95rem; margin-top: 6px; font-family:'Oswald'; color: var(--cat-black);">🎉 Verão CAT: +20% Rendimento</h3>
                                <p style="font-size:0.7rem; color:#444; line-height: 1.3; margin-top: 4px;">Todas as máquinas contratadas em Julho recebem acréscimo de 20% no valor de retorno diário coletado.</p>
                            </div>
                            <div style="margin-top: 8px;">
                                <div class="evento-stats" style="font-size: 0.65rem; padding: 6px; box-shadow: none; border-color: #777; margin-bottom: 6px;">
                                    <div class="data-row"><span>Status:</span><span class="highlight-green" style="font-weight:800;">ATIVO</span></div>
                                    <div class="data-row"><span>Boost Extra:</span><span class="mono-text highlight-green">+20%</span></div>
                                </div>
                                <button class="btn btn-primary" on:click={() => showToast('Você já está participando automaticamente! Aproveite os ganhos extras.', 'win')} style="padding: 6px; font-size: 0.7rem; margin: 0; width: 100%;">PARTICIPANDO</button>
                            </div>
                        </div>
                        
                        <!-- Slide 2 -->
                        <div class="evento-slide" style="scroll-snap-align: start; flex-shrink: 0; width: 280px; background: #fff; border: 2.2px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); padding: 12px; display: flex; flex-direction: column; justify-content: space-between; border-radius: 2px; min-height: 230px;">
                            <div>
                                <span class="evento-tag" style="background: var(--cat-yellow); color: var(--cat-black); font-size: 0.55rem; padding: 2px 4px; border: 1.5px solid var(--cat-black); border-radius: 2px; font-weight:800; text-transform:uppercase; display:inline-block;">Bônus Especial</span>
                                <p style="font-size:0.6rem; color:#666; margin-top: 4px; font-weight:700;">⏱️ 2d 18h restantes</p>
                                <h3 style="font-size:0.95rem; margin-top: 6px; font-family:'Oswald'; color: var(--cat-black);">👥 Mutirão de Operadores</h3>
                                <p style="font-size:0.7rem; color:#444; line-height: 1.3; margin-top: 4px;">Convide 3 operadores ativos e qualificados (cadastro, depósito e máquina ativa) e ganhe R$ 500,00.</p>
                            </div>
                            <div style="margin-top: 8px;">
                                <div class="evento-stats" style="font-size: 0.65rem; padding: 6px; box-shadow: none; border-color: #777; margin-bottom: 6px;">
                                    <div class="data-row"><span>Progresso:</span><strong class="mono-text">{referrals.filter(r => r.status === 'ativo').length} / 3 indicados</strong></div>
                                    <div class="prog-inline" style="height: 6px; border: 1px solid var(--cat-black); background: #eee; border-radius: 1px; margin-top: 2px; overflow:hidden;"><div class="prog-inline-fill" style="width: {(referrals.filter(r => r.status === 'ativo').length / 3) * 100}%; background: var(--cat-green); height: 100%;"></div></div>
                                </div>
                                <button class="btn btn-primary" on:click={() => { showEventsModal = false; showInviteModal = true; }} style="padding: 6px; font-size: 0.7rem; margin: 0; width: 100%;">📋 CONVIDAR AGORA</button>
                            </div>
                        </div>
                        
                        <!-- Slide 3 -->
                        <div class="evento-slide" style="scroll-snap-align: start; flex-shrink: 0; width: 280px; background: #fff; border: 2.2px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); padding: 12px; display: flex; flex-direction: column; justify-content: space-between; border-radius: 2px; min-height: 230px;">
                            <div>
                                <span class="evento-tag" style="background: var(--cat-yellow); color: var(--cat-black); font-size: 0.55rem; padding: 2px 4px; border: 1.5px solid var(--cat-black); border-radius: 2px; font-weight:800; text-transform:uppercase; display:inline-block;">Cashback Flash</span>
                                <p style="font-size:0.6rem; color:#666; margin-top: 4px; font-weight:700;">⏱️ 05h 12m restantes</p>
                                <h3 style="font-size:0.95rem; margin-top: 6px; font-family:'Oswald'; color: var(--cat-black);">💰 Depósito Turbinado (10%)</h3>
                                <p style="font-size:0.7rem; color:#444; line-height: 1.3; margin-top: 4px;">Depósitos a partir de R$ 1.000,00 ganham 10% de cashback extra em saldo de mineração.</p>
                            </div>
                            <div style="margin-top: 8px;">
                                <div class="evento-stats" style="font-size: 0.65rem; padding: 6px; box-shadow: none; border-color: #777; margin-bottom: 6px;">
                                    <div class="data-row"><span>Bônus:</span><span class="mono-text">10% extra</span></div>
                                    <div class="data-row"><span>Exemplo:</span><span class="mono-text highlight-green">+R$ 100,00</span></div>
                                </div>
                                <button class="btn btn-claim" on:click={() => { showEventsModal = false; showDepositModal = true; }} style="padding: 6px; font-size: 0.7rem; margin: 0; width: 100%;">💵 DEPOSITAR</button>
                            </div>
                        </div>
                    </div>
                    
                    <p style="font-size: 0.62rem; color: #777; text-align: center; margin-top: 10px; font-weight: 700;">Deslize para o lado para navegar pelos eventos ➔</p>
                </div>
            </div>
        </div>
    {/if}
{/if}

<!-- 5. Modal SMS de Verificação (Não Protegido por Token, para Cadastro) -->
{#if showSmsModal}
    <div class="modal-overlay open" style="z-index: 2000;">
        <div class="modal-card" style="max-width: 360px; width: 92%;">
            <div class="modal-header">
                <h2>💬 Verificação de Celular</h2>
                <button class="close-modal" on:click={() => showSmsModal = false}>✕</button>
            </div>
            <div class="modal-body" style="text-align: center;">
                <p style="font-size:0.75rem; color:var(--cat-gray); margin-bottom:12px; line-height: 1.4;">
                    Enviamos um código de confirmação de 6 dígitos via SMS para o número <strong>{regPhone}</strong>.
                </p>
                
                <!-- Balão de SMS Recebido Simulado (UX Interativa Premium!) -->
                <div style="background: #E8F5E9; border: 2.5px solid var(--cat-black); border-radius: 4px; padding: 12px; margin-bottom: 16px; text-align: left; box-shadow: 3px 3px 0px var(--cat-black);">
                    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:4px;">
                        <span style="font-size:0.65rem; font-weight:800; color:var(--cat-green); letter-spacing:0.5px;">🔔 SMS RECEBIDO AGORA</span>
                        <span style="font-size:0.6rem; color:#666; font-weight:700;">CatBrasil</span>
                    </div>
                    <p style="font-size:0.75rem; color:var(--cat-black); font-weight:700;">
                        Código de verificação de cadastro: <span style="color:var(--cat-red); font-size:0.9rem; font-family:'JetBrains Mono'; font-weight:900;">{generatedVerificationCode}</span>
                    </p>
                </div>

                <div class="login-input-group" style="text-align: center;">
                    <label for="verificationCodeInput" style="text-align: center; display:block; margin-bottom:6px;">Digite o código:</label>
                    <input type="text" maxlength="6" placeholder="000000" bind:value={verificationCodeInput} id="verificationCodeInput" style="text-align: center; letter-spacing: 6px; font-weight: 800; font-size: 1.3rem; border: 2.5px solid var(--cat-black); width: 85%; margin: auto; padding: 8px;">
                </div>

                <button class="login-btn-submit" on:click={handleRegister} style="margin-top: 18px; width: 100%;">
                    📝 CONFIRMAR E REGISTRAR
                </button>
            </div>
        </div>
    </div>
{/if}

<!-- TOAST CONTAINER -->
<div class="toast-container">
    {#each toasts as t (t.id)}
        <button type="button" class="toast {t.type}" on:click={() => removeToast(t.id)}>
            <span style="flex-grow: 1; text-align: left; font-weight: 600; line-height: 1.35;">{t.message}</span>
            <span class="toast-close">✕</span>
        </button>
    {/each}
</div>

<style>
    /* Estilos scoped podem ser definidos aqui se necessário. O layout principal utiliza src/app.css */
</style>
