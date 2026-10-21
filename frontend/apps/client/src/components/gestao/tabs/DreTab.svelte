<script>
    export let bankMetrics = {};
    export let gestaoConfig = {};
    export let gwName = "Gateway PIX Padrão";
    export let gwDepositFee = 1.5;
    export let gwWithdrawFee = 1.00;
    export let gwPixExpMinutes = 15;

    export let onSaveGatewaySettings = () => {};
    export let onToggleGlobalLock = () => {};
</script>

<!-- ABA 1: DRE AUTOMÁTICO & AUDITORIA DE CAIXA DESACOPLADA -->
<div style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    
    <!-- PanelHeader do DRE -->
    <div style="border-bottom: 1px solid #e2e8f0; background: linear-gradient(to right, #f8fafc, #ffffff); padding: 12px 16px; display: flex; align-items: center; justify-content: space-between;">
        <div style="display: flex; align-items: center; gap: 12px;">
            <span style="width: 24px; height: 24px; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.7rem; font-weight: 700; display: flex; align-items: center; justify-content: center;">01</span>
            <div>
                <h3 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">DEMONSTRAÇÃO DO RESULTADO DA CASA (DRE AUTOMÁTICO v3.0)</h3>
                <p style="font-size: 0.68rem; color: #64748b; margin: 2px 0 0 0;">Métricas de retenção e lucros recalculados dinamicamente com base no uso real dos clientes na Garagem</p>
            </div>
        </div>
        <div style="display: flex; gap: 8px;">
            <span style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #065f46; background: #ecfdf5; border: 1px solid #047857; padding: 3px 8px;">
                MARGEM REAL BANCA: {bankMetrics.profitMargin}%
            </span>
            <span style="font-family: monospace; font-size: 0.72rem; font-weight: 700; color: #1e3a8a; background: #eff6ff; border: 1px solid #1d4ed8; padding: 3px 8px;">
                RETENÇÃO EMPÍRICA: {bankMetrics.empiricalForgetRate}%
            </span>
        </div>
    </div>

    <!-- Grid de MetricCards Brutalistas -->
    <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px;">
        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #059669; padding: 16px;">
            <span style="font-size: 0.6rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">RECEITA BRUTA (ENTRADAS PIX)</span>
            <div style="font-family: monospace; font-size: 1.6rem; font-weight: 700; color: #065f46; margin-top: 4px;">
                R$ {bankMetrics.totalDeposits ? bankMetrics.totalDeposits.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
            <span style="font-size: 0.68rem; color: #64748b; margin-top: 4px; display: block;">Gateway: {gwName} ({bankMetrics.gatewayFeePercent || 1.5}%)</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #b91c1c; padding: 16px;">
            <span style="font-size: 0.6rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">SAÍDAS AUTORIZADAS (SAQUES)</span>
            <div style="font-family: monospace; font-size: 1.6rem; font-weight: 700; color: #991b1b; margin-top: 4px;">
                R$ {bankMetrics.totalWithdrawals ? bankMetrics.totalWithdrawals.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
            <span style="font-size: 0.68rem; color: #64748b; margin-top: 4px; display: block;">Transferências PIX executadas</span>
        </div>

        <div style="background: #fff; border: 1px solid #e2e8f0; border-left: 4px solid #1e3a8a; padding: 16px;">
            <span style="font-size: 0.6rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block;">CUSTÓDIA PASSIVA (SALDO CLIENTES)</span>
            <div style="font-family: monospace; font-size: 1.6rem; font-weight: 700; color: #0f172a; margin-top: 4px;">
                R$ {bankMetrics.totalUserBalances ? bankMetrics.totalUserBalances.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
            <span style="font-size: 0.68rem; color: #64748b; margin-top: 4px; display: block;">Cobertura do Caixa: <strong>{bankMetrics.liquidityCoverageRatio || 100}%</strong></span>
        </div>

        <div style="background: #0f172a; color: #fff; border: 1px solid #0f172a; border-left: 4px solid #ffb200; padding: 16px;">
            <span style="font-size: 0.6rem; font-family: monospace; font-weight: 700; color: #ffb200; text-transform: uppercase; display: block;">RESULTADO LÍQUIDO REAL DA BANCA</span>
            <div style="font-family: monospace; font-size: 1.6rem; font-weight: 700; color: #ffb200; margin-top: 4px;">
                R$ {bankMetrics.netBankProfit ? bankMetrics.netBankProfit.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}
            </div>
            <span style="font-size: 0.68rem; color: #cbd5e1; margin-top: 4px; display: block;">Margem Retida Casa: <strong>{bankMetrics.profitMargin || 100}%</strong></span>
        </div>
    </div>

    <!-- Tabela Discriminada de DRE Estruturado (Bloomberg Style) -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <div style="border-bottom: 1px solid #e2e8f0; padding-bottom: 8px; margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center;">
            <h4 style="font-family: monospace; font-size: 0.78rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0;">DEMONSTRATIVO FINANCEIRO AUDITADO DA BANCA</h4>
            <span style="font-family: monospace; font-size: 0.65rem; color: #64748b;">DADOS REAIS PROCESSADOS EM TEMPO REAL</span>
        </div>

        <table style="width: 100%; border-collapse: collapse; font-family: monospace; font-size: 0.75rem;">
            <thead>
                <tr style="background: #f8fafc; border-bottom: 1px solid #e2e8f0; color: #475569; text-transform: uppercase; font-size: 0.62rem; text-align: left;">
                    <th style="padding: 8px 12px; border-right: 1px solid #e2e8f0;">RUBRICA CONTÁBIL / LINHA DO DRE</th>
                    <th style="padding: 8px 12px; border-right: 1px solid #e2e8f0; text-align: right;">VALOR CONSOLIDADO (R$)</th>
                    <th style="padding: 8px 12px; text-align: right;">% SOBRE RECEITA BRUTA</th>
                </tr>
            </thead>
            <tbody>
                <!-- (+) Entradas Brutas -->
                <tr style="border-bottom: 1px solid #e2e8f0; background: #ecfdf5;">
                    <td style="padding: 8px 12px; border-right: 1px solid #e2e8f0; font-weight: 700; color: #065f46;">(+) RECEITA BRUTA DE ENTRADAS (DEPÓSITOS PIX)</td>
                    <td style="padding: 8px 12px; border-right: 1px solid #e2e8f0; font-weight: 700; color: #065f46; text-align: right;">R$ {bankMetrics.totalDeposits ? bankMetrics.totalDeposits.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 8px 12px; text-align: right; font-weight: 700; color: #065f46;">100,0%</td>
                </tr>
                
                <!-- (-) Taxas Gateway -->
                <tr style="border-bottom: 1px solid #f1f5f9;">
                    <td style="padding: 6px 12px 6px 24px; border-right: 1px solid #f1f5f9; color: #64748b;">(-) Taxas de Liquidação do Gateway [{gwName}] ({bankMetrics.gatewayFeePercent || 1.5}%)</td>
                    <td style="padding: 6px 12px; border-right: 1px solid #f1f5f9; text-align: right; color: #b91c1c;">- R$ {bankMetrics.gatewayFees ? bankMetrics.gatewayFees.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 6px 12px; text-align: right; color: #b91c1c;">- {(bankMetrics.totalDeposits > 0 ? (bankMetrics.gatewayFees / bankMetrics.totalDeposits * 100) : 0).toFixed(1)}%</td>
                </tr>

                <!-- (=) Receita Líquida -->
                <tr style="border-bottom: 1px solid #e2e8f0; background: #fafafa;">
                    <td style="padding: 8px 12px; border-right: 1px solid #e2e8f0; font-weight: 700; color: #0f172a;">(=) RECEITA OPERACIONAL LÍQUIDA DO CAIXA</td>
                    <td style="padding: 8px 12px; border-right: 1px solid #e2e8f0; font-weight: 700; color: #0f172a; text-align: right;">R$ {bankMetrics.netDeposits ? bankMetrics.netDeposits.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 8px 12px; text-align: right; font-weight: 700; color: #0f172a;">{(bankMetrics.totalDeposits > 0 ? (bankMetrics.netDeposits / bankMetrics.totalDeposits * 100) : 0).toFixed(1)}%</td>
                </tr>

                <!-- (-) Saques Executados -->
                <tr style="border-bottom: 1px solid #f1f5f9;">
                    <td style="padding: 6px 12px 6px 24px; border-right: 1px solid #f1f5f9; color: #b91c1c;">(-) Retiradas Pagas a Operadores (Saques PIX Autorizados)</td>
                    <td style="padding: 6px 12px; border-right: 1px solid #f1f5f9; text-align: right; color: #b91c1c;">- R$ {bankMetrics.totalWithdrawals ? bankMetrics.totalWithdrawals.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 6px 12px; text-align: right; color: #b91c1c;">- {(bankMetrics.totalDeposits > 0 ? (bankMetrics.totalWithdrawals / bankMetrics.totalDeposits * 100) : 0).toFixed(1)}%</td>
                </tr>

                <!-- (+) Retenção de Coletas Expiradas -->
                <tr style="border-bottom: 1px solid #f1f5f9; background: #fffbeb;">
                    <td style="padding: 6px 12px 6px 24px; border-right: 1px solid #f1f5f9; color: #b45309;">(+) Retenção Automática por Coletas Expiradas (Taxa Real: {bankMetrics.empiricalForgetRate || 33.3}%)</td>
                    <td style="padding: 6px 12px; border-right: 1px solid #f1f5f9; text-align: right; color: #b45309; font-weight: 700;">+ R$ {bankMetrics.retainedExpiredYields ? bankMetrics.retainedExpiredYields.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 6px 12px; text-align: right; color: #b45309; font-weight: 700;">+ {(bankMetrics.totalDeposits > 0 ? (bankMetrics.retainedExpiredYields / bankMetrics.totalDeposits * 100) : 0).toFixed(1)}%</td>
                </tr>

                <!-- (+) Retenção de Roleta Perdida -->
                <tr style="border-bottom: 1px solid #f1f5f9; background: #fffbeb;">
                    <td style="padding: 6px 12px 6px 24px; border-right: 1px solid #f1f5f9; color: #b45309;">(+) Retenção Automática por Giros de Roleta Cancelados</td>
                    <td style="padding: 6px 12px; border-right: 1px solid #f1f5f9; text-align: right; color: #b45309; font-weight: 700;">+ R$ {bankMetrics.retainedUnspunPrizes ? bankMetrics.retainedUnspunPrizes.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 6px 12px; text-align: right; color: #b45309; font-weight: 700;">+ {(bankMetrics.totalDeposits > 0 ? (bankMetrics.retainedUnspunPrizes / bankMetrics.totalDeposits * 100) : 0).toFixed(1)}%</td>
                </tr>

                <!-- (-) Prêmios da Roleta Pago -->
                <tr style="border-bottom: 1px solid #f1f5f9;">
                    <td style="padding: 6px 12px 6px 24px; border-right: 1px solid #f1f5f9; color: #64748b;">(-) Prêmios da Roleta Diária Efetivados</td>
                    <td style="padding: 6px 12px; border-right: 1px solid #f1f5f9; text-align: right; color: #b91c1c;">- R$ {bankMetrics.totalRoulettePrizes ? bankMetrics.totalRoulettePrizes.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 6px 12px; text-align: right; color: #b91c1c;">- {(bankMetrics.totalDeposits > 0 ? (bankMetrics.totalRoulettePrizes / bankMetrics.totalDeposits * 100) : 0).toFixed(1)}%</td>
                </tr>

                <!-- (-) Comissões Afiliados -->
                <tr style="border-bottom: 1px solid #e2e8f0;">
                    <td style="padding: 6px 12px 6px 24px; border-right: 1px solid #e2e8f0; color: #64748b;">(-) Comissões Pagas à Rede de Afiliados (5%)</td>
                    <td style="padding: 6px 12px; border-right: 1px solid #e2e8f0; text-align: right; color: #b91c1c;">- R$ {bankMetrics.totalCommissionsPaid ? bankMetrics.totalCommissionsPaid.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 6px 12px; text-align: right; color: #b91c1c;">- {(bankMetrics.totalDeposits > 0 ? (bankMetrics.totalCommissionsPaid / bankMetrics.totalDeposits * 100) : 0).toFixed(1)}%</td>
                </tr>

                <!-- (=) RESULTADO LÍQUIDO DA CASA -->
                <tr style="background: #0f172a; color: #ffb200; font-size: 0.82rem;">
                    <td style="padding: 10px 12px; border-right: 1px solid #334155; font-weight: 900;">(=) RESULTADO OPERACIONAL LÍQUIDO DA BANCA (LUCRO DA CASA)</td>
                    <td style="padding: 10px 12px; border-right: 1px solid #334155; font-weight: 900; text-align: right;">R$ {bankMetrics.netBankProfit ? bankMetrics.netBankProfit.toLocaleString('pt-BR', { minimumFractionDigits: 2 }) : '0,00'}</td>
                    <td style="padding: 10px 12px; font-weight: 900; text-align: right;">{bankMetrics.profitMargin || 100}%</td>
                </tr>
            </tbody>
        </table>
    </div>

    <!-- AUDITORIA AUTOMÁTICA DE USO + CONFIGURAÇÃO DO GATEWAY PIX -->
    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px;">
        
        <!-- Painel de Auditoria Automática de Uso -->
        <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
            <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 6px 0;">AUDITORIA DE USO AUTOMÁTICA (RETENÇÃO REAIS)</h4>
            <p style="font-size: 0.7rem; color: #475569; margin: 0 0 12px 0;">Cálculo empírico automático baseado no comportamento real de coletas dos clientes.</p>

            <div style="background: #f8fafc; border: 1px solid #e2e8f0; padding: 12px; font-family: monospace; font-size: 0.75rem; display: flex; flex-direction: column; gap: 6px;">
                <div style="display: flex; justify-content: space-between;">
                    <span>Coletas Efetuadas no Prazo:</span>
                    <strong style="color: #047857;">{bankMetrics.successCollectCount || 0} eventos</strong>
                </div>
                <div style="display: flex; justify-content: space-between;">
                    <span>Coletas Expiradas por Atraso:</span>
                    <strong style="color: #b91c1c;">{bankMetrics.expiredCollectCount || 0} eventos</strong>
                </div>
                <div style="display: flex; justify-content: space-between; border-top: 1px dashed #cbd5e1; padding-top: 6px; font-weight: 700;">
                    <span>Taxa Empírica de Esquecimento:</span>
                    <strong style="color: #047857; font-size: 0.9rem;">{bankMetrics.empiricalForgetRate || 33.3}% AUTOMÁTICA</strong>
                </div>
                <div style="display: flex; justify-content: space-between; color: #b45309; font-weight: 700;">
                    <span>Total Retido pelo Caixa:</span>
                    <strong>R$ {bankMetrics.retainedExpiredYields ? bankMetrics.retainedExpiredYields.toFixed(2) : '0.00'}</strong>
                </div>
            </div>
        </div>

        <!-- Configuração do Gateway PIX -->
        <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
            <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 6px 0;">PARÂMETROS DO GATEWAY PIX (PROVEDOR OFICIAL)</h4>
            <p style="font-size: 0.7rem; color: #475569; margin: 0 0 12px 0;">Insira as taxas e o tempo de expiração do PIX do seu Gateway.</p>

            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 8px;">
                <div>
                    <label for="gw-name-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">NOME DO GATEWAY:</label>
                    <input id="gw-name-inp" type="text" placeholder="Ex: SuitPay, PrimePag..." bind:value={gwName} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
                <div>
                    <label for="gw-dep-fee-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">TAXA DEPÓSITO (%):</label>
                    <input id="gw-dep-fee-inp" type="number" step="0.1" bind:value={gwDepositFee} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
            </div>

            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px;">
                <div>
                    <label for="gw-wdr-fee-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">TAXA SAQUE FIXA (R$):</label>
                    <input id="gw-wdr-fee-inp" type="number" step="0.5" bind:value={gwWithdrawFee} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
                <div>
                    <label for="gw-exp-min-inp" style="font-size: 0.55rem; font-family: monospace; font-weight: 700; color: #64748b; text-transform: uppercase; display: block; margin-bottom: 2px;">EXPIRAÇÃO PIX (MINUTOS):</label>
                    <input id="gw-exp-min-inp" type="number" bind:value={gwPixExpMinutes} style="width: 100%; padding: 6px; font-family: monospace; font-size: 0.75rem; border: 1px solid #cbd5e1; box-sizing: border-box;" />
                </div>
            </div>

            <button on:click={onSaveGatewaySettings} style="width: 100%; background: #1e3a8a; color: #fff; font-family: monospace; font-size: 0.75rem; font-weight: 700; text-transform: uppercase; padding: 10px; border: 1px solid #000; cursor: pointer;">
                SALVAR CONFIGURAÇÃO DO GATEWAY PIX
            </button>
        </div>

    </div>

    <!-- Painel Trava de Emergência Global -->
    <div style="background: #fff; border: 1px solid #e2e8f0; padding: 16px;">
        <h4 style="font-family: monospace; font-size: 0.75rem; font-weight: 700; color: #0f172a; text-transform: uppercase; margin: 0 0 6px 0;">CONTROLE TÁTICO DE SEGURANÇA E LIQUIDEZ GLOBAL</h4>
        <p style="font-size: 0.72rem; color: #475569; margin: 0 0 14px 0;">Interrompa todas as retiradas do sistema instantaneamente em caso de instabilidade de caixa.</p>
        
        <button 
            on:click={onToggleGlobalLock}
            style="font-family: monospace; font-size: 0.75rem; font-weight: 700; text-transform: uppercase; padding: 12px 20px; border: 1px solid #000; background: {gestaoConfig.globalWithdrawalsLocked ? '#047857' : '#b91c1c'}; color: #fff; cursor: pointer; width: 100%;"
        >
            {gestaoConfig.globalWithdrawalsLocked ? 'LIBERAR SAQUES GLOBAIS PARA TODOS OS CLIENTES' : 'TRAVAR TODOS OS SAQUES DO SISTEMA EM MASSA'}
        </button>
    </div>

</div>
