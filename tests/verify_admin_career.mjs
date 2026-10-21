import { chromium } from 'playwright';

async function run() {
    console.log('🚀 Iniciando verificação automatizada do Plano de Carreira no ERP...');
    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();

    page.on('console', msg => console.log('PAGE LOG:', msg.text()));

    console.log('1. Navegando para http://localhost:3002...');
    await page.goto('http://localhost:3002/');
    await page.waitForTimeout(1000);

    // Login se necessário
    const loginBtn = await page.$('button[type="submit"], button:has-text("ACESSAR SISTEMA ERP"), button:has-text("AUTENTICAR")');
    if (loginBtn) {
        console.log('Preenchendo credenciais de login...');
        await page.fill('input[placeholder*="operador"], input[type="text"]', 'operador_teste');
        const passInputs = await page.$$('input[type="password"]');
        if (passInputs.length >= 1) await passInputs[0].fill('senhaSegura123');
        if (passInputs.length >= 2) await passInputs[1].fill('123456');
        await loginBtn.click();
        await page.waitForTimeout(2000);
    }

    console.log('2. Navegando para a aba CARREIRA & 12 NÍVEIS...');
    const tabBtn = await page.waitForSelector('button:has-text("CARREIRA & 12 NÍVEIS"), button:has-text("CARREIRA")');
    await tabBtn.click();
    await page.waitForTimeout(2000);

    console.log('3. Validando banner do Plano de Carreira Perpétuo...');
    const bannerText = await page.textContent('h2:has-text("PLANO DE CARREIRA PROFISSIONAL")');
    console.log('Título encontrado:', bannerText?.trim());

    console.log('4. Validando card de Manutenção do Nível 12...');
    const maintenanceTitle = await page.textContent(':text("MANUTENÇÃO DE ATIVIDADE & BÔNUS DO NÍVEL 12")');
    console.log('Card de manutenção encontrado:', maintenanceTitle?.trim());

    console.log('5. Capturando screenshot da aba Carreira...');
    await page.screenshot({ path: 'tests/screenshots/admin/27_career_admin_tab.png', fullPage: true });
    console.log('✅ Screenshot salva em tests/screenshots/admin/27_career_admin_tab.png');

    console.log('6. Abrindo modal de Reset do Ranking...');
    const resetBtn = await page.waitForSelector('button:has-text("RESETAR RANKING")');
    await resetBtn.click();
    await page.waitForTimeout(1000);

    const modalTitle = await page.textContent('h3:has-text("CONFIRMAÇÃO DE REINÍCIO DO RANKING")');
    console.log('Título do Modal:', modalTitle?.trim());

    console.log('7. Capturando screenshot do modal de confirmação do ranking...');
    await page.screenshot({ path: 'tests/screenshots/admin/28_ranking_reset_modal.png' });
    console.log('✅ Screenshot salva em tests/screenshots/admin/28_ranking_reset_modal.png');

    await browser.close();
    console.log('🎉 Verificação da Etapa 3 concluída com 100% de sucesso!');
}

run().catch(err => {
    console.error('❌ Erro na verificação:', err);
    process.exit(1);
});
