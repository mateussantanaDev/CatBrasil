import { io } from 'socket.io-client';
import { notificationService } from './notificationService';

let socket = null;

export const websocketService = {
    connect(userId, onRealtimeUpdate = () => {}) {
        if (!userId) return;

        if (socket && socket.connected) {
            socket.emit('join_room', userId);
            return;
        }

        socket = io('http://localhost:5001', {
            transports: ['websocket', 'polling'],
            reconnectionAttempts: 5,
            reconnectionDelay: 1000
        });

        socket.on('connect', () => {
            console.log('⚡ [WebSocket] Conectado ao servidor de eventos em tempo real!');
            socket.emit('join_room', userId);
        });

        // Escuta Notificação em Tempo Real de Depósito PIX Confirmado
        socket.on('pix_confirmed', (data) => {
            console.log('⚡ [Realtime Event] PIX Confirmado:', data);
            notificationService.notifyPixConfirmed(data.paidAmount);
            onRealtimeUpdate({ type: 'PIX_CONFIRMED', ...data });
        });

        // Escuta Notificação em Tempo Real de Atualização do Perfil / Coleta
        socket.on('profile_updated', (data) => {
            console.log('⚡ [Realtime Event] Perfil Atualizado:', data);
            onRealtimeUpdate({ type: 'PROFILE_UPDATED', ...data });
        });

        socket.on('disconnect', () => {
            console.log('⚡ [WebSocket] Desconectado do servidor.');
        });
    },

    disconnect() {
        if (socket) {
            socket.disconnect();
            socket = null;
        }
    }
};
