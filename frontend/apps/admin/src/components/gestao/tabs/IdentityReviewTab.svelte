<script>
    import { onMount, onDestroy } from 'svelte';
    import { api } from '../../../services/api.js';

    export let onForbidden = () => {};

    let pendingIds = [];
    let selectedId = '';
    let inspectedPerson = null;
    let inspectCode = '';
    let decisionCode = '';
    let decision = '';
    let evidenceRef = '';
    let errorMessage = '';
    let successMessage = '';
    let queueLoading = false;
    let inspectLoading = false;
    let decisionLoading = false;
    let active = true;

    function clearReview() {
        inspectedPerson = null;
        selectedId = '';
        inspectCode = '';
        decisionCode = '';
        decision = '';
        evidenceRef = '';
        errorMessage = '';
    }

    function handleVisibilityChange() {
        if (document.hidden) clearReview();
    }

    function handleError(error) {
        if (error.errorCode === 'IDENTITY_REVIEW_FORBIDDEN') {
            clearReview();
            pendingIds = [];
            onForbidden();
            return;
        }
        errorMessage = error.message || 'Não foi possível concluir a revisão.';
    }

    async function loadPending() {
        if (!active) return;
        queueLoading = true;
        errorMessage = '';
        try {
            const response = await api.get('/admin/identity/pending');
            if (active) pendingIds = Array.isArray(response.userIds) ? response.userIds : [];
        } catch (error) {
            if (active) handleError(error);
        } finally {
            if (active) queueLoading = false;
        }
    }

    function beginReview(userId) {
        clearReview();
        successMessage = '';
        selectedId = userId;
    }

    async function inspectIdentity() {
        if (!selectedId || inspectLoading || !/^\d{6}$/.test(inspectCode.trim())) {
            errorMessage = 'Informe o código 2FA de 6 dígitos para consultar os dados.';
            return;
        }
        errorMessage = '';
        inspectLoading = true;
        const userId = selectedId;
        try {
            const person = await api.post(`/admin/users/${encodeURIComponent(userId)}/identity/inspect`, {
                twoFactorCode: inspectCode.trim()
            });
            if (!active || selectedId !== userId) return;
            if (person.identityStatus !== 'pending') {
                clearReview();
                errorMessage = 'Este documento não está mais pendente de revisão.';
                await loadPending();
                return;
            }
            inspectedPerson = {
                cpf: person.cpf,
                legalName: person.legalName,
                email: person.email
            };
        } catch (error) {
            if (active) {
                inspectedPerson = null;
                handleError(error);
            }
        } finally {
            inspectCode = '';
            if (active) inspectLoading = false;
        }
    }

    async function submitDecision() {
        if (!inspectedPerson || !selectedId || decisionLoading) return;
        if (!['verified', 'rejected'].includes(decision)) {
            errorMessage = 'Selecione aprovar ou recusar após revisar os dados.';
            return;
        }
        if (evidenceRef.trim().length < 5) {
            errorMessage = 'Informe a referência da evidência usada na decisão.';
            return;
        }
        if (!/^\d{6}$/.test(decisionCode.trim())) {
            errorMessage = 'Informe o código 2FA de 6 dígitos para registrar a decisão.';
            return;
        }

        errorMessage = '';
        decisionLoading = true;
        const userId = selectedId;
        const chosenDecision = decision;
        try {
            await api.patch(`/admin/users/${encodeURIComponent(userId)}/identity`, {
                status: chosenDecision,
                evidenceRef: evidenceRef.trim(),
                twoFactorCode: decisionCode.trim()
            });
            if (!active) return;
            clearReview();
            successMessage = chosenDecision === 'verified'
                ? 'Documento aprovado e decisão registrada.'
                : 'Documento recusado e decisão registrada.';
            await loadPending();
        } catch (error) {
            if (active) handleError(error);
        } finally {
            decisionCode = '';
            if (active) decisionLoading = false;
        }
    }

    onMount(() => {
        document.addEventListener('visibilitychange', handleVisibilityChange);
        loadPending();
    });

    onDestroy(() => {
        active = false;
        document.removeEventListener('visibilitychange', handleVisibilityChange);
        clearReview();
        pendingIds = [];
    });
</script>

