<script>
    import { gestaoService } from '../../services/gestaoService.js';

    export let onLoginSuccess = () => {};
    export let onCancel = () => {};
    export let onShowToast = (msg, type) => {};

    let erpUsername = '';
    let erpPassword = '';
    let erpPin = '';
    let erpTwoFactorCode = '';
    let showPassword = false;
    let isLoading = false;
    let errorMessage = '';

    async function handleErpLogin() {
        errorMessage = '';
        if (!erpUsername.trim() || !erpPassword.trim() || !erpPin.trim() || !/^\d{6}$/.test(erpTwoFactorCode.trim())) {
            errorMessage = 'INFORME USUÁRIO, SENHA, PIN E CÓDIGO 2FA DE 6 DÍGITOS';
            if (onShowToast) onShowToast(errorMessage, 'error');
            return;
        }

        isLoading = true;

        const res = await gestaoService.loginErp(erpUsername, erpPassword, erpPin, erpTwoFactorCode);
        isLoading = false;

        if (res && res.success) {
            if (onShowToast) onShowToast('Autenticação ERP aprovada com sucesso.', 'win');
            onLoginSuccess(res.user);
        } else {
            const err = (res && res.error) || 'CREDENCIAIS INVÁLIDAS. VERIFIQUE OS DADOS E TENTE NOVAMENTE.';
            errorMessage = err;
            if (onShowToast) onShowToast(errorMessage, 'error');
        }
    }

</script>

