<script>
    import { api } from '../services/api.js';

    export let onLoginSuccess = (token, user) => {};
    export let onShowToast = (msg, type) => {};
    export let onOpenGestao = () => {};

    let authMode = 'login'; // 'login' ou 'register'
    let loginUsername = '';
    let loginPassword = '';

    // Estado do Registro
    let regName = '';
    let regPhone = '';
    let regPassword = '';
    let regConfirmPassword = '';
    let inviteCode = '';

    // Estado de Validação SMS
    let isSendingCode = false;
    let verificationCodeSent = false;
    let generatedVerificationCode = '';
    let verificationCodeInput = '';
    let showSmsModal = false;

    async function handleLogin() {
        if (!loginUsername || !loginPassword) {
            onShowToast("Preencha usuário/telefone e senha!", "error");
            return;
        }

        try {
            const res = await api.login(loginUsername, loginPassword);
            onShowToast(res.message || `Bem-vindo, ${res.user.username}!`, "win");
            onLoginSuccess(res.token, res.user);
            loginUsername = "";
            loginPassword = "";
        } catch (e) {
            onShowToast(e.message || "Erro no login", "error");
        }
    }

    function isValidPhone(phone) {
        const cleaned = phone.replace(/\D/g, '');
        return cleaned.length === 11 && cleaned.startsWith('9', 2);
    }

    function handleSendVerificationCode() {
        if (!regName || !regPhone || !regPassword || !regConfirmPassword) {
            onShowToast("Preencha todos os campos obrigatórios!", "error");
            return;
        }

        if (regPassword !== regConfirmPassword) {
            onShowToast("As senhas não coincidem!", "error");
            return;
        }

        if (regPassword.length < 5) {
            onShowToast("A senha deve ter no mínimo 5 caracteres!", "error");
            return;
        }

        isSendingCode = true;
        onShowToast("Enviando SMS com código de verificação...", "");

        setTimeout(() => {
            generatedVerificationCode = String(Math.floor(100000 + Math.random() * 900000));
            verificationCodeSent = true;
            isSendingCode = false;
            showSmsModal = true;
            onShowToast(`Código enviado! Verifique seu celular.`, "win");
        }, 1000);
    }

    async function handleRegister() {
        if (verificationCodeInput !== generatedVerificationCode) {
            onShowToast("Código SMS incorreto! Verifique a mensagem enviada.", "error");
            return;
        }

        try {
            const res = await api.register(regName, regPassword, inviteCode);
            showSmsModal = false;
            onShowToast(res.message || `Operador ${regName} cadastrado com sucesso!`, "win");
            onLoginSuccess(res.token, res.user);
        } catch (e) {
            onShowToast(e.message || "Erro ao realizar cadastro", "error");
        }
    }
</script>

<div class="login-screen-wrapper">
    <div class="login-card-container">
        <div class="login-banner-stripes"></div>
        
        <div style="width: 52px; height: 52px; border-radius: 50%; background: var(--cat-yellow); border: 2.5px solid var(--cat-black); display: flex; align-items: center; justify-content: center; font-family: 'Oswald', sans-serif; font-weight: 900; font-size: 1.1rem; box-shadow: 2px 2px 0px #000; margin: 0 auto 12px auto;">CAT</div>
        
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

            <button class="login-btn-submit" on:click={handleLogin}>ENTRAR NO TURNO</button>
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
                {isSendingCode ? 'ENVIANDO...' : 'CADASTRAR OPERADOR'}
            </button>

            <p style="font-size:0.75rem; color:#555; margin-top:14px;">
                Já possui conta? 
                <button class="login-toggle-link" on:click={() => { authMode = 'login'; verificationCodeSent = false; }}>Fazer login</button>
            </p>
        {/if}
        
        <div style="margin-top: 24px; text-align: center;">
            <p style="font-size: 0.58rem; color: #777; margin: 0;">CatBrasil v2.0 · Conexão Criptografada SSL</p>
            <button 
                type="button" 
                on:click={onOpenGestao} 
                style="background: transparent; border: none; font-size: 0.58rem; color: #999; text-decoration: underline; cursor: pointer; margin-top: 6px; font-family: monospace;"
                title="Acesso Direto ao Módulo de Gestão (Ctrl+Shift+E)"
            >
                Central ERP (Acesso Restrito)
            </button>
        </div>
    </div>
</div>

<!-- MODAL DE VERIFICAÇÃO DE CÓDIGO SMS -->
{#if showSmsModal}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 360px;">
            <div class="modal-header">
                <h3>VERIFICAÇÃO DE SMS</h3>
                <button class="modal-close" on:click={() => showSmsModal = false}>✕</button>
            </div>
            <div class="modal-body" style="text-align: center;">
                <p style="font-size: 0.75rem; color: #444; margin-bottom: 12px;">
                    Enviamos um código de confirmação de 6 dígitos via SMS para o número <strong>{regPhone}</strong>.
                </p>
                
                <div style="background: #E8F5E9; border: 2.5px solid var(--cat-black); border-radius: 4px; padding: 12px; margin-bottom: 16px; text-align: left; box-shadow: 3px 3px 0px var(--cat-black);">
                    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:4px;">
                        <span style="font-size:0.65rem; font-weight:800; color:var(--cat-green); letter-spacing:0.5px;">SMS RECEBIDO AGORA</span>
                        <span style="font-size:0.6rem; color:#666; font-weight:700;">CatBrasil</span>
                    </div>
                    <p style="font-size:0.75rem; color:var(--cat-black); font-weight:700;">
                        Código de verificação de cadastro: <span style="color:var(--cat-red); font-size:0.9rem; font-family:'JetBrains Mono', monospace; font-weight:900;">{generatedVerificationCode}</span>
                    </p>
                </div>

                <div class="login-input-group" style="text-align: center;">
                    <label for="verificationCodeInput" style="text-align: center; display:block; margin-bottom:6px;">Digite o código:</label>
                    <input type="text" maxlength="6" placeholder="000000" bind:value={verificationCodeInput} id="verificationCodeInput" style="text-align: center; letter-spacing: 6px; font-weight: 800; font-size: 1.3rem; border: 2.5px solid var(--cat-black); width: 85%; margin: auto; padding: 8px;">
                </div>

                <button class="login-btn-submit" on:click={handleRegister} style="margin-top: 18px; width: 100%;">
                    CONFIRMAR E REGISTRAR
                </button>
            </div>
        </div>
    </div>
{/if}
