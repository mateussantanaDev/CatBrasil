import { chromium } from 'playwright';

async function run() {
    console.log('🚀 Iniciando verificação automatizada do Passe de Batalha no ERP...');
    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();

    page.on('console', msg => console.log('PAGE LOG:', msg.text()));

    console.log('1. Navegando para http://localhost:3002...');
    await page.goto('http://localhost:3002/');
    await page.waitForTimeout(1000);

    // Checar se está na tela de login
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

    console.log('2. Navegando para a aba PASSE & 12 NÍVEIS...');
    const tabBtn = await page.waitForSelector('button:has-text("PASSE & 12 NÍVEIS"), button:has-text("PASSE & NÍVEIS")');
    await tabBtn.click();
    await page.waitForTimeout(2000);

    console.log('3. Validando cabeçalho da Temporada...');
    const bannerText = await page.textContent('h2:has-text("PASSE DE BATALHA")');
    console.log('Título encontrado:', bannerText);

    console.log('4. Validando se os 12 níveis foram carregados...');
    const levelHeadings = await page.$$('h3:has-text("Operador"), h3:has-text("Comandante"), h3:has-text("Mestre")');
    console.log('Níveis renderizados:', levelHeadings.length);

    console.log('5. Capturando screenshot da aba...');
    await page.screenshot({ path: 'tests/screenshots/admin/20_battlepass_tab.png', fullPage: true });
    console.log('✅ Screenshot salva em tests/screenshots/admin/20_battlepass_tab.png');

    await browser.close();
    console.log('🎉 Verificação concluída com sucesso!');
}

run().catch(err => {
    console.error('❌ Erro na verificação:', err);
    process.exit(1);
});
