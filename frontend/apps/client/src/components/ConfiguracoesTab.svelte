<script>
    import { cpfDigits, formatCpf, isValidCpf, isValidIdentityName, isValidEmail } from '../utils/identity.js';

    export let user = {};
    export let referrals = [];
    export let transactions = [];
    export let submenuOpen = {};
    export let txFilter = 'todos';
    export let inputPassword = '';
    export let inputCurrentPassword = '';
    export let inputAccountPassword = '';
    export let inputTwoFactorCode = '';

    export let onToggleSubmenu = () => {};
    export let onSaveSecurityPassword = () => {};
    export let onOpen2FA = () => {};
    export let onCopyInviteLink = () => {};
    export let onLevelUp = () => {};
    export let onLogout = () => {};
    export let onSetTxFilter = () => {};
    export let onSimulateReferral = () => {};
    export let onResetRoulette = () => {};
    export let onFullReset = () => {};
    export let onSubmitIdentity = async () => false;

    let identityCpf = '';
    let identityLegalName = '';
    let identityEmail = '';
    let identityError = '';
    let identitySubmitting = false;

    function handleIdentityCpfInput(event) {
        identityCpf = formatCpf(event.currentTarget.value);
        identityError = '';
    }

    async function handleIdentitySubmit(event) {
        event.preventDefault();
        if (identitySubmitting) return;
        if (!isValidCpf(identityCpf)) {
            identityError = 'CPF inválido. Confira os 11 dígitos.';
            return;
        }
        if (!isValidIdentityName(identityLegalName)) {
            identityError = 'Informe seu nome civil completo, como consta no CPF.';
            return;
        }
        if (!isValidEmail(identityEmail)) {
            identityError = 'Informe um email válido para o titular.';
            return;
        }

        identityError = '';
        identitySubmitting = true;
        try {
            const submitted = await onSubmitIdentity({
                cpf: cpfDigits(identityCpf),
                legalName: identityLegalName.trim().replace(/\s+/g, ' '),
                email: identityEmail.trim().toLowerCase()
            });
            if (submitted) {
                identityCpf = '';
                identityLegalName = '';
                identityEmail = '';
            }
        } catch (error) {
            identityError = error.message || 'Não foi possível enviar os dados. Tente novamente.';
        } finally {
            identitySubmitting = false;
        }
    }

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

    function documentStatus(status) {
        switch (status) {
            case 'verified': return { label: 'Documento conferido', color: '#a5d6a7', background: '#1b5e20', border: '#4caf50' };
            case 'pending': return { label: 'Documento pendente de conferência', color: '#ffecb3', background: '#6b4f00', border: '#fbbf24' };
            case 'rejected': return { label: 'Documento recusado', color: '#ffcdd2', background: '#7f1d1d', border: '#ef4444' };
            case 'missing': return { label: 'Documento não informado', color: '#ddd', background: '#444', border: '#888' };
            default: return { label: 'Situação do documento indisponível', color: '#ddd', background: '#444', border: '#888' };
        }
    }
</script>

