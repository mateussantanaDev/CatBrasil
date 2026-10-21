function reais(value) {
  const amount = Number(value);
  return Number.isFinite(amount) && amount >= 0 ? amount.toFixed(2) : null;
}

export function realtimeToast({ event, payload }) {
  switch (event) {
    case 'pix_confirmed': {
      if (payload.outcome !== 'applied') return null;
      const amount = reais(payload.paidAmount);
      return amount === null ? null : { message: `⚡ Depósito PIX de R$ ${amount} confirmado em tempo real!`, type: 'win' };
    }
    case 'withdraw_paid': {
      if (payload.outcome !== 'applied') return null;
      const amount = reais(payload.paidAmount);
      return amount === null ? null : { message: `✅ Saque PIX de R$ ${amount} liquidado e confirmado pelo gateway!`, type: 'win' };
    }
    case 'withdraw_rejected':
      return { message: '❌ Seu saque foi cancelado e o valor voltou ao saldo.', type: 'error' };
    case 'balance_updated': {
      const balance = reais(payload.newBalance);
      return balance === null ? null : { message: `💰 Saldo atualizado: R$ ${balance}`, type: 'win' };
    }
    case 'announcement_created':
      return { message: '📢 Novo comunicado da gestão publicado!', type: 'win' };
    case 'daily_reset_completed':
      return { message: '🌙 Novo ciclo diário liberado! Seu giro da Roleta está disponível.', type: 'win' };
    default:
      return null;
  }
}
