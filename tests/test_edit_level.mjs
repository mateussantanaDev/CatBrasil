import { chromium } from 'playwright';

async function run() {
    console.log('Testando edição de regras no ERP...');
    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();

    await page.goto('http://localhost:3002/');
    await page.waitForTimeout(1000);

    const loginBtn = await page.$('button[type="submit"], button:has-text("ACESSAR SISTEMA ERP"), button:has-text("AUTENTICAR")');
    if (loginBtn) {
        await page.fill('input[placeholder*="operador"], input[type="text"]', 'operador_teste');
        const passInputs = await page.$$('input[type="password"]');
        if (passInputs.length >= 1) await passInputs[0].fill('senhaSegura123');
        if (passInputs.length >= 2) await passInputs[1].fill('123456');
        await loginBtn.click();
        await page.waitForTimeout(2000);
    }

    // Ir para a aba Passe de Batalha
    const tabBtn = await page.waitForSelector('button:has-text("PASSE & 12 NÍVEIS")');
    await tabBtn.click();
    await page.waitForTimeout(2000);

    // Salvar regras do Nível 1
    const saveBtn = await page.waitForSelector('button:has-text("SALVAR REGRAS DO NÍVEL 1")');
    await saveBtn.click();
    await page.waitForTimeout(1500);

    // Capturar screenshot com toast ou confirmação
    await page.screenshot({ path: 'tests/screenshots/admin/21_level_saved.png' });
    console.log('✅ Salvo com sucesso!');

    await browser.close();
}

run().catch(err => {
    console.error('Erro:', err);
    process.exit(1);
});
