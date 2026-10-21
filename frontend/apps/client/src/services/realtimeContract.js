// Contrato enviado pelo Hub Go: { event, payload, userId? }.
export function parseRealtimeMessage(raw) {
  let message;
  try {
    message = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!message || typeof message !== 'object' || Array.isArray(message) ||
      typeof message.event !== 'string' || !message.event.trim() ||
      !message.payload || typeof message.payload !== 'object' || Array.isArray(message.payload)) {
    return null;
  }
  return { event: message.event, payload: message.payload, userId: message.userId };
}
