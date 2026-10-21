<script>
    export let machines = [];
    export let users = [];
    export let onSaveMachinesCatalog = () => {};
    export let onShowToast = () => {};

    // Estado de Cadastro de Novo Modelo
    let newName = "";
    let newCategory = "PESADA";
    let newPrice = 50.0;
    let newReturnPercent = 15.0; // 15% ao dia
    let newDays = 30; // 30 dias
    let newMaxPerUser = 1; // Limite de 1 por cliente
    let newImageUrl = "";

    // Estado de Injeção de Promoção Direcionada (Usuário / Rede)
    let promoTargetType = "user"; // "user" | "network"
    let promoTargetId = "";
    let promoMachineId = "trator_d8t";
    let promoCustomPrice = 40.0;
    let promoCustomPercent = 20.0;
    let promoCustomMaxPurchases = 3;

    let userPromos = [];

    // Cálculos Reativos da Calculadora Automática de Rendimento
    $: calcDailyYield = parseFloat(((newPrice * (newReturnPercent / 100))).toFixed(2));
    $: calcTotalContractReturn = parseFloat((calcDailyYield * newDays).toFixed(2));

    async function handleAddMachine() {
        if (!newName || newPrice <= 0 || newReturnPercent <= 0) {
            onShowToast("Preencha o nome, preço e porcentagem de retorno válidos!", "error");
            return;
        }

        const newMachine = {
            id: "cat_custom_" + Date.now(),
            name: newName,
            category: newCategory,
            price: parseFloat(newPrice),
            returnPercentPerDay: parseFloat(newReturnPercent),
            durationDays: parseInt(newDays),
            dailyYield: calcDailyYield,
            totalContractReturn: calcTotalContractReturn,
            maxPurchasesPerUser: parseInt(newMaxPerUser),
            active: true,
            imageUrl: newImageUrl || "https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80"
        };

        machines = [newMachine, ...machines];
        if (onSaveMachinesCatalog) onSaveMachinesCatalog();

        newName = "";
        newPrice = 50.0;
        newReturnPercent = 15.0;
        newDays = 30;
        newMaxPerUser = 1;
        newImageUrl = "";
        onShowToast(`Modelo ${newMachine.name} cadastrado! Rendimento diário: R$ ${calcDailyYield.toFixed(2)}`, "win");
    }

    function toggleMachineActive(idx) {
        machines[idx].active = !machines[idx].active;
        if (onSaveMachinesCatalog) onSaveMachinesCatalog();
        onShowToast(`Status de ${machines[idx].name} alterado com sucesso!`, "info");
    }

    function deleteMachine(idx) {
        const item = machines[idx];
        machines.splice(idx, 1);
        machines = [...machines];
        if (onSaveMachinesCatalog) onSaveMachinesCatalog();
        onShowToast(`Modelo ${item.name} removido do catálogo!`, "info");
    }

    // Injeção de Promoção Personalizada por Usuário ou Rede
    function handleCreateUserPromo() {
        if (!promoTargetId) {
            onShowToast("Selecione o Usuário ou Líder da Rede alvo da promoção!", "error");
            return;
        }

        const targetUserObj = users.find(u => u.id === promoTargetId);
        const targetName = targetUserObj ? targetUserObj.username : promoTargetId;

        userPromos = [{
            id: "promo-" + Date.now(),
            title: promoTargetType === 'user' ? `OFERTA EXCLUSIVA: ${targetName.toUpperCase()}` : `PROMOÇÃO DE REDE: EQUIPE ${targetName.toUpperCase()}`,
            targetType: promoTargetType,
            targetId: promoTargetId,
            machineId: promoMachineId,
            customPrice: parseFloat(promoCustomPrice),
            customReturnPercentPerDay: parseFloat(promoCustomPercent),
            customMaxPurchases: parseInt(promoCustomMaxPurchases),
            active: true
        }, ...userPromos];

        onShowToast(`Promoção ativada com sucesso para ${targetName}!`, "win");
    }

    function handleDeleteUserPromo(id) {
        userPromos = userPromos.filter(p => p.id !== id);
        onShowToast("Promoção direcionada removida com sucesso.", "info");
    }
</script>

