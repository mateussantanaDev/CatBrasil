import { notificationService } from './notificationService';

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
        const token = sessionStorage.getItem('catbrasil_admin_token') || '';
        if (!token) return;

        try {
            socket = new WebSocket(wsBase, ['bearer', token]);

            socket.onopen = () => {
                console.log('⚡ [WebSocket ERP] Conectado ao servidor de eventos em tempo real CatBrasil!');
                if (reconnectTimer) clearTimeout(reconnectTimer);
            };

            socket.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    const eventName = data.event || data.type || '';
                    const payload = data.payload !== undefined ? data.payload : (data.data !== undefined ? data.data : data);

                    const normalized = {
                        ...data,
                        ...(typeof payload === 'object' && payload !== null ? payload : {}),
                        type: eventName,
                        event: eventName,
                        payload
                    };

                    console.log('⚡ [Realtime Event ERP]', eventName, normalized);

                    if (eventName === 'pix_confirmed' || eventName === 'PIX_CONFIRMED') {
                        const paid = normalized.paidAmount || normalized.amount || 0;
                        notificationService.notifyPixConfirmed(paid);
                    }

                    onRealtimeUpdate(normalized);
                } catch (e) {
                    console.warn('[WS ERP] Mensagem recebida:', event.data);
                }
            };

            socket.onclose = () => {
                socket = null;
                if (manuallyClosed || !sessionStorage.getItem('catbrasil_admin_token')) return;
                reconnectTimer = setTimeout(() => {
                    this.connect(userId, onRealtimeUpdate);
                }, 3000);
            };

            socket.onerror = () => {
                socket?.close();
            };
        } catch (err) {
            console.warn('[WebSocket ERP] Falha ao iniciar conexão:', err);
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
