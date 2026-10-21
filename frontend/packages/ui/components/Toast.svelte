<script>
  let {
    toasts = [],
    onremove
  } = $props();
</script>

<div class="toast-container">
  {#each toasts as t (t.id)}
    <div class="toast-card toast-{t.type}">
      <div class="toast-icon">
        {#if t.type === 'success'}✅
        {:else if t.type === 'error'}⚠️
        {:else if t.type === 'warning'}⚡
        {:else}ℹ️{/if}
      </div>
      <div class="toast-message">{t.message}</div>
      <button class="toast-close" onclick={() => onremove && onremove(t.id)}>✕</button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    top: 20px;
    right: 20px;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-width: 360px;
    width: calc(100vw - 40px);
    pointer-events: none;
  }

  .toast-card {
    pointer-events: auto;
    background: #1e1e1e;
    color: #fff;
    border: 2px solid var(--cat-black);
    box-shadow: 4px 4px 0px 0px rgba(0,0,0,0.8);
    padding: 12px 14px;
    display: flex;
    align-items: center;
    gap: 10px;
    animation: toastSlideIn 0.2s ease-out;
  }

  .toast-success {
    border-left: 6px solid var(--cat-green);
  }

  .toast-error {
    border-left: 6px solid var(--cat-red);
  }

  .toast-warning {
    border-left: 6px solid var(--cat-yellow);
  }

  .toast-info {
    border-left: 6px solid var(--cat-blue);
  }

  .toast-icon {
    font-size: 1.1rem;
  }

  .toast-message {
    flex: 1;
    font-size: 0.85rem;
    line-height: 1.3;
    font-weight: 500;
  }

  .toast-close {
    background: transparent;
    border: none;
    color: #888;
    cursor: pointer;
    font-size: 0.8rem;
    padding: 4px;
  }

  .toast-close:hover {
    color: #fff;
  }

  @keyframes toastSlideIn {
    from {
      transform: translateX(100%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }
</style>
