<script>
    export let user = {};
    export let onOpenDeposit = () => {};
    export let onOpenWithdraw = () => {};
    export let onOpenRanking = () => {};
    export let onOpenRoleta = () => {};
    export let onOpenPecas = () => {};
    export let onOpenBattlePass = () => {};
</script>

<header>
    <div class="header-top">
        <button 
            type="button"
            class="user-level mono-text" 
            style="cursor: pointer; background: var(--cat-black); border: 1px solid var(--cat-yellow); text-align: left;"
            on:click={onOpenBattlePass}
        >
            NÍVEL {user.nivel || 1} · OPERADOR ❯
        </button>
        <div class="balance">
            <span>Saldo Reais</span>
            <strong class="mono-text">R$ {user.balance ? user.balance.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</strong>
        </div>
    </div>
    <div class="xp-container" role="button" tabindex="0" style="cursor: pointer;" on:click={onOpenBattlePass} on:keydown={(e) => e.key === 'Enter' && onOpenBattlePass()}>
        <div class="xp-bar-bg">
            <div class="xp-bar-fill" style="width: {Math.min(100, ((user.xp || 240) / 1000) * 100)}%;"></div>
        </div>
        <div class="xp-info">
            <span>{user.xp || 240} / 1000 XP</span>
            <span style="font-weight:700;color:var(--cat-yellow);">PLANO DE CARREIRA ❯</span>
        </div>
    </div>
    <div class="quick-actions" style="display: flex; gap: 4px; overflow-x: auto; padding-bottom: 2px; -webkit-overflow-scrolling: touch;">
        <button class="btn-action-small" id="btn-header-deposit" on:click={onOpenDeposit}>Depositar</button>
        <button class="btn-action-small" id="btn-header-withdraw" on:click={onOpenWithdraw}>Sacar</button>
        <button class="btn-action-small btn-passe" id="btn-header-battlepass" style="background:#eab308;color:#000;font-weight:900;display:inline-flex;align-items:center;gap:3px;" on:click={onOpenBattlePass}>
            🏆 Carreira
        </button>
        <button class="btn-action-small btn-roleta" id="btn-header-roleta" style="background:#22c55e;color:#fff;font-weight:800;display:inline-flex;align-items:center;gap:3px;" on:click={onOpenRoleta}>
            🎰 Roleta
        </button>
        <button class="btn-action-small btn-oficina" id="btn-header-oficina" style="background:#0284c7;color:#fff;font-weight:800;display:inline-flex;align-items:center;gap:3px;" on:click={onOpenPecas}>
            🧩 Oficina
        </button>
        <button class="btn-action-small" id="btn-header-ranking" style="background:#ffb200;color:#111;font-weight:900;display:inline-flex;align-items:center;gap:3px;" on:click={onOpenRanking}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#111" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M6 9H4.5a2.5 2.5 0 0 1 0-5H6"/>
                <path d="M18 9h1.5a2.5 2.5 0 0 0 0-5H18"/>
                <path d="M4 22h16"/>
                <path d="M10 14.66V17c0 .55-.45 1-1 1H7v4h10v-4h-2c-.55 0-1-.45-1-1v-2.34"/>
                <path d="M18 4H6v7a6 6 0 0 0 12 0V4z"/>
            </svg>
            Ranking
        </button>
    </div>
</header>
