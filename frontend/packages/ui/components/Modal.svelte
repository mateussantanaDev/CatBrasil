<script>
  let {
    isOpen = false,
    title = '',
    onclose,
    children,
    hazardHeader = true,
    maxWidth = 'max-w-lg'
  } = $props();

  function handleBackdrop(e) {
    if (e.target === e.currentTarget && onclose) {
      onclose();
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Escape' && isOpen && onclose) {
      onclose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if isOpen}
  <div
    class="modal-backdrop"
    onclick={handleBackdrop}
    role="presentation"
  >
    <div class="modal-window {maxWidth}">
      {#if hazardHeader}
        <div class="hazard-stripes modal-hazard-bar"></div>
      {/if}
      
      <div class="modal-header">
        <h3 class="modal-title">{title}</h3>
        <button class="modal-close-btn" onclick={onclose} aria-label="Fechar modal">✕</button>
      </div>

      <div class="modal-body">
        {@render children?.()}
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(0, 0, 0, 0.85);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 16px;
  }

  .modal-window {
    width: 100%;
    max-height: 90vh;
    background: #181818;
    border: 3px solid var(--cat-black);
    box-shadow: var(--modal-shadow);
    display: flex;
    flex-direction: column;
    position: relative;
    overflow: hidden;
    animation: modalPop 0.15s ease-out;
  }

  .modal-hazard-bar {
    height: 8px;
    width: 100%;
  }

  .modal-header {
    background: #111111;
    padding: 14px 18px;
    border-bottom: 2px solid #282828;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .modal-title {
    color: var(--cat-yellow);
    font-size: 1.15rem;
    letter-spacing: 1px;
  }

  .modal-close-btn {
    background: #222;
    color: #fff;
    border: 1px solid #444;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .modal-close-btn:hover {
    background: var(--cat-red);
    border-color: var(--cat-red);
  }

  .modal-body {
    padding: 20px;
    overflow-y: auto;
  }

  @keyframes modalPop {
    from {
      transform: scale(0.95);
      opacity: 0;
    }
    to {
      transform: scale(1);
      opacity: 1;
    }
  }
</style>
