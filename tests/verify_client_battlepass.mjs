import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const SCREENSHOT_DIR = path.resolve('tests/screenshots/client');
fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

const BASE_URL = 'http://localhost:3000/';

async function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function runClientBattlePassVerification() {
    console.log('================================================================');
    console.log('🚜 CATBRASIL - VERIFICAÇÃO CLIENT APP: PASSE DE BATALHA 75 DIAS');
    console.log('📱 Viewport Mobile iPhone 14 (390x844 @ 2x Touch)');
    console.log('🌐 Alvo:', BASE_URL);
    console.log('================================================================\n');

    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({
        viewport: { width: 390, height: 844 },
        deviceScaleFactor: 2,
        isMobile: true,
        hasTouch: true,
        userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15'
    });

    const page = await context.newPage();

    try {
        console.log('1. Navegando para a página de login...');
        await page.goto(BASE_URL, { waitUntil: 'networkidle' });
        await delay(1000);

        console.log('2. Realizando login com operador "mateus"...');
        await page.fill('#username', 'mateus');
        await page.fill('#password', 'senhaSegura123');

        const btnEntrar = page.locator('button.login-btn-submit, button:has-text("ENTRAR NA OPERAÇÃO")');
        await btnEntrar.first().click();
        await page.waitForLoadState('networkidle');
        await delay(1500);

        // Se houver modal de boas-vindas / comunicados aberto, fecha ele
        const closeBtn = page.locator('.modal-close, button:has-text("✕")');
        if (await closeBtn.first().isVisible({ timeout: 2000 }).catch(() => false)) {
            console.log('   ↳ Fechando modal de boas-vindas...');
            await closeBtn.first().click();
            await delay(500);
        }

        console.log('3. Validando tela Garagem...');
        const headerLevel = await page.locator('.user-level').textContent();
        console.log(`   ↳ Nível do usuário no Header: ${headerLevel?.trim()}`);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '22_garagem_vale_seguro.png'), fullPage: false });
        console.log('   📸 Screenshot salva: 22_garagem_vale_seguro.png');

        console.log('4. Clicando na aba "PASSE 75D" na barra inferior...');
        const navPasse = page.locator('#nav-passe, button:has-text("PASSE 75D")');
        if (await navPasse.count() > 0) {
            await navPasse.first().click();
        } else {
            console.log('   ↳ Clicando pelo botão do Header 🏆 Passe 75D...');
            await page.locator('#btn-header-battlepass').click();
        }
        await delay(1500);

        const bannerText = await page.locator('text=PASSE DE BATALHA — TEMPORADA').textContent();
        console.log(`   ↳ Banner detectado: "${bannerText?.trim()}"`);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '23_battlepass_tab_top.png'), fullPage: false });
        console.log('   📸 Screenshot salva: 23_battlepass_tab_top.png');

        console.log('5. Verificando Missões de Promoção...');
        const missoesHeading = page.locator('text=MISSÕES OBRIGATÓRIAS PARA PROMOÇÃO');
        if (await missoesHeading.count() > 0) {
            await missoesHeading.first().scrollIntoViewIfNeeded();
            await delay(500);
            await page.screenshot({ path: path.join(SCREENSHOT_DIR, '24_battlepass_tab_missions.png'), fullPage: false });
            console.log('   📸 Screenshot salva: 24_battlepass_tab_missions.png');
        }

        console.log('6. Verificando Trilha Visual de 12 Níveis...');
        const trilhaHeading = page.locator('text=TRILHA DO PASSE DE BATALHA');
        if (await trilhaHeading.count() > 0) {
            await trilhaHeading.first().scrollIntoViewIfNeeded();
            await delay(500);
            await page.screenshot({ path: path.join(SCREENSHOT_DIR, '25_battlepass_tab_track.png'), fullPage: false });
            console.log('   📸 Screenshot salva: 25_battlepass_tab_track.png');
        }

        console.log('7. Verificando a trava estrita de avanço de nível...');
        const btnLevelUp = page.locator('button:has-text("CUMPRA AS MISSÕES"), button:has-text("AVANÇAR PARA O NÍVEL")');
        const btnText = await btnLevelUp.textContent();
        const isDisabled = await btnLevelUp.isDisabled();
        console.log(`   ↳ Botão de promoção: "${btnText?.trim()}" (Desabilitado: ${isDisabled})`);

        await page.evaluate(() => window.scrollTo(0, 0));
        await delay(300);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '26_battlepass_full.png'), fullPage: true });
        console.log('   📸 Screenshot salva: 26_battlepass_full.png');

        console.log('\n✅ [SUCESSO] Teste do Passe de Batalha 75D no App Cliente concluído com êxito!');
    } catch (err) {
        console.error('❌ Erro no teste:', err);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'error_client_battlepass.png') });
    } finally {
        await browser.close();
    }
}

runClientBattlePassVerification();
