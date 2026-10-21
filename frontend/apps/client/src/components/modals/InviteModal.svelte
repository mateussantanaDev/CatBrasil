<script>
    export let show = false;
    export let user = {};
    export let referrals = [];
    export let onClose = () => {};
    export let onCopyLink = () => {};
</script>

{#if show}
    <div class="modal-overlay open">
        <div class="modal-card open" style="max-width: 400px;">
            <div class="modal-header">
                <h3>PROGRAMA DE AFILIADOS & CONVITES</h3>
                <button class="modal-close" on:click={onClose}>✕</button>
            </div>
            <div class="modal-body">
                <div style="background: var(--cat-black); color: var(--cat-yellow); padding: 10px; border-radius: 2px; text-align: center; margin-bottom: 10px;">
                    <strong style="font-family:'Oswald'; font-size: 0.95rem;">GANHE 5% SOBRE TODOS OS RENDIMENTOS</strong>
                    <p style="font-size: 0.65rem; color: #fff; margin-top: 2px;">Convide outros operadores e receba 5% de comissão direta.</p>
                </div>

                <div class="data-row"><span>Seu Link de Convite:</span><span class="mono-text">https://catbrasil.app/ref/{user.id || 'CB-458F'}</span></div>
                <button class="btn btn-primary" on:click={onCopyLink} style="margin-top: 8px;">
                    COPIAR LINK DE INDICAÇÃO
                </button>

                <h4 style="font-family:'Oswald'; font-size:0.85rem; margin-top:14px; margin-bottom:6px; color:var(--cat-black);">MEUS INDICADOS ({referrals.length}):</h4>
                <div style="max-height: 140px; overflow-y: auto;">
                    {#each referrals as r}
                        <div style="background: #fff; border: 1.5px solid var(--cat-black); padding: 6px 8px; margin-bottom: 4px; border-radius: 2px; display: flex; justify-content: space-between; align-items: center; font-size: 0.72rem;">
                            <span>{r.nome}</span>
                            <span class={r.status === 'ativo' ? 'status-ativo' : 'status-inativo'}>
                                {r.status.toUpperCase()}
                            </span>
                        </div>
                    {/each}
                </div>
            </div>
        </div>
    </div>
{/if}