<section style="background:#fff; border:1px solid #cbd5e1; border-radius:4px; padding:20px; max-width:900px; width:100%; box-sizing:border-box;">
    <header style="display:flex; align-items:center; justify-content:space-between; gap:12px; flex-wrap:wrap;">
        <div>
            <h3 style="font-family:monospace; margin:0; font-size:1.1rem;">Conferência manual de identidade</h3>
            <p style="font-size:0.75rem; color:#475569; margin:6px 0 0;">Apenas diretoria. Cada consulta e decisão exige código 2FA.</p>
        </div>
        <button type="button" on:click={loadPending} disabled={queueLoading} style="padding:8px 12px; cursor:pointer;">
            {queueLoading ? 'ATUALIZANDO...' : 'ATUALIZAR FILA'}
        </button>
    </header>

    {#if errorMessage}
        <p role="alert" style="background:#fef2f2; border:1px solid #ef4444; color:#991b1b; padding:8px; font-size:0.75rem;">{errorMessage}</p>
    {/if}
    {#if successMessage}
        <p role="status" style="background:#ecfdf5; border:1px solid #10b981; color:#065f46; padding:8px; font-size:0.75rem;">{successMessage}</p>
    {/if}

    <h4 style="font-family:monospace; font-size:0.85rem; margin:20px 0 8px;">PENDÊNCIAS ({pendingIds.length})</h4>
    {#if pendingIds.length === 0 && !queueLoading}
        <p style="font-size:0.75rem; color:#475569;">Nenhum documento pendente.</p>
    {:else}
        <div style="display:flex; flex-wrap:wrap; gap:8px;">
            {#each pendingIds as userId (userId)}
                <button type="button" on:click={() => beginReview(userId)} style="padding:8px; cursor:pointer; border:1px solid #64748b; background:{selectedId === userId ? '#ffb200' : '#f8fafc'}; font-family:monospace; font-size:0.72rem;">
                    ID {userId}
                </button>
            {/each}
        </div>
    {/if}

    {#if selectedId}
        <div style="margin-top:18px; border-top:1px solid #cbd5e1; padding-top:16px;">
            <div style="display:flex; justify-content:space-between; align-items:center; gap:8px;">
                <strong style="font-size:0.8rem;">Revisão do operador {selectedId}</strong>
                <button type="button" on:click={clearReview} style="cursor:pointer; padding:6px 10px;">FECHAR REVISÃO</button>
            </div>

            {#if !inspectedPerson}
                <form on:submit|preventDefault={inspectIdentity} style="display:grid; gap:8px; margin-top:12px; max-width:360px;">
                    <label for="identity-inspect-code" style="font-size:0.75rem;">Código 2FA para consultar os dados</label>
                    <input id="identity-inspect-code" type="password" inputmode="numeric" maxlength="6" autocomplete="one-time-code" bind:value={inspectCode} placeholder="6 dígitos" style="padding:8px;">
                    <button type="submit" disabled={inspectLoading} style="padding:10px; cursor:pointer; background:#ffb200; font-weight:800;">{inspectLoading ? 'CONSULTANDO...' : 'CONSULTAR IDENTIDADE'}</button>
                </form>
            {:else}
                <div style="background:#f8fafc; border:1px solid #cbd5e1; padding:12px; margin-top:12px; font-size:0.78rem; line-height:1.8;">
                    <div><strong>Nome civil:</strong> {inspectedPerson.legalName}</div>
                    <div><strong>Email:</strong> {inspectedPerson.email}</div>
                    <div><strong>CPF:</strong> {inspectedPerson.cpf}</div>
                </div>
                <form on:submit|preventDefault={submitDecision} style="display:grid; gap:10px; margin-top:14px; max-width:520px;">
                    <fieldset style="border:1px solid #cbd5e1; display:flex; gap:18px; flex-wrap:wrap; font-size:0.8rem;">
                        <legend>Decisão manual</legend>
                        <label><input type="radio" bind:group={decision} value="verified"> Aprovar</label>
                        <label><input type="radio" bind:group={decision} value="rejected"> Recusar</label>
                    </fieldset>
                    <label for="identity-evidence" style="font-size:0.75rem;">Referência da evidência ou protocolo interno</label>
                    <input id="identity-evidence" type="text" maxlength="200" bind:value={evidenceRef} placeholder="Ex.: protocolo de conferência" style="padding:8px;">
                    <small style="font-size:0.68rem; color:#475569;">Não inclua CPF ou outros dados pessoais na referência.</small>
                    <label for="identity-decision-code" style="font-size:0.75rem;">Código 2FA para registrar a decisão</label>
                    <input id="identity-decision-code" type="password" inputmode="numeric" maxlength="6" autocomplete="one-time-code" bind:value={decisionCode} placeholder="6 dígitos" style="padding:8px;">
                    <button type="submit" disabled={decisionLoading || !decision} style="padding:10px; cursor:pointer; background:#ffb200; font-weight:800;">
                        {decisionLoading ? 'REGISTRANDO...' : 'CONFIRMAR DECISÃO'}
                    </button>
                </form>
            {/if}
        </div>
    {/if}
</section>
