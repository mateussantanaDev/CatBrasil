<script>
    export let toasts = [];
    export let onRemoveToast = () => {};
</script>

<div class="toast-container">
    {#each toasts as t (t.id)}
        <button type="button" class="toast {t.type}" on:click={() => onRemoveToast(t.id)}>
            <span style="flex-grow: 1; text-align: left; font-weight: 600; line-height: 1.35;">{t.message}</span>
            <span class="toast-close">✕</span>
        </button>
    {/each}
</div>
