import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const SCREENSHOT_DIR = path.resolve('tests/screenshots/client');
fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

const BASE_URL = 'http://localhost:3000/';

async function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function runClientCareerVerification() {
    console.log('================================================================');
    console.log('🚜 CATBRASIL - VERIFICAÇÃO CLIENT APP: PLANO DE CARREIRA & NÍVEIS');
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
        console.log('1. Navegando para a página inicial...');
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

        console.log('3. Validando Header com Plano de Carreira...');
        const headerCareerText = await page.locator('text=PLANO DE CARREIRA').textContent();
        console.log(`   ↳ Texto no Header: "${headerCareerText?.trim()}"`);

        const headerBtnCareer = await page.locator('#btn-header-battlepass').textContent();
        console.log(`   ↳ Botão no Header: "${headerBtnCareer?.trim()}"`);

        console.log('4. Validando BottomNav com botão CARREIRA...');
        const navCareer = page.locator('nav.bottom-nav button:has-text("CARREIRA")');
        console.log(`   ↳ Botão CARREIRA encontrado na barra inferior: ${await navCareer.count() > 0}`);
        await navCareer.first().click();
        await delay(1500);

        console.log('5. Validando Banner do Plano de Carreira...');
        const bannerTitle = await page.locator('h2:has-text("PLANO DE CARREIRA CATBRASIL")').textContent();
        console.log(`   ↳ Título do Banner: "${bannerTitle?.trim()}"`);

        const bannerSubtitle = await page.locator('text=Progresso Perpétuo').textContent();
        console.log(`   ↳ Subtítulo do Banner: "${bannerSubtitle?.trim()}"`);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '29_client_career_tab_top.png'), fullPage: false });
        console.log('   📸 Screenshot salva: 29_client_career_tab_top.png');

        console.log('6. Validando Trilha Visual de 12 Níveis Perpétuos...');
        const trackTitle = page.locator('text=TRILHA DO PLANO DE CARREIRA');
        await trackTitle.scrollIntoViewIfNeeded();
        await delay(500);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '30_client_career_tab_track.png'), fullPage: false });
        console.log('   📸 Screenshot salva: 30_client_career_tab_track.png');

        console.log('7. Navegando para a aba RANKING...');
        const navRanking = page.locator('nav.bottom-nav button:has-text("RANKING")');
        await navRanking.first().click();
        await delay(1500);

        console.log('8. Validando Temporada de Ranking de 75 Dias...');
        const rankingSeasonBadge = await page.locator('text=TEMPORADA DE MINERAÇÃO (75 DIAS)').textContent();
        console.log(`   ↳ Badge de Temporada no Ranking: "${rankingSeasonBadge?.trim()}"`);

        const rankingNotice = await page.locator('text=A temporada de 75 dias renova exclusivamente o ranking').textContent();
        console.log(`   ↳ Aviso de Níveis Perpétuos: "${rankingNotice?.trim()}"`);

        await page.screenshot({ path: path.join(SCREENSHOT_DIR, '31_client_ranking_tab_season.png'), fullPage: false });
        console.log('   📸 Screenshot salva: 31_client_ranking_tab_season.png');

        console.log('\n✅ [SUCESSO] Teste do Plano de Carreira & Ranking no App Cliente concluído com 100% de êxito!');
    } catch (err) {
        console.error('❌ Erro no teste:', err);
        await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'error_client_career.png') });
        process.exit(1);
    } finally {
        await browser.close();
    }
}

runClientCareerVerification();
