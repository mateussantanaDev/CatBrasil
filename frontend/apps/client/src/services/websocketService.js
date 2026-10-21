import { notificationService } from './notificationService.js';
import { parseRealtimeMessage } from './realtimeContract.js';

let socket = null;
let reconnectTimer = null;
let manuallyClosed = false;

export const websocketService = {
    connect(userId, onRealtimeUpdate = () => {}) {
        if (!userId) return;
        manuallyClosed = false;

        if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
            return;
        }

        const wsBase = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/ws`;
        const token = localStorage.getItem('cb_token') || '';
        if (!token) return;

        try {
            socket = new WebSocket(wsBase, ['bearer', token]);

            socket.onopen = () => {
                console.log('⚡ [WebSocket] Conectado ao servidor de eventos em tempo real CatBrasil!');
                if (reconnectTimer) clearTimeout(reconnectTimer);
            };

            socket.onmessage = (event) => {
                const message = parseRealtimeMessage(event.data);
                if (!message) {
                    console.warn('[WS] Mensagem inválida recebida');
                    return;
                }
                if (message.event === 'pix_confirmed' && message.payload.outcome === 'applied') {
                    notificationService.notifyPixConfirmed(message.payload.paidAmount);
                }
                onRealtimeUpdate(message);
            };

            socket.onclose = () => {
                socket = null;
                if (manuallyClosed || !localStorage.getItem('cb_token')) return;
                reconnectTimer = setTimeout(() => {
                    this.connect(userId, onRealtimeUpdate);
                }, 3000);
            };

            socket.onerror = () => {
                socket?.close();
            };
        } catch (err) {
            console.warn('[WebSocket] Falha ao iniciar conexão:', err);
        }
    },

    disconnect() {
        manuallyClosed = true;
        if (reconnectTimer) clearTimeout(reconnectTimer);
        if (socket) {
            socket.close();
            socket = null;
        }
    }
};
