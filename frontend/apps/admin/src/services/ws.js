class AdminWebSocketClient {
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
        try {
          const message = JSON.parse(event.data);
          const eventName = message.event || message.type || 'message';
          const payload = message.payload !== undefined ? message.payload : (message.data !== undefined ? message.data : message);

          this.emit(eventName, payload);
          if (eventName === 'balance_updated') {
            this.emit('user:balance_updated', payload);
          }
          if (eventName === 'pix_confirmed') {
            this.emit('payment:pix_confirmed', payload);
          }
        } catch (e) {
          console.warn('[Admin WS] Mensagem inválida recebida:', event.data);
        }
      };

      this.socket.onclose = () => {
        this.isConnected = false;
        this.emit('disconnected', { status: 'closed' });
        this.socket = null;
        if (this.manuallyClosed) return;
        this.reconnectTimer = setTimeout(() => {
          this.connect(token);
        }, 3000);
      };

      this.socket.onerror = () => {
        this.socket?.close();
      };
    } catch (err) {
      console.warn('[Admin WS] Falha ao conectar:', err);
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
          console.error('[Admin WS] Erro:', e);
        }
      });
    }
  }
}

export const adminWs = new AdminWebSocketClient();
