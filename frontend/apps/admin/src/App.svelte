<script>
    import GestaoErp from './components/gestao/GestaoErp.svelte';
    import ToastContainer from './components/ToastContainer.svelte';
    import { api } from './services/api.js';

    let users = [];
    let transactions = [];
    let toast = { show: false, message: '', type: 'info' };

    function showToast(message, type = 'info') {
        toast = { show: true, message, type };
        setTimeout(() => {
            toast.show = false;
        }, 4000);
    }

    async function loadData() {
        try {
            const u = await api.getAdminUsers();
            if (u && u.length > 0) users = u;
            const t = await api.getAdminTransactions();
            if (t && t.length > 0) transactions = t;
        } catch (e) {
            console.warn("ERP dados carregados via cache local/mock:", e.message);
        }
    }

</script>

<div class="app-container" style="max-width: 100%; height: 100vh; overflow-y: auto;">
    <GestaoErp 
        {users}
        {transactions}
        onCloseErp={() => { window.location.href = '/'; }}
        onShowToast={showToast}
    />
</div>

{#if toast.show}
    <ToastContainer 
        message={toast.message} 
        type={toast.type} 
        onClose={() => toast.show = false} 
    />
{/if}
