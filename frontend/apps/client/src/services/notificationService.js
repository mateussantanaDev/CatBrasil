export const notificationService = {
    // Solicita permissão para Notificações Push ao usuário
    async requestPermission() {
        if (!('Notification' in window)) {
            console.warn("⚠️ Este navegador não suporta notificações nativas.");
            return false;
        }

        if (Notification.permission === 'granted') {
            return true;
        }

        if (Notification.permission !== 'denied') {
            const permission = await Notification.requestPermission();
            return permission === 'granted';
        }

        return false;
    },

    // Envia uma Notificação Local ou Push nativa
    async sendNotification(title, options = {}) {
        const hasPermission = await this.requestPermission();
        if (!hasPermission) return;

        const defaultOptions = {
            icon: '/favicon.svg',
            badge: '/favicon.svg',
            vibrate: [200, 100, 200],
            ...options
        };

        if ('serviceWorker' in navigator && navigator.serviceWorker.controller) {
            const reg = await navigator.serviceWorker.ready;
            reg.showNotification(title, defaultOptions);
        } else {
            new Notification(title, defaultOptions);
        }
    },

    // Alerta de Máquina Pronta para Coleta
    notifyMachineReady(machineName) {
        this.sendNotification("🚜 Rendimento Pronto para Coleta!", {
            body: `Seu ${machineName} completou o ciclo de 24h! Você tem 10 minutos para coletar antes que expire.`,
            tag: 'machine-ready'
        });
    },

    // Alerta de Depósito PIX Confirmado
    notifyPixConfirmed(amount) {
        this.sendNotification("⚡ Depósito PIX Confirmado!", {
            body: `Seu pagamento de R$ ${parseFloat(amount).toFixed(2)} foi confirmado e seu saldo já foi creditado!`,
            tag: 'pix-confirmed'
        });
    }
};