<!-- ABA 7: CATÁLOGO DE FROTA E PROMOÇÕES DIRECIONADAS DESACOPLADO -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- PanelHeader com KPIs da Frota -->
    <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px;">
        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #1e3a8a; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">MODELOS CADASTRADOS</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #1e3a8a; margin-top: 2px;">
                {machines.length} Equipamentos
            </div>
            <span style="font-size: 0.62rem; color: #64748b;">Catálogo global de frota</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #059669; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">PROMOÇÕES DIRECIONADAS</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #065f46; margin-top: 2px;">
                {userPromos.length} Ofertas Ativas
            </div>
            <span style="font-size: 0.62rem; color: #065f46; font-weight: 700;">POR USUÁRIO OU REDE</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #b45309; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">LIMITE PADRÃO POR CLIENTE</span>
            <div style="font-family: monospace; font-size: 1.4rem; font-weight: 700; color: #b45309; margin-top: 2px;">
                1 a 10 UNIDADES
            </div>
            <span style="font-size: 0.62rem; color: #64748b;">Restrição individual de compra</span>
        </div>

        <div style="background: #0f172a; color: #fff; border: 1px solid #0f172a; border-left: 4px solid #ffb200; padding: 12px;">
            <span style="font-size: 0.58rem; font-family: monospace; font-weight: 700; color: #ffb200; text-transform: uppercase; display: block;">CÁLCULO AUTOMÁTICO DE RENDIMENTO</span>
            <div style="font-family: monospace; font-size: 0.9rem; font-weight: 700; color: #fff; margin-top: 4px;">
                SISTEMA REATIVO (% x DIAS)
            </div>
            <span style="font-size: 0.62rem; color: #cbd5e1; display: block; margin-top: 2px;">Rendimento calculado no cadastro</span>
        </div>
    </div>

    <!-- Formulário com Calculadora Automática de Rendimento -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <div style="border-bottom: 1px solid #e2e8f0; padding-bottom: 8px; margin-bottom: 14px; display: flex; justify-content: space-between; align-items: center;">
            <h3 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">CADASTRO DE MODELO DE MÁQUINA (CALCULADORA AUTOMÁTICA DE RENDIMENTO)</h3>
            <span style="font-family: monospace; font-size: 0.65rem; color: #64748b;">CÁLCULO AUTOMÁTICO DO RENDIMENTO DIA E RETORNO DO CONTRATO</span>
        </div>

        <div style="display: grid; grid-template-columns: 2fr 1fr 1fr 1fr 1fr 1fr; gap: 10px; margin-bottom: 10px;">
            <div>
                <label for="mch-n-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">NOME DO MODELO:</label>
                <input id="mch-n-inp" type="text" placeholder="Ex: Trator CAT D8T Spec" bind:value={newName} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="mch-c-sel" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">CATEGORIA:</label>
                <select id="mch-c-sel" bind:value={newCategory} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;">
                    <option value="PESADA">PESADA</option>
                    <option value="ESCAVAÇÃO">ESCAVAÇÃO</option>
                    <option value="CARREGAMENTO">CARREGAMENTO</option>
                    <option value="NIVELAMENTO">NIVELAMENTO</option>
                    <option value="TRANSPORTE">TRANSPORTE</option>
                    <option value="COMPACTAÇÃO">COMPACTAÇÃO</option>
                </select>
            </div>

            <div>
                <label for="mch-p-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">VALOR MÁQUINA (R$):</label>
                <input id="mch-p-inp" type="number" step="10" bind:value={newPrice} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="mch-r-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">RETORNO DIA (%):</label>
                <input id="mch-r-inp" type="number" step="0.5" bind:value={newReturnPercent} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; font-weight: 900; color: #065f46; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="mch-d-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">DURAÇÃO (DIAS):</label>
                <input id="mch-d-inp" type="number" bind:value={newDays} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="mch-m-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #b45309; text-transform: uppercase; display: block; margin-bottom: 2px;">LIMITE / CLIENTE:</label>
                <input id="mch-m-inp" type="number" min="1" max="100" bind:value={newMaxPerUser} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; font-weight: 900; color: #b45309; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>
        </div>

        <!-- Barra de Resultado Calculado Automático em Tempo Real -->
        <div style="background: #f8fafc; border: 1px solid #cbd5e1; padding: 10px 14px; margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center; font-family: monospace; font-size: 0.75rem;">
            <div>
                <span style="color: #64748b; text-transform: uppercase;">RENDIMENTO DIÁRIO CALCULADO:</span>
                <strong style="color: #065f46; font-size: 0.9rem; margin-left: 6px;">R$ {calcDailyYield.toFixed(2)} / dia</strong>
            </div>

            <div>
                <span style="color: #64748b; text-transform: uppercase;">RETORNO TOTAL DO CONTRATO ({newDays} DIAS):</span>
                <strong style="color: #1e3a8a; font-size: 0.9rem; margin-left: 6px;">R$ {calcTotalContractReturn.toFixed(2)}</strong>
            </div>

            <div style="color: #b45309; font-weight: 700;">
                RESTRIÇÃO: MÁXIMO DE {newMaxPerUser} UNIDADES POR USUÁRIO
            </div>
        </div>

        <div style="display: grid; grid-template-columns: 3fr 1fr; gap: 10px;">
            <div>
                <label for="mch-u-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">URL DA FOTO DO EQUIPAMENTO:</label>
                <input id="mch-u-inp" type="url" placeholder="https://images.unsplash.com/..." bind:value={newImageUrl} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div style="display: flex; align-items: flex-end;">
                <button on:click={handleAddMachine} style="width: 100%; height: 31px; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.7rem; font-weight: 700; text-transform: uppercase; border: 1px solid #000; cursor: pointer;">
                    CADASTRAR E PUBLICAR MODELO
                </button>
            </div>
        </div>
    </div>

    <!-- PAINEL DE INJEÇÃO DE PROMOÇÕES DIRECIONADAS (USUÁRIO ESPECÍFICO / REDE DE AFILIADOS) -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <div style="border-bottom: 1px solid #e2e8f0; padding-bottom: 8px; margin-bottom: 14px; display: flex; justify-content: space-between; align-items: center;">
            <h3 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">INJEÇÃO DE OFERTA EXCLUSIVA / PROMOÇÃO DIRECIONADA</h3>
            <span style="font-family: monospace; font-size: 0.65rem; color: #047857; font-weight: 700;">ATIVAR PROMOÇÃO PARA 1 CLIENTE OU UMA ÁRVORE DE AFILIADOS</span>
        </div>

        <div style="display: grid; grid-template-columns: 1fr 2fr 1.5fr 1fr 1fr 1fr; gap: 10px; margin-bottom: 12px; align-items: flex-end;">
            <div>
                <label for="prm-tgt-typ" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">TIPO DE ALVO:</label>
                <select id="prm-tgt-typ" bind:value={promoTargetType} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;">
                    <option value="user">1 USUÁRIO ESPECÍFICO</option>
                    <option value="network">REDE DO LÍDER (AFILIADOS)</option>
                </select>
            </div>

            <div>
                <label for="prm-tgt-id" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">SELECIONE O USUÁRIO / LÍDER ALVO:</label>
                <select id="prm-tgt-id" bind:value={promoTargetId} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;">
                    <option value="">-- SELECIONE O CLIENTE --</option>
                    {#each users as u}
                        <option value={u.id}>#{u.id} - {u.username.toUpperCase()} ({u.phone})</option>
                    {/each}
                </select>
            </div>

            <div>
                <label for="prm-mch-id" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">MÁQUINA EM OFERTA:</label>
                <select id="prm-mch-id" bind:value={promoMachineId} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;">
                    {#each machines as m}
                        <option value={m.id}>{m.name} (R$ {m.price})</option>
                    {/each}
                </select>
            </div>

            <div>
                <label for="prm-prc-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">PREÇO PROMO (R$):</label>
                <input id="prm-prc-inp" type="number" step="5" bind:value={promoCustomPrice} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #065f46; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="prm-pct-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">RETORNO DIA (%):</label>
                <input id="prm-pct-inp" type="number" step="0.5" bind:value={promoCustomPercent} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; font-weight: 900; color: #b91c1c; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>

            <div>
                <label for="prm-lim-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">LIMITE / CLIENTE:</label>
                <input id="prm-lim-inp" type="number" min="1" max="10" bind:value={promoCustomMaxPurchases} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; font-weight: 700; border: 1px solid #cbd5e1; box-sizing: border-box;" />
            </div>
        </div>

        <button on:click={handleCreateUserPromo} style="width: 100%; background: #047857; color: #fff; font-family: monospace; font-size: 0.75rem; font-weight: 700; text-transform: uppercase; padding: 10px; border: 1px solid #000; cursor: pointer;">
            ACTIVAR PROMOÇÃO DIRECIONADA INSTANTANEMENTE
        </button>

        <!-- Tabela de Promoções Ativas -->
        {#if userPromos.length > 0}
            <h4 style="font-family: monospace; font-size: 0.72rem; font-weight: 700; text-transform: uppercase; color: #475569; margin: 14px 0 8px 0;">PROMOÇÕES DIRECIONADAS ATIVAS ({userPromos.length}):</h4>
            <div style="display: flex; flex-direction: column; gap: 6px;">
                {#each userPromos as p}
                    <div style="border: 1px solid #e2e8f0; padding: 8px 12px; background: #ecfdf5; display: flex; align-items: center; justify-content: space-between; font-family: monospace; font-size: 0.72rem;">
                        <div>
                            <strong style="color: #065f46;">{p.title}</strong>
                            <span style="color: #64748b; margin-left: 8px;">MÁQUINA: #{p.machineId} | PREÇO: R$ {p.customPrice.toFixed(2)} | RETORNO: {p.customReturnPercentPerDay}%/dia | LIMITE: {p.customMaxPurchases} UNID</span>
                        </div>
                        <button on:click={() => handleDeleteUserPromo(p.id)} style="background: #b91c1c; color: #fff; font-size: 0.6rem; padding: 4px 8px; border: none; cursor: pointer; font-weight: 700;">CANCELAR OFERTA</button>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <!-- Tabela Densa B2G do Catálogo de Frota -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
            <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">CATÁLOGO GERAL DE MÁQUINAS (VALORES RECALCULADOS AUTOMATICAMENTE)</h4>
            <button on:click={onSaveMachinesCatalog} style="padding: 6px 14px; font-size: 0.68rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #047857; color: #fff; cursor: pointer;">
                SALVAR ALTERAÇÕES DO CATÁLOGO
            </button>
        </div>

        <div style="display: flex; flex-direction: column; gap: 8px;">
            {#each machines as machine, idx}
                <div style="display: grid; grid-template-columns: 60px 2fr 1fr 1fr 1fr 1fr 1fr 160px; gap: 10px; padding: 10px; border: 1px solid #e2e8f0; background: {machine.active ? '#fff' : '#fef2f2'}; align-items: center;">
                    
                    <!-- Thumbnail -->
                    <img src={machine.imageUrl} alt={machine.name} style="width: 55px; height: 40px; object-fit: cover; border: 1px solid #000;" />

                    <!-- Informações Básicas -->
                    <div>
                        <strong style="font-family: monospace; font-size: 0.78rem; color: #0f172a; display: block;">{machine.name}</strong>
                        <span style="font-size: 0.58rem; font-family: monospace; color: #64748b;">CAT: {machine.category} · ID: {machine.id}</span>
                    </div>

                    <!-- Preço de Aluguel -->
                    <div>
                        <label for="mch-ed-p-{idx}" style="font-size: 0.52rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">VALOR (R$):</label>
                        <input id="mch-ed-p-{idx}" type="number" bind:value={machine.price} style="width: 100%; padding: 4px; font-family: monospace; font-size: 0.75rem; font-weight: 700; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <!-- Retorno Dia % -->
                    <div>
                        <label for="mch-ed-r-{idx}" style="font-size: 0.52rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">RETORNO DIA (%):</label>
                        <input id="mch-ed-r-{idx}" type="number" step="0.5" bind:value={machine.returnPercentPerDay} style="width: 100%; padding: 4px; font-family: monospace; font-size: 0.75rem; font-weight: 900; color: #065f46; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <!-- Duração Dias -->
                    <div>
                        <label for="mch-ed-d-{idx}" style="font-size: 0.52rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">DURAÇÃO (DIAS):</label>
                        <input id="mch-ed-d-{idx}" type="number" bind:value={machine.durationDays} style="width: 100%; padding: 4px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <!-- Limite por Cliente -->
                    <div>
                        <label for="mch-ed-m-{idx}" style="font-size: 0.52rem; font-family: monospace; color: #b45309; text-transform: uppercase; display: block;">LIMITE / CLIENTE:</label>
                        <input id="mch-ed-m-{idx}" type="number" min="1" max="100" bind:value={machine.maxPurchasesPerUser} style="width: 100%; padding: 4px; font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #b45309; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                    </div>

                    <!-- Rendimento Calculado -->
                    <div>
                        <span style="font-size: 0.52rem; font-family: monospace; color: #64748b; text-transform: uppercase; display: block;">RENDIMENTO / DIA:</span>
                        <strong style="font-family: monospace; font-size: 0.8rem; color: #1e3a8a;">R$ {(machine.price * (machine.returnPercentPerDay / 100)).toFixed(2)}</strong>
                    </div>

                    <!-- Ações -->
                    <div style="display: flex; gap: 4px; justify-content: flex-end;">
                        <button 
                            on:click={() => toggleMachineActive(idx)}
                            style="padding: 4px 8px; font-size: 0.6rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: {machine.active ? '#047857' : '#b45309'}; color: #fff; cursor: pointer;"
                        >
                            {machine.active ? 'ATIVO' : 'PAUSADO'}
                        </button>
                        <button 
                            on:click={() => deleteMachine(idx)}
                            style="padding: 4px 8px; font-size: 0.6rem; font-family: monospace; font-weight: 700; border: 1px solid #000; background: #b91c1c; color: #fff; cursor: pointer;"
                        >
                            EXCLUIR
                        </button>
                    </div>

                </div>
            {/each}
        </div>
    </div>

</div>
