<script>
    import { api } from '../services/api.js';
    import { cpfDigits, formatCpf, isValidCpf, isValidIdentityName, isValidEmail } from '../utils/identity.js';

    export let onLoginSuccess = (token, user) => {};
    export let onShowToast = (msg, type) => {};
    export let onOpenGestao = () => {};

    let authMode = 'login'; // 'login' ou 'register'
    let loginUsername = '';
    let loginPassword = '';

    // Estado do Registro
    let regName = '';
    let regLegalName = '';
    let regEmail = '';
    let regPhone = '';
    let regCpf = '';
    let regCpfError = '';
    let regPassword = '';
    let regConfirmPassword = '';
    let inviteCode = '';

    // Estado de Validação SMS
    let isSendingCode = false;
    let verificationCodeSent = false;
    let verificationCodeInput = '';
    let showSmsModal = false;
    let isSubmittingRegister = false;

    let loginTwoFactorCode = '';
    let show2FAField = false;

    async function handleLogin() {
        if (!loginUsername || !loginPassword) {
            onShowToast("Preencha usuário e senha!", "error");
            return;
        }

        try {
            const res = await api.login(loginUsername, loginPassword, loginTwoFactorCode);
            onShowToast(res.message || `Bem-vindo, ${res.user.username}!`, "win");
            onLoginSuccess(res.token, res.user);
            loginUsername = "";
            loginPassword = "";
            loginTwoFactorCode = "";
        } catch (e) {
            if (e.errorCode === 'TWO_FACTOR_REQUIRED' || e.errorCode === 'INVALID_2FA_CODE') {
                show2FAField = true;
            }
            onShowToast(e.message || "Erro no login", "error");
        }
    }

    function isValidPhone(phone) {
        const cleaned = phone.replace(/\D/g, '');
        return cleaned.length === 11 && cleaned.startsWith('9', 2);
    }

    function hasValidPayerContact() {
        if (!isValidIdentityName(regLegalName)) {
            onShowToast('Informe seu nome civil completo, como consta no CPF.', 'error');
            return false;
        }
        if (!isValidEmail(regEmail)) {
            onShowToast('Informe um email válido para o titular.', 'error');
            return false;
        }
        return true;
    }

    function handleCpfInput(event) {
        regCpf = formatCpf(event.currentTarget.value);
        regCpfError = '';
    }

    async function handleSendVerificationCode() {
        if (!regName || !regLegalName || !regEmail || !regPhone || !regCpf || !regPassword || !regConfirmPassword) {
            onShowToast("Preencha todos os campos obrigatórios!", "error");
            return;
        }

        if (!hasValidPayerContact()) return;

        if (!isValidCpf(regCpf)) {
            regCpfError = 'CPF inválido. Confira os 11 dígitos.';
            onShowToast(regCpfError, 'error');
            return;
        }

        if (regPassword !== regConfirmPassword) {
            onShowToast("As senhas não coincidem!", "error");
            return;
        }

        if (regPassword.length < 8) {
            onShowToast("A senha deve ter no mínimo 8 caracteres!", "error");
            return;
        }

        if (!isValidPhone(regPhone)) {
            onShowToast("Informe um celular brasileiro com DDD!", "error");
            return;
        }

        try {
            isSendingCode = true;
            onShowToast("Enviando SMS com código de verificação...", "");
            await api.sendSMS(regPhone);
            verificationCodeSent = true;
            showSmsModal = true;
            onShowToast("Código de verificação enviado via SMS!", "win");
        } catch (e) {
            onShowToast(e.message || "Erro ao solicitar envio de SMS", "error");
        } finally {
            isSendingCode = false;
        }
    }

    async function handleRegister() {
        if (isSubmittingRegister) return;
        if (!hasValidPayerContact()) {
            showSmsModal = false;
            return;
        }
        if (!isValidCpf(regCpf)) {
            regCpfError = 'CPF inválido. Confira os 11 dígitos.';
            showSmsModal = false;
            onShowToast(regCpfError, 'error');
            return;
        }
        if (!/^\d{6}$/.test(verificationCodeInput.trim())) {
            onShowToast("Digite o código SMS de 6 dígitos recebido no celular.", "error");
            return;
        }

        try {
            isSubmittingRegister = true;
            const res = await api.register(regName, regPassword, regPhone, inviteCode, verificationCodeInput, cpfDigits(regCpf), regLegalName, regEmail);
            showSmsModal = false;
            onShowToast(res.message || `Operador ${regName} cadastrado com sucesso!`, "win");
            onLoginSuccess(res.token, res.user);
        } catch (e) {
            if (/cpf/i.test(`${e.errorCode || ''} ${e.message || ''}`)) {
                regCpfError = e.message || 'CPF recusado. Confira os dados informados.';
                showSmsModal = false;
            }
            onShowToast(e.message || "Erro ao realizar cadastro", "error");
        } finally {
            isSubmittingRegister = false;
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
                <label for="username">Usuário:</label>
                <input type="text" placeholder="Digite seu usuário" bind:value={loginUsername} id="username">
            </div>

            <div class="login-input-group">
                <label for="password">Senha de Acesso:</label>
                <input type="password" placeholder="Digite sua senha" bind:value={loginPassword} id="password">
            </div>

            {#if show2FAField}
                <div class="login-input-group">
                    <label for="login-2fa" style="color: #b45309; font-weight: 800;">Código 2FA (Google Authenticator):</label>
                    <input 
                        type="text" 
                        maxlength="6" 
                        placeholder="000000" 
                        bind:value={loginTwoFactorCode} 
                        id="login-2fa" 
                        style="text-align: center; letter-spacing: 6px; font-weight: 800; font-size: 1.2rem; border: 2.5px solid var(--cat-black);"
                    />
                </div>
            {/if}

            <button class="login-btn-submit" on:click={handleLogin}>ENTRAR NO TURNO</button>
			<p style="font-size:0.75rem; color:#555; margin-top:14px;">
                Novo no pátio? 
                <button class="login-toggle-link" on:click={() => authMode = 'register'}>Criar nova conta</button>
			</p>
        {:else}
            <!-- FORMULÁRIO DE CADASTRO COM VALIDAÇÃO CELULAR SMS -->
            <div class="login-input-group">
                <label for="regName">Usuário do Operador (para login):</label>
                <input type="text" placeholder="Ex: operador_cat" bind:value={regName} id="regName" autocomplete="username">
            </div>

            <div class="login-input-group">
                <label for="regLegalName">Nome civil completo do titular:</label>
                <input type="text" placeholder="Como consta no CPF" bind:value={regLegalName} id="regLegalName" autocomplete="name">
            </div>

            <div class="login-input-group">
                <label for="regEmail">Email do titular:</label>
                <input type="email" placeholder="nome@exemplo.com" bind:value={regEmail} id="regEmail" autocomplete="email">
            </div>

            <div class="login-input-group">
                <label for="regPhone">Número de Celular (com DDD):</label>
                <input type="text" placeholder="Ex: 11999999999" bind:value={regPhone} id="regPhone">
            </div>

            <div class="login-input-group">
                <label for="regCpf">CPF do titular:</label>
                <input
                    id="regCpf"
                    type="text"
                    inputmode="numeric"
                    maxlength="14"
                    placeholder="000.000.000-00"
                    value={regCpf}
                    on:input={handleCpfInput}
                    aria-invalid={regCpfError ? 'true' : 'false'}
                    aria-describedby="regCpfHelp"
                    autocomplete="off"
                >
                <small id="regCpfHelp" style="font-size: 0.65rem; color: {regCpfError ? '#b91c1c' : '#555'};">
                    {regCpfError || 'Nome, email e CPF ficarão pendentes de revisão manual antes de operações reais.'}
                </small>
            </div>

            <div class="login-input-group">
                <label for="regPassword">Senha do Operador:</label>
                <input type="password" placeholder="Mínimo 8 caracteres" bind:value={regPassword} id="regPassword">
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
                
                <div class="login-input-group" style="text-align: center;">
                    <label for="verificationCodeInput" style="text-align: center; display:block; margin-bottom:6px;">Digite o código:</label>
                    <input type="text" maxlength="6" placeholder="000000" bind:value={verificationCodeInput} id="verificationCodeInput" style="text-align: center; letter-spacing: 6px; font-weight: 800; font-size: 1.3rem; border: 2.5px solid var(--cat-black); width: 85%; margin: auto; padding: 8px;">
                </div>

                <button class="login-btn-submit" on:click={handleRegister} disabled={isSubmittingRegister} style="margin-top: 18px; width: 100%;">
                    {isSubmittingRegister ? 'CADASTRANDO...' : 'CONFIRMAR E REGISTRAR'}
                </button>
            </div>
        </div>
    </div>
{/if}
