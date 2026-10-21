import { parseRealtimeMessage } from './realtimeContract.js';

class WebSocketClient {
  constructor() {
    this.socket = null;
    this.listeners = new Map();
    this.reconnectTimer = null;
    this.isConnected = false;
    this.manuallyClosed = false;
  }

  connect(token) {
    this.manuallyClosed = false;
    if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const wsUrl = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/ws`;
    if (!token) return;

    try {
      this.socket = new WebSocket(wsUrl, ['bearer', token]);

      this.socket.onopen = () => {
        this.isConnected = true;
        if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
        this.emit('connected', { status: 'open' });
      };

      this.socket.onmessage = (event) => {
        const message = parseRealtimeMessage(event.data);
        if (!message) {
          console.warn('[WS] Mensagem inválida recebida');
          return;
        }
        this.emit(message.event, message.payload);
      };

      this.socket.onclose = () => {
        this.isConnected = false;
        this.emit('disconnected', { status: 'closed' });
        this.socket = null;
        if (this.manuallyClosed) return;
        // Reconexão automática após 3 segundos
        this.reconnectTimer = setTimeout(() => {
          this.connect(token);
        }, 3000);
      };

      this.socket.onerror = () => {
        this.socket?.close();
      };
    } catch (err) {
      console.warn('[WS] Falha ao iniciar conexão:', err);
    }
  }

  disconnect() {
    this.manuallyClosed = true;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
    this.isConnected = false;
  }

  on(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event).add(callback);
    return () => this.off(event, callback);
  }

  off(event, callback) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).delete(callback);
    }
  }

  emit(event, data) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).forEach((cb) => {
        try {
          cb(data);
        } catch (e) {
          console.error('[WS] Erro no listener:', e);
        }
      });
    }
  }
}

export const ws = new WebSocketClient();