<div class="tab-content active" style="padding-bottom: 50px; width: 100%; box-sizing: border-box;">
    
    <!-- Header do Perfil do Operador (High-Trust & Clean) -->
    <div style="background: var(--cat-black); color: var(--cat-white); padding: 14px 16px; border: var(--brutal-border); border-radius: 4px; box-shadow: var(--brutal-shadow); margin-bottom: 12px; width: 100%; box-sizing: border-box;">
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap;">
            <div style="display: flex; align-items: center; gap: 10px; min-width: 0;">
                <div style="width: 44px; height: 44px; border-radius: 50%; background: var(--cat-yellow); border: 2px solid var(--cat-white); display: flex; align-items: center; justify-content: center; font-family:'Oswald'; font-weight: 900; font-size: 1.1rem; flex-shrink: 0; color: #000;">CAT</div>
                <div style="min-width: 0;">
                    <h3 style="font-size: 1.15rem; color: var(--cat-yellow); font-family:'Oswald'; margin: 0; line-height: 1.1; word-break: break-word;">{user.username ? user.username.toUpperCase() : 'OPERADOR'}</h3>
                    <span style="font-size: 0.58rem; color: #bbb; text-transform: uppercase; font-weight: 700;">OPERADOR · NÍVEL {user.nivel}</span>
                </div>
            </div>
            <div style="display: flex; gap: 6px; align-items: center; flex-wrap: wrap;">
                <span style="font-size: 0.58rem; background: {documentStatus(user.identityStatus).background}; border: 1px solid {documentStatus(user.identityStatus).border}; color: {documentStatus(user.identityStatus).color}; padding: 2px 6px; font-weight: 800; border-radius: 2px;">{documentStatus(user.identityStatus).label}</span>
                <button on:click={onCopyInviteLink} style="font-size: 0.58rem; background: var(--cat-gray); border: 1px solid var(--cat-yellow); color: var(--cat-yellow); padding: 2px 6px; font-weight: 800; border-radius: 2px; cursor: pointer; white-space: nowrap;">ID #{user.id}</button>
            </div>
        </div>
    </div>

    <!-- Conferência de identidade para contas antigas e documentos recusados -->
    <section style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 12px; padding: 14px; width: 100%; box-sizing: border-box;">
        <h3 style="font-family:'Oswald'; font-size: 1rem; margin: 0 0 6px;">Identidade do titular</h3>
        {#if user.identityStatus === 'verified'}
            <p style="font-size: 0.75rem; margin: 0;">Documento conferido. Cada operação ainda está sujeita às regras financeiras aplicáveis.</p>
        {:else if user.identityStatus === 'pending'}
            <p style="font-size: 0.75rem; margin: 0;">Documento pendente de conferência manual. Operações reais ficam aguardando a decisão da equipe.</p>
        {:else if user.identityStatus === 'missing' || user.identityStatus === 'rejected'}
            <p style="font-size: 0.75rem; margin: 0 0 12px;">
                {user.identityStatus === 'rejected'
                    ? 'Os dados anteriores foram recusados. Revise as informações e envie novamente para conferência manual.'
                    : 'Informe os dados do titular para iniciar a conferência manual antes de operações reais.'}
            </p>
            <form on:submit={handleIdentitySubmit} style="display: grid; gap: 8px;">
                <label for="identity-legal-name" style="font-size: 0.7rem; font-weight: 800;">Nome civil completo</label>
                <input id="identity-legal-name" type="text" autocomplete="name" bind:value={identityLegalName} placeholder="Como consta no CPF" class="input-field" style="width:100%; box-sizing:border-box; padding:8px;">
                <label for="identity-email" style="font-size: 0.7rem; font-weight: 800;">Email do titular</label>
                <input id="identity-email" type="email" autocomplete="email" bind:value={identityEmail} placeholder="nome@exemplo.com" class="input-field" style="width:100%; box-sizing:border-box; padding:8px;">
                <label for="identity-cpf" style="font-size: 0.7rem; font-weight: 800;">CPF do titular</label>
                <input id="identity-cpf" type="text" inputmode="numeric" maxlength="14" autocomplete="off" value={identityCpf} on:input={handleIdentityCpfInput} placeholder="000.000.000-00" aria-invalid={identityError ? 'true' : 'false'} aria-describedby="identity-error" class="input-field" style="width:100%; box-sizing:border-box; padding:8px;">
                {#if identityError}
                    <p id="identity-error" role="alert" style="font-size: 0.7rem; color: #b91c1c; margin: 0;">{identityError}</p>
                {/if}
                <button type="submit" class="btn btn-primary" disabled={identitySubmitting} style="padding:10px; width:100%;">
                    {identitySubmitting ? 'ENVIANDO...' : 'ENVIAR PARA CONFERÊNCIA'}
                </button>
                <small style="font-size: 0.65rem; color: #555;">O envio inicia uma revisão manual; o formato válido do CPF não confirma a identidade.</small>
            </form>
        {:else}
            <p style="font-size: 0.75rem; margin: 0;">Não foi possível consultar o estado do documento. Atualize a página e tente novamente.</p>
        {/if}
    </section>

    <!-- Resumo Financeiro da Conta (2 Colunas Compactas) -->
    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px; width: 100%; box-sizing: border-box;">
        <div style="background: #fff; border: 2px solid var(--cat-black); padding: 10px; border-radius: 4px; box-shadow: 2.5px 2.5px 0px var(--cat-black); min-width: 0;">
            <span style="font-size: 0.58rem; color: #666; text-transform: uppercase; font-weight: 800; display: block;">Saldo Livre</span>
            <strong class="mono-text" style="font-size: 0.88rem; color: var(--cat-green); display: block; margin-top: 1px; word-break: break-word;">R$ {user.balance ? user.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</strong>
        </div>
        <div style="background: #fff; border: 2px solid var(--cat-black); padding: 10px; border-radius: 4px; box-shadow: 2.5px 2.5px 0px var(--cat-black); min-width: 0;">
            <span style="font-size: 0.58rem; color: #666; text-transform: uppercase; font-weight: 800; display: block;">Bônus Nível</span>
            <strong class="mono-text highlight-yellow" style="font-size: 0.88rem; display: block; margin-top: 1px; word-break: break-word;">+{((user.nivel - 1) * 5).toFixed(1)}% Extra</strong>
        </div>
    </div>

    <!-- SMART ACCORDION 1: SEGURANÇA & SENHA FINANCEIRA -->
    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 10px; width: 100%; box-sizing: border-box;">
        <button 
            on:click={() => onToggleSubmenu('conta')}
            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
        >
            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
                <strong style="font-family:'Oswald'; font-size: 0.95rem; color: var(--cat-black); word-break: break-word;">Segurança & Senha de Saque</strong>
            </div>
            <div style="display: flex; align-items: center; gap: 8px; margin-left: auto;">
                <span style="font-size: 0.58rem; background: #E8F5E9; border: 1px solid #2e7d32; color: #2e7d32; padding: 2px 6px; font-weight: 800; border-radius: 2px; white-space: nowrap;">
                    {user.hasSecurityPassword ? 'DEFINIDA' : 'PENDENTE'}
                </span>
                <span style="font-size: 0.8rem; color: #555; flex-shrink: 0;">{submenuOpen.conta ? '▲' : '▼'}</span>
            </div>
        </button>

        {#if submenuOpen.conta}
            <div style="padding: 14px; border-top: 1.5px dashed var(--cat-black); background: #fafafa; width: 100%; box-sizing: border-box;">
                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                    <span>Status de Retiradas:</span>
                    <span style="font-weight: 800;">{user.withdrawLocked ? 'BLOQUEADO' : 'SUJEITO A VALIDAÇÃO FINANCEIRA'}</span>
                </div>
                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                    <span>Autenticação 2FA:</span>
                    <span class="mono-text">{user.twoFactorEnabled ? 'ATIVA' : 'PENDENTE'}</span>
                </div>
                <div class="data-row" style="font-size: 0.75rem; padding: 6px 0; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px;">
                    <span>Senha de Retirada:</span>
                    <span class={user.hasSecurityPassword ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                        {user.hasSecurityPassword ? 'DEFINIDA' : 'PENDENTE DE CADASTRO'}
                    </span>
                </div>

                <div style="margin-top: 12px; background: #fff; border: 1.5px solid var(--cat-black); padding: 12px; border-radius: 4px; box-shadow: 2px 2px 0px rgba(0,0,0,0.1); width: 100%; box-sizing: border-box;">
                    <label class="input-field-label" for="sec-pass-input" style="font-size: 0.68rem; display: block; margin-bottom: 4px;">Cadastrar / Alterar Senha de Saque (6 dígitos):</label>
                    {#if user.hasSecurityPassword}
                        <label class="input-field-label" for="sec-pass-current" style="font-size: 0.68rem; display: block; margin-bottom: 4px;">PIN atual:</label>
                        <input id="sec-pass-current" type="password" inputmode="numeric" maxlength="6" bind:value={inputCurrentPassword} class="input-field" style="width:100%; box-sizing:border-box; margin-bottom:8px; padding:8px;">
                    {/if}
                    <label class="input-field-label" for="sec-pass-account" style="font-size: 0.68rem; display: block; margin-bottom: 4px;">Senha da conta:</label>
                    <input id="sec-pass-account" type="password" autocomplete="current-password" bind:value={inputAccountPassword} class="input-field" style="width:100%; box-sizing:border-box; margin-bottom:8px; padding:8px;">
                    <input id="sec-pass-input" type="password" maxlength="6" placeholder="******" class="input-field" bind:value={inputPassword} style="letter-spacing: 6px; font-weight: 800; font-size: 1.1rem; text-align: center; width: 100%; box-sizing: border-box; padding: 8px; border: 2px solid var(--cat-black); border-radius: 2px;">
                    {#if user.twoFactorEnabled}
                        <label class="input-field-label" for="sec-pass-2fa" style="font-size: 0.68rem; display: block; margin:8px 0 4px;">Código 2FA:</label>
                        <input id="sec-pass-2fa" type="text" inputmode="numeric" maxlength="6" bind:value={inputTwoFactorCode} class="input-field" style="width:100%; box-sizing:border-box; padding:8px;">
                    {/if}
                    <button class="btn btn-primary" on:click={onSaveSecurityPassword} style="margin-top: 10px; font-size: 0.8rem; padding: 10px; width: 100%; box-sizing: border-box;">
                        SALVAR SENHA FINANCEIRA
                    </button>
                </div>
                <button class="btn btn-primary" on:click={onOpen2FA} disabled={user.twoFactorEnabled} style="margin-top:10px; padding:10px; width:100%;">
                    {user.twoFactorEnabled ? '2FA ATIVO' : 'ATIVAR AUTENTICAÇÃO 2FA'}
                </button>
            </div>
        {/if}
    </div>

    <!-- SMART ACCORDION 2: PROGRESSO DE CARREIRA -->
    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 10px; width: 100%; box-sizing: border-box;">
        <button 
            on:click={() => onToggleSubmenu('niveis')}
            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
        >
            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
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
                    MISSÕES PARA ALCANÇAR NÍVEL {user.nivel + 1}:
                </h5>

                <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 6px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 0.75rem;">
                    <span>1. Coletas na Garagem</span>
                    <strong class={user.coletasCompleted >= 5 ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                        {user.coletasCompleted} / 5 {user.coletasCompleted >= 5 ? 'OK' : 'PENDENTE'}
                    </strong>
                </div>

                <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 6px; border-radius: 2px; font-size: 0.75rem;">
                    <div style="display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap;">
                        <span>2. Contrato Pesado</span>
                        <strong class={user.tratorRented ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                            {user.tratorRented ? '1 / 1 OK' : '0 / 1 PENDENTE'}
                        </strong>
                    </div>
                    <p style="font-size: 0.62rem; color: #666; margin-top: 4px; line-height: 1.3;">Requisito: Alugue o Trator D8T na Loja de máquinas.</p>
                </div>

                <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 8px 10px; margin-bottom: 8px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 0.75rem;">
                    <span>3. Qualificação de Rede</span>
                    <strong class={referrals.filter(r => r.status === 'ativo').length === 3 ? 'highlight-green' : 'highlight-red'} style="font-weight: 800;">
                        {referrals.filter(r => r.status === 'ativo').length} / 3 {referrals.filter(r => r.status === 'ativo').length === 3 ? 'OK' : 'PENDENTE'}
                    </strong>
                </div>

                <button 
                    class="btn" 
                    disabled={!(user.coletasCompleted >= 5 && user.tratorRented && referrals.filter(r => r.status === 'ativo').length === 3)} 
                    on:click={onLevelUp}
                    style="margin-top: 8px; padding: 10px; font-size: 0.8rem; width: 100%; box-sizing: border-box; background: {user.coletasCompleted >= 5 && user.tratorRented && referrals.filter(r => r.status === 'ativo').length === 3 ? 'var(--cat-yellow)' : 'var(--cat-light-gray)'};"
                >
                    {user.coletasCompleted >= 5 && user.tratorRented && referrals.filter(r => r.status === 'ativo').length === 3 ? 'PROMOVER CARREIRA DE OPERADOR' : 'MISSÕES PENDENTES'}
                </button>
            </div>
        {/if}
    </div>

    <!-- SMART ACCORDION 3: REDE DE AFILIADOS -->
    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 10px; width: 100%; box-sizing: border-box;">
        <button 
            id="btn-accordion-convidados"
            on:click={() => onToggleSubmenu('convidados')}
            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
        >
            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
                <strong style="font-family:'Oswald'; font-size: 0.95rem; color: var(--cat-black); word-break: break-word;">Rede & Indicação (5%)</strong>
            </div>
            <div style="display: flex; align-items: center; gap: 8px; margin-left: auto;">
                <span style="font-size: 0.58rem; background: {user.referralBonusSuspended ? '#FFEBEE' : '#E8F5E9'}; border: 1px solid {user.referralBonusSuspended ? '#d32f2f' : '#2e7d32'}; color: {user.referralBonusSuspended ? '#d32f2f' : '#2e7d32'}; padding: 2px 6px; font-weight: 800; border-radius: 2px; white-space: nowrap;">
                    {user.referralBonusSuspended ? 'SUSPENSO' : '5% ATIVO'}
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
                                    {c.status.toUpperCase()}
                                </span>
                            </div>
                        </div>
                    {/each}
                </div>

                <button class="btn btn-primary" on:click={onCopyInviteLink} style="margin-top: 10px; font-size: 0.8rem; padding: 10px; width: 100%; box-sizing: border-box;">
                    COPIAR LINK DE CONVITE
                </button>
            </div>
        {/if}
    </div>

    <!-- SMART ACCORDION 4: HISTÓRICO FINANCEIRO -->
    <div style="background: #fff; border: 2.5px solid var(--cat-black); box-shadow: 3px 3px 0px var(--cat-black); border-radius: 4px; margin-bottom: 14px; width: 100%; box-sizing: border-box;">
        <button 
            id="btn-accordion-historico"
            on:click={() => onToggleSubmenu('historico')}
            style="width: 100%; background: #fff; border: none; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; cursor: pointer; text-align: left; box-sizing: border-box;"
        >
            <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
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
                        id="btn-filter-todos"
                        on:click={() => onSetTxFilter('todos')}
                        style="padding: 6px 10px; font-size: 0.65rem; margin: 0; flex: 1 1 auto; border-color: var(--cat-black); background: {txFilter === 'todos' ? 'var(--cat-yellow)' : '#eee'}; color: var(--cat-black); font-weight: 800; white-space: nowrap;"
                    >
                        Todos ({transactions.length})
                    </button>
                    <button 
                        class="btn" 
                        id="btn-filter-depositos"
                        on:click={() => onSetTxFilter('depositos')}
                        style="padding: 6px 10px; font-size: 0.65rem; margin: 0; flex: 1 1 auto; border-color: var(--cat-black); background: {txFilter === 'depositos' ? 'var(--cat-yellow)' : '#eee'}; color: var(--cat-black); font-weight: 800; white-space: nowrap;"
                    >
                        Depósitos
                    </button>
                    <button 
                        class="btn" 
                        id="btn-filter-saques"
                        on:click={() => onSetTxFilter('saques')}
                        style="padding: 6px 10px; font-size: 0.65rem; margin: 0; flex: 1 1 auto; border-color: var(--cat-black); background: {txFilter === 'saques' ? 'var(--cat-yellow)' : '#eee'}; color: var(--cat-black); font-weight: 800; white-space: nowrap;"
                    >
                        Saques
                    </button>
                    <button 
                        class="btn" 
                        id="btn-filter-coletas"
                        on:click={() => onSetTxFilter('rendimentos')}
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
    <button class="btn btn-primary" on:click={onLogout} style="background: var(--cat-black); color: var(--cat-yellow); border-color: var(--cat-black); width: 100%; box-sizing: border-box; font-family: 'Oswald', sans-serif; padding: 12px; font-size: 0.9rem;">
        FINALIZAR TURNO (LOGOUT)
    </button>

    {#if import.meta.env.DEV}
    <!-- Área de Testes QA Discreta -->
    <div style="margin-top: 12px; text-align: center; width: 100%; box-sizing: border-box;">
        <button 
            on:click={() => onToggleSubmenu('eventos')}
            style="background: transparent; border: none; font-size: 0.62rem; color: #999; text-decoration: underline; cursor: pointer;"
        >
            {submenuOpen.eventos ? '▲ Ocultar Testes' : '▼ Opções de Teste QA'}
        </button>
        
        {#if submenuOpen.eventos}
            <div style="margin-top: 6px; background: #F5F5F5; border: 1px solid #ccc; padding: 8px; border-radius: 2px; text-align: left; width: 100%; box-sizing: border-box;">
                <div style="display: flex; gap: 4px; margin-bottom: 4px; flex-wrap: wrap;">
                    <button class="btn" on:click={() => onSimulateReferral(false)} style="font-size:0.58rem; padding: 4px; margin:0; flex:1; background:#ef5350; color:white;">Inativar Maria</button>
                    <button class="btn" on:click={() => onSimulateReferral(true)} style="font-size:0.58rem; padding: 4px; margin:0; flex:1; background:#66bb6a; color:white;">Ativar Todos</button>
                </div>
                <button class="btn" on:click={onResetRoulette} style="font-size:0.58rem; padding: 4px; margin:0 0 4px 0; background:var(--cat-gray); color:white; width:100%;">Reset Roleta</button>
                <button class="btn" on:click={onFullReset} style="font-size:0.58rem; padding: 4px; margin:0; background:var(--cat-red); color:white; width:100%;">Reset Full</button>
            </div>
        {/if}
    </div>
    {/if}

</div>
