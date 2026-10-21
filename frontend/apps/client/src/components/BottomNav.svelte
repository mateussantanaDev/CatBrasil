<script>
    export let currentTab = 'garagem';
    export let onSelectTab = () => {};
    export let garageCount = 0;
</script>

<nav class="bottom-nav">
    <!-- GARAGEM (Solid Heavy Vehicle Icon) -->
    <button 
        class="nav-item {currentTab === 'garagem' ? 'active' : ''}" 
        on:click={() => onSelectTab('garagem')}
    >
        <div class="nav-icon" style="display: flex; align-items: center; justify-content: center;">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                <path d="M19 8h-2V5c0-1.1-.9-2-2-2H9c-1.1 0-2 .9-2 2v3H5c-1.1 0-2 .9-2 2v6c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2v-6c0-1.1-.9-2-2-2zM9 5h6v3H9V5zm-2 15c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zm10 0c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2z"/>
            </svg>
        </div>
        <span>GARAGEM</span>
        {#if garageCount > 0}
            <span class="nav-badge">{garageCount}</span>
        {/if}
    </button>

    <!-- LOJA CAT (Solid Industrial Store Icon) -->
    <button 
        class="nav-item {currentTab === 'loja' ? 'active' : ''}" 
        on:click={() => onSelectTab('loja')}
    >
        <div class="nav-icon" style="display: flex; align-items: center; justify-content: center;">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                <path d="M20 4H4v2h16V4zm1 10v-2l-1-5H4l-1 5v2h1v6h10v-6h4v6h2v-6h1zm-9 6H6v-4h6v4z"/>
            </svg>
        </div>
        <span>LOJA CAT</span>
    </button>

    <!-- PLANO DE CARREIRA (Solid Shield / Level Progression Icon) -->
    <button 
        class="nav-item {currentTab === 'battlepass' ? 'active' : ''}" 
        on:click={() => onSelectTab('battlepass')}
    >
        <div class="nav-icon" style="display: flex; align-items: center; justify-content: center;">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm-2 16l-4-4 1.41-1.41L10 14.17l6.59-6.59L18 9l-8 8z"/>
            </svg>
        </div>
        <span>CARREIRA</span>
    </button>

    <!-- RANKING (Solid Industrial Championship Trophy Icon) -->
    <button 
        class="nav-item {currentTab === 'ranking' ? 'active' : ''}" 
        on:click={() => onSelectTab('ranking')}
    >
        <div class="nav-icon" style="display: flex; align-items: center; justify-content: center;">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                <path d="M19 5h-2V3H7v2H5c-1.1 0-2 .9-2 2v1c0 2.55 1.92 4.63 4.39 4.94A5.01 5.01 0 0 0 11 15.9V18H8v2h8v-2h-3v-2.1a5.01 5.01 0 0 0 3.61-3.96C19.08 11.63 21 9.55 21 7V6c0-1.1-.9-2-2-2zM5 8V7h2v3.82C5.84 10.4 5 9.3 5 8zm14 0c0 1.3-.84 2.4-2 2.82V7h2v1z"/>
            </svg>
        </div>
        <span>RANKING</span>
    </button>

    <!-- AJUSTES (Solid Industrial Gear Settings Icon) -->
    <button 
        class="nav-item {currentTab === 'config' ? 'active' : ''}" 
        on:click={() => onSelectTab('config')}
    >
        <div class="nav-icon" style="display: flex; align-items: center; justify-content: center;">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                <path d="M19.43 12.98c.04-.32.07-.64.07-.98s-.03-.66-.07-.98l2.11-1.65c.19-.15.24-.42.12-.64l-2-3.46c-.12-.22-.39-.3-.61-.22l-2.49 1c-.52-.4-1.08-.73-1.69-.98l-.38-2.65C14.46 2.18 14.25 2 14 2h-4c-.25 0-.46.18-.49.42l-.38 2.65c-.61.25-1.17.59-1.69.98l-2.49-1c-.23-.09-.49 0-.61.22l-2 3.46c-.13.22-.07.49.12.64l2.11 1.65c-.04.32-.07.65-.07.98s.03.66.07.98l-2.11 1.65c-.19.15-.24.42-.12.64l2 3.46c.12.22.39.3.61.22l2.49-1c.52.4 1.08.73 1.69.98l.38 2.65c.03.24.24.42.49.42h4c.25 0 .46-.18.49-.42l.38-2.65c.61-.25 1.17-.59 1.69-.98l2.49 1c.23.09.49 0 .61-.22l2-3.46c.12-.22.07-.49-.12-.64l-2.11-1.65zM12 15.5c-1.93 0-3.5-1.57-3.5-3.5s1.57-3.5 3.5-3.5 3.5 1.57 3.5 3.5-1.57 3.5-3.5 3.5z"/>
            </svg>
        </div>
        <span>AJUSTES</span>
    </button>
</nav>