<div class="erp-login-shell">
    <div class="erp-login-card">
        
        <!-- Header Branding B2G -->
        <div class="erp-card-header">
            <div class="erp-brand-logo">CAT</div>
            <div class="erp-brand-text">
                <h1 class="erp-title">CATBRASIL · GESTÃO ERP</h1>
                <span class="erp-subtitle">CENTRAL DE LIQUIDEZ E CONTROLE DE OPERAÇÕES</span>
            </div>
        </div>

        <!-- Barra de Status de Segurança -->
        <div class="erp-status-banner">
            <span class="status-indicator"></span>
            <span class="status-label">AUTENTICAÇÃO INSTITUCIONAL B2G · SSL 256-BIT</span>
        </div>

        <!-- Alerta de Erro -->
        {#if errorMessage}
            <div class="erp-alert-box">
                <span>{errorMessage}</span>
            </div>
        {/if}

        <!-- Formulário de Login -->
        <form class="erp-form" on:submit|preventDefault={handleErpLogin}>
            <div class="erp-form-group">
                <label for="erp-user-input">USUÁRIO DO GESTOR</label>
                <input 
                    id="erp-user-input" 
                    type="text" 
                    placeholder="Digite o nome de usuário" 
                    bind:value={erpUsername} 
                    autocomplete="off"
                />
            </div>

            <div class="erp-form-group">
                <div class="label-row">
                    <label for="erp-pass-input">SENHA DE ACESSO</label>
                    <button 
                        type="button" 
                        class="toggle-pass-link" 
                        on:click={() => showPassword = !showPassword}
                    >
                        {showPassword ? '[ OCULTAR ]' : '[ EXIBIR ]'}
                    </button>
                </div>
                <input 
                    id="erp-pass-input" 
                    type={showPassword ? 'text' : 'password'} 
                    placeholder="Digite a senha de gestor" 
                    bind:value={erpPassword} 
                    autocomplete="current-password"
                />
            </div>

            <div class="erp-form-group">
                <label for="erp-pin-input">PIN DE SEGURANÇA</label>
                <input 
                    id="erp-pin-input" 
                    type="password" 
                    maxlength="64" 
                    placeholder="Digite o PIN administrativo" 
                    bind:value={erpPin} 
                />
            </div>

            <div class="erp-form-group">
                <label for="erp-2fa-input">CÓDIGO 2FA / AUTHENTICATOR</label>
                <input 
                    id="erp-2fa-input" 
                    type="text" 
                    maxlength="6" 
                    placeholder="000000" 
                    bind:value={erpTwoFactorCode} 
                    autocomplete="one-time-code"
                />
            </div>

            <!-- Botões de Ação -->
            <div class="erp-actions-group">
                <button 
                    type="submit" 
                    class="btn-erp-primary" 
                    disabled={isLoading}
                >
                    {#if isLoading}
                        PROCESSANDO AUTENTICAÇÃO...
                    {:else}
                        AUTENTICAR NO ERP DE GESTÃO
                    {/if}
                </button>

                {#if onCancel}
                    <button type="button" class="btn-erp-secondary" on:click={onCancel}>
                        VOLTAR À PLATAFORMA
                    </button>
                {/if}
            </div>
        </form>

        <!-- Rodapé do Card -->
        <div class="erp-card-footer">
            <span>CATBRASIL ERP v3.0 · MÓDULO DE GESTÃO INSTITUCIONAL</span>
        </div>
    </div>
</div>

<style>
    .erp-login-shell {
        position: fixed;
        inset: 0;
        z-index: 999999;
        background: #090d16;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 16px;
        font-family: 'Inter', system-ui, -apple-system, sans-serif;
        box-sizing: border-box;
    }

    .erp-login-card {
        width: 100%;
        max-width: 420px;
        background: #0f172a;
        border: 1px solid #1e293b;
        border-radius: 0;
        color: #f8fafc;
        display: flex;
        flex-direction: column;
        box-shadow: 0 20px 40px rgba(0, 0, 0, 0.6);
    }

    .erp-card-header {
        background: #020617;
        padding: 16px 20px;
        border-bottom: 2px solid #ffb200;
        display: flex;
        align-items: center;
        gap: 14px;
    }

    .erp-brand-logo {
        width: 36px;
        height: 36px;
        background: #ffb200;
        color: #000000;
        font-family: monospace;
        font-weight: 900;
        font-size: 0.95rem;
        display: flex;
        align-items: center;
        justify-content: center;
        border: 1px solid #000000;
        flex-shrink: 0;
        border-radius: 0;
    }

    .erp-brand-text {
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .erp-title {
        font-family: monospace;
        font-size: 0.9rem;
        font-weight: 700;
        color: #ffffff;
        letter-spacing: 0.5px;
        margin: 0;
        text-transform: uppercase;
        line-height: 1.2;
    }

    .erp-subtitle {
        font-size: 0.58rem;
        font-family: monospace;
        color: #ffb200;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .erp-status-banner {
        background: #020617;
        padding: 8px 20px;
        display: flex;
        align-items: center;
        gap: 8px;
        border-bottom: 1px solid #1e293b;
    }

    .status-indicator {
        width: 6px;
        height: 6px;
        background: #10b981;
        border-radius: 0;
    }

    .status-label {
        font-size: 0.58rem;
        font-family: monospace;
        color: #94a3b8;
        font-weight: 700;
        letter-spacing: 0.5px;
        text-transform: uppercase;
    }

    .erp-alert-box {
        margin: 16px 20px 0 20px;
        padding: 10px 12px;
        background: #450a0a;
        border: 1px solid #991b1b;
        color: #fca5a5;
        font-size: 0.68rem;
        font-family: monospace;
        font-weight: 700;
        border-radius: 0;
        text-transform: uppercase;
        line-height: 1.3;
    }

    .erp-form {
        padding: 20px;
        display: flex;
        flex-direction: column;
        gap: 16px;
    }

    .erp-form-group {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .label-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .erp-form-group label {
        font-size: 0.62rem;
        font-family: monospace;
        font-weight: 700;
        color: #94a3b8;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .toggle-pass-link {
        background: transparent;
        border: none;
        color: #64748b;
        font-family: monospace;
        font-size: 0.6rem;
        font-weight: 700;
        cursor: pointer;
        padding: 0;
    }

    .toggle-pass-link:hover {
        color: #ffb200;
    }

    .erp-form-group input {
        width: 100%;
        background: #020617;
        border: 1px solid #334155;
        border-radius: 0;
        padding: 10px 12px;
        color: #ffffff;
        font-family: monospace;
        font-size: 0.8rem;
        outline: none;
        box-sizing: border-box;
        transition: border-color 0.15s;
    }

    .erp-form-group input:focus {
        border-color: #ffb200;
    }

    .erp-actions-group {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-top: 6px;
    }

    .btn-erp-primary {
        background: #ffb200;
        color: #000000;
        border: 1px solid #000000;
        border-radius: 0;
        padding: 11px;
        font-family: monospace;
        font-size: 0.78rem;
        font-weight: 700;
        letter-spacing: 0.5px;
        cursor: pointer;
        text-transform: uppercase;
        transition: background-color 0.15s;
        width: 100%;
    }

    .btn-erp-primary:hover:not(:disabled) {
        background: #e6a000;
    }

    .btn-erp-primary:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .btn-erp-secondary {
        background: transparent;
        color: #94a3b8;
        border: 1px solid #334155;
        border-radius: 0;
        padding: 9px;
        font-family: monospace;
        font-size: 0.7rem;
        font-weight: 700;
        cursor: pointer;
        text-transform: uppercase;
        transition: all 0.15s;
        text-align: center;
        width: 100%;
    }

    .btn-erp-secondary:hover {
        background: #1e293b;
        color: #ffffff;
        border-color: #475569;
    }

    .erp-card-footer {
        background: #020617;
        border-top: 1px solid #1e293b;
        padding: 10px 20px;
        text-align: center;
        font-family: monospace;
        font-size: 0.58rem;
        color: #64748b;
        text-transform: uppercase;
    }
</style>
