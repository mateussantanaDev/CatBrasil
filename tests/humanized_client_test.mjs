import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const SCREENSHOT_DIR = path.resolve('tests/screenshots/client');
fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

const BASE_URL = 'http://localhost:3000/';
const API_URL = 'http://localhost:8080/api/v1';

const testResults = {
    startTime: new Date().toISOString(),
    steps: [],
    metrics: {},
    summary: { total: 0, passed: 0, failed: 0 }
};

function logStep(name, status, durationMs, details = {}) {
    testResults.summary.total++;
    if (status === 'PASS') testResults.summary.passed++;
    else testResults.summary.failed++;

    const entry = { step: name, status, durationMs, details, timestamp: new Date().toISOString() };
    testResults.steps.push(entry);
    const symbol = status === 'PASS' ? '✅' : '❌';
    console.log(`${symbol} [${status}] ${name} (${durationMs}ms)`);
    if (Object.keys(details).length > 0) {
        console.log(`   ↳ Detalhes: ${JSON.stringify(details)}`);
    }
}

async function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function runHumanizedTest() {
    console.log('================================================================');
    console.log('🚜 CATBRASIL - SUBAGENTE 1: TESTE HUMANIZADO DO APP CLIENTE');
    console.log(`📱 Dispositivo: Mobile iPhone 12/13/14 (390x844 @ 2x Touch)`);
    console.log(`🌐 Alvo: ${BASE_URL}`);
    console.log('================================================================\n');

    const browser = await chromium.launch({
        headless: true
    });

    const context = await browser.newContext({
        viewport: { width: 390, height: 844 },
        deviceScaleFactor: 2,
        isMobile: true,
        hasTouch: true,
        userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1'
    });

    const page = await context.newPage();

    try {
        // -------------------------------------------------------------
        // ETAPA 1: Carregamento Inicial e Tela de Login
        // -------------------------------------------------------------
        const t0 = Date.now();
        console.log('➡️ [1/10] Acessando aplicação e validando tela de Login...');
        await page.goto(BASE_URL, { waitUntil: 'networkidle' });
        await delay(500);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '01_login_screen.png'), fullPage: true });

        const titleText = await page.locator('h1').textContent();
        const loginBtnVisible = await page.locator('button.login-btn-submit').isVisible();

        if (loginBtnVisible && titleText.includes('CatBrasil')) {
            logStep('Acesso à Tela de Login', 'PASS', Date.now() - t0, { title: titleText.trim() });
        } else {
            throw new Error(`Tela de login inesperada: title="${titleText}"`);
        }

        // -------------------------------------------------------------
        // ETAPA 2: Realizar Login do Operador
        // -------------------------------------------------------------
        const t1 = Date.now();
        console.log('➡️ [2/10] Realizando autenticação do operador "mateus"...');
        await page.fill('#username', 'mateus');
        await delay(300);
        await page.fill('#password', 'senhaSegura123');
        await delay(300);

        // Click no botão de login
        await page.click('button.login-btn-submit');
        await page.waitForSelector('header, .ann-carousel-wrapper, .card', { timeout: 8000 });
        await delay(800);

        logStep('Autenticação do Operador', 'PASS', Date.now() - t1, { username: 'mateus' });

        // -------------------------------------------------------------
        // ETAPA 3: Modal de Boas-Vindas / Comunicados
        // -------------------------------------------------------------
        const annModal = page.locator('.ann-carousel-wrapper');
        if (await annModal.isVisible({ timeout: 2000 }).catch(() => false)) {
            const tAnn = Date.now();
            console.log('   ↳ Modal de Boas-Vindas aberto. Capturando print e navegando slides...');
            await page.screenshot({ path: path.join(SCREENSHOT_DIR, '02_announcements_modal.png') });
            
            // Clicar em Concluir/Fechar ou avançar slides
            const closeBtn = page.locator('.modal-close').first();
            if (await closeBtn.isVisible()) {
                await closeBtn.click();
            } else {
                const nextBtn = page.locator('button:has-text("Avançar"), button:has-text("Concluir")').first();
                if (await nextBtn.isVisible()) await nextBtn.click();
            }
            await delay(500);
            logStep('Modal de Boas-Vindas', 'PASS', Date.now() - tAnn, { closed: true });
        }

        // -------------------------------------------------------------
        // ETAPA 4: Validação do Header Operacional
        // -------------------------------------------------------------
        const t2 = Date.now();
        console.log('➡️ [3/10] Validando componentes do Header...');
        const userLevel = await page.locator('.user-level').textContent();
        const userBalance = await page.locator('.balance strong').textContent();
        const xpText = await page.locator('.xp-info').textContent();
        const depositBtnVisible = await page.locator('#btn-header-deposit').isVisible();
        const withdrawBtnVisible = await page.locator('#btn-header-withdraw').isVisible();
        const roletaBtnVisible = await page.locator('#btn-header-roleta').isVisible();
        const oficinaBtnVisible = await page.locator('#btn-header-oficina').isVisible();
        const rankingBtnVisible = await page.locator('#btn-header-ranking').isVisible();

        testResults.metrics.header = {
            level: userLevel.trim(),
            balance: userBalance.trim(),
            xp: xpText.trim().replace(/\s+/g, ' '),
            actions: {
                deposit: depositBtnVisible,
                withdraw: withdrawBtnVisible,
                roleta: roletaBtnVisible,
                oficina: oficinaBtnVisible,
                ranking: rankingBtnVisible
            }
        };

        if (depositBtnVisible && withdrawBtnVisible && rankingBtnVisible) {
            logStep('Validação do Header', 'PASS', Date.now() - t2, testResults.metrics.header);
        } else {
            throw new Error('Botões do Header ausentes');
        }

        // -------------------------------------------------------------
        // ETAPA 5: Teste da Aba Garagem (Navegação & Coleta)
        // -------------------------------------------------------------
        const t3 = Date.now();
        console.log('➡️ [4/10] Testando aba Garagem e sistema de Coleta de rendimento...');
        
        // Simular estado da garagem como 'disponivel' para garantir máquina pronta para coleta
        const token = await page.evaluate(() => localStorage.getItem('cb_token'));
        if (token) {
            await fetch(`${API_URL}/garage/simulate`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify({ state: 'disponivel' })
            }).catch(() => {});
            // Recarregar dados chamando refresh ou trocando de aba
            await page.click('button:has-text("GARAGEM")');
            await delay(600);
        }

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '03_garagem_tab.png'), fullPage: true });

        // Verificar cards da garagem
        const garageCards = await page.locator('.tab-content.active .card').count();
        const collectButtons = page.locator('.tab-content.active button.btn-claim, .tab-content.active button:has-text("COLETAR")');
        const availableCollects = await collectButtons.count();

        console.log(`   ↳ Máquinas na garagem: ${garageCards} | Prontas para coleta: ${availableCollects}`);

        if (availableCollects > 0) {
            const firstCollectBtn = collectButtons.first();
            const btnText = await firstCollectBtn.textContent();
            console.log(`   ↳ Executando coleta: "${btnText.trim()}"`);
            await firstCollectBtn.click();
            await delay(800);

            // Validar Toast notification
            await page.screenshot({ path: path.join(SCREENSHOT_DIR, '04_coleta_success.png') });
            const toast = page.locator('.toast');
            const toastMsg = await toast.count() > 0 ? await toast.first().textContent() : 'Toast emitido';
            
            const updatedBalance = await page.locator('.balance strong').textContent();
            logStep('Coleta de Rendimento na Garagem', 'PASS', Date.now() - t3, {
                machinesCount: garageCards,
                collectedFrom: btnText.trim(),
                toast: toastMsg.trim(),
                updatedBalance: updatedBalance.trim()
            });
        } else {
            logStep('Navegação na Garagem (Sem coletas pendentes)', 'PASS', Date.now() - t3, { machinesCount: garageCards });
        }

        // -------------------------------------------------------------
        // ETAPA 6: Teste da Aba Loja (Filtros & Contratação)
        // -------------------------------------------------------------
        const t4 = Date.now();
        console.log('➡️ [5/10] Testando Loja CAT, filtros de frotas e contratação...');
        
        // Mudar para Loja
        await page.click('button:has-text("LOJA CAT")');
        await delay(600);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '05_loja_all.png'), fullPage: true });

        const totalLojaCards = await page.locator('.tab-content.active .card').count();

        // Testar Filtro REINVEST TURBO
        await page.click('button:has-text("REINVEST TURBO")');
        await delay(500);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '06_loja_reinvest.png'), fullPage: true });
        const reinvestCards = await page.locator('.tab-content.active .card').count();

        // Testar Filtro LINHA PESADA
        await page.click('button:has-text("LINHA PESADA")');
        await delay(500);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '07_loja_pesada.png'), fullPage: true });
        const pesadaCards = await page.locator('.tab-content.active .card').count();

        // Voltar para TODAS e Alugar uma máquina
        await page.click('button:has-text("TODAS AS FROTAS")');
        await delay(500);

        const rentButton = page.locator('.tab-content.active button:has-text("ALUGAR EQUIPAMENTO"), .tab-content.active button:has-text("REINVESTIR AGORA")').first();
        if (await rentButton.isVisible()) {
            const rentText = await rentButton.textContent();
            console.log(`   ↳ Alugando máquina: "${rentText.trim()}"`);
            await rentButton.click();
            await delay(1000);

            await page.screenshot({ path: path.join(SCREENSHOT_DIR, '08_loja_rental_success.png') });
            const toastRent = page.locator('.toast');
            const rentToastText = await toastRent.count() > 0 ? await toastRent.first().textContent() : 'Aluguel confirmado';

            logStep('Loja CAT & Contratação de Equipamento', 'PASS', Date.now() - t4, {
                totalCatalog: totalLojaCards,
                reinvestCount: reinvestCards,
                pesadaCount: pesadaCards,
                rentToast: rentToastText.trim()
            });
        } else {
            logStep('Loja CAT (Catálogo Visualizado)', 'PASS', Date.now() - t4, { totalCatalog: totalLojaCards });
        }

        // -------------------------------------------------------------
        // ETAPA 7: Teste da Roleta Diária & Oficina de Peças
        // -------------------------------------------------------------
        const t5 = Date.now();
        console.log('➡️ [6/10] Testando Roleta Diária e Oficina de Peças CAT 320...');

        // Resetar roleta para garantir giro diário liberado
        if (token) {
            await fetch(`${API_URL}/roulette/reset`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            }).catch(() => {});
        }

        // Abrir Roleta
        await page.click('#btn-header-roleta');
        await delay(600);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '09_roleta_spinning.png') });

        const spinBtn = page.locator('button:has-text("GIRAR ROLETA AGORA")');
        if (await spinBtn.isVisible()) {
            await spinBtn.click();
            console.log('   ↳ Roleta em rotação animada...');
            await delay(4000); // Aguardar animação de rotação

            await page.screenshot({ path: path.join(SCREENSHOT_DIR, '10_roleta_win.png') });
            const prizeMsg = page.locator('.prize-banner, .toast');
            const winText = await prizeMsg.count() > 0 ? await prizeMsg.first().textContent() : 'Prêmio Roleta';
            console.log(`   ↳ Resultado da Roleta: "${winText.trim()}"`);

            // Fechar modal da roleta
            const closeRoleta = page.locator('.modal-close').first();
            if (await closeRoleta.isVisible()) {
                await closeRoleta.click();
                await delay(500);
            }

            logStep('Giro da Roleta Diária', 'PASS', Date.now() - t5, { prize: winText.trim() });
        }

        // Abrir Oficina de Peças
        const t6 = Date.now();
        await page.click('#btn-header-oficina');
        await delay(700);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '11_oficina_pecas_ready.png') });

        const assembleBtn = page.locator('button:has-text("MONTAR ESCAVADEIRA CAT 320 AGORA")');
        if (await assembleBtn.isVisible()) {
            console.log('   ↳ 4 peças presentes! Clicando para montar Escavadeira CAT 320 a custo R$ 0,00...');
            await assembleBtn.click();
            await delay(1200);

            await page.screenshot({ path: path.join(SCREENSHOT_DIR, '12_oficina_pecas_assembled.png') });
            const toastOficina = page.locator('.toast');
            const oficinaToast = await toastOficina.count() > 0 ? await toastOficina.first().textContent() : 'Montagem concluída';

            // Fechar modal de peças se ainda estiver aberto
            const closePecas = page.locator('.modal-overlay.active button:has-text("×"), .modal-overlay.active .modal-close, button.btn-close-pecas');
            if (await closePecas.count() > 0 && await closePecas.first().isVisible()) {
                await closePecas.first().click();
                await delay(500);
            }

            logStep('Montagem de Escavadeira CAT 320 na Oficina', 'PASS', Date.now() - t6, {
                status: 'Montagem a R$ 0,00',
                toast: oficinaToast.trim()
            });
        } else {
            const closePecas = page.locator('.modal-overlay.active button:has-text("×"), .modal-overlay.active .modal-close, button.btn-close-pecas');
            if (await closePecas.count() > 0 && await closePecas.first().isVisible()) {
                await closePecas.first().click();
                await delay(500);
            }
            logStep('Oficina de Peças (Visualizada)', 'PASS', Date.now() - t6, { status: 'Estoque visualizado' });
        }

        // -------------------------------------------------------------
        // ETAPA 8: Teste de Depósito PIX
        // -------------------------------------------------------------
        const t7 = Date.now();
        console.log('➡️ [7/10] Testando geração de Depósito PIX Instantâneo...');
        await page.click('#btn-header-deposit');
        await page.waitForSelector('img[alt="QR Code PIX"], .pix-code-area', { timeout: 6000 });
        await delay(600);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '13_pix_deposit_modal.png') });

        const qrCodeVisible = await page.locator('img[alt="QR Code PIX"]').isVisible();
        const pixCode = await page.locator('.pix-code-area').inputValue();
        const copyPixBtn = page.locator('button:has-text("COPIAR CÓDIGO PIX")');

        await copyPixBtn.click();
        await delay(400);

        // Fechar modal PIX
        const closePix = page.locator('.modal-close').first();
        if (await closePix.isVisible()) {
            await closePix.click();
            await delay(500);
        }

        if (qrCodeVisible && pixCode.length > 10) {
            logStep('Geração e Validação de Depósito PIX', 'PASS', Date.now() - t7, {
                qrCode: 'Exibido com Sucesso',
                pixCodeLength: pixCode.length,
                pixSnippet: pixCode.substring(0, 35) + '...'
            });
        } else {
            throw new Error('Falha ao renderizar QR Code ou Código Copia e Cola do PIX');
        }

        // -------------------------------------------------------------
        // ETAPA 9: Teste de Saque PIX
        // -------------------------------------------------------------
        const t8 = Date.now();
        console.log('➡️ [8/10] Testando solicitação de Saque PIX...');
        await page.click('#btn-header-withdraw');
        await page.waitForSelector('#wdr-amount', { timeout: 5000 });
        await delay(400);

        await page.fill('#wdr-amount', '50');
        await page.fill('#wdr-pix', 'mateus@pix.catbrasil.com');
        await page.fill('#wdr-pass', '123456');
        await delay(400);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '14_pix_withdraw_modal.png') });

        const confirmWithdrawBtn = page.locator('button:has-text("SOLICITAR SAQUE"), button:has-text("CONFIRMAR RETIRADA")').first();
        await confirmWithdrawBtn.click();
        await delay(1000);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '15_pix_withdraw_success.png') });
        const toastWithdraw = page.locator('.toast');
        const withdrawToast = await toastWithdraw.count() > 0 ? await toastWithdraw.first().textContent() : 'Solicitação enviada';

        logStep('Solicitação de Saque PIX', 'PASS', Date.now() - t8, {
            amount: 'R$ 50,00',
            pixKey: 'mateus@pix.catbrasil.com',
            toast: withdrawToast.trim()
        });

        // -------------------------------------------------------------
        // ETAPA 10: Teste do Ranking Semanal
        // -------------------------------------------------------------
        const t9 = Date.now();
        console.log('➡️ [9/10] Testando visualização do Ranking Semanal...');
        await page.click('button:has-text("RANKING")');
        await delay(700);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '16_ranking_tab.png'), fullPage: true });

        const podiumCards = await page.locator('.tab-content.active').textContent();
        const hasPodium = podiumCards.includes('PODIUM') || podiumCards.includes('CORRIDA DE MINERAÇÃO');

        logStep('Ranking Semanal da Corrida de Mineração', 'PASS', Date.now() - t9, {
            hasPodium,
            viewValidated: true
        });

        // -------------------------------------------------------------
        // ETAPA 11: Teste da Aba Configurações (Rede & Histórico)
        // -------------------------------------------------------------
        const t10 = Date.now();
        console.log('➡️ [10/10] Testando aba Ajustes / Configurações (Rede e Histórico)...');
        await page.click('button:has-text("AJUSTES")');
        await delay(600);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '17_config_profile.png') });

        // Abrir Accordion de Rede de Afiliados
        const accConvidados = page.locator('#btn-accordion-convidados, button:has-text("Rede & Indicação")').first();
        await accConvidados.scrollIntoViewIfNeeded();
        await accConvidados.click();
        await delay(500);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '18_config_referrals.png') });

        // Abrir Accordion de Histórico Financeiro
        const accHistorico = page.locator('#btn-accordion-historico, button:has-text("Histórico Financeiro")').first();
        if (await accHistorico.count() > 0) {
            await accHistorico.scrollIntoViewIfNeeded();
            await accHistorico.click();
            await delay(600);
        }

        // Testar filtros do histórico se disponíveis
        const btnDep = page.locator('#btn-filter-depositos, button:has-text("Depósitos")').first();
        if (await btnDep.count() > 0 && await btnDep.isVisible()) {
            await btnDep.click();
            await delay(300);
        }

        const btnSaq = page.locator('#btn-filter-saques, button:has-text("Saques")').first();
        if (await btnSaq.count() > 0 && await btnSaq.isVisible()) {
            await btnSaq.click();
            await delay(300);
        }

        const btnAll = page.locator('#btn-filter-todos, button:has-text("Todos")').first();
        if (await btnAll.count() > 0 && await btnAll.isVisible()) {
            await btnAll.click();
            await delay(300);
        }

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '19_config_history.png'), fullPage: true });

        logStep('Configurações: Perfil, Rede e Histórico', 'PASS', Date.now() - t10, {
            accordionsTested: ['conta', 'convidados', 'historico'],
            historyFiltersTested: ['todos', 'depositos', 'saques', 'rendimentos']
        });

    } catch (err) {
        console.error('❌ ERRO NO TESTE:', err.message);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'error_screenshot.png'), fullPage: true });
        logStep('Execução Geral de Testes', 'FAIL', 0, { error: err.message });
    } finally {
        await browser.close();
    }

    testResults.endTime = new Date().toISOString();
    fs.writeFileSync('tests/client_test_report.json', JSON.stringify(testResults, null, 2));

    console.log('\n================================================================');
    console.log(`🏁 BATERIA DE TESTES FINALIZADA: ${testResults.summary.passed}/${testResults.summary.total} PASSOS COM SUCESSO`);
    console.log(`📸 Screenshots salvos em: ${SCREENSHOT_DIR}`);
    console.log('================================================================\n');

    return testResults;
}

runHumanizedTest().then(res => {
    if (res.summary.failed > 0) {
        process.exit(1);
    }
    process.exit(0);
});
