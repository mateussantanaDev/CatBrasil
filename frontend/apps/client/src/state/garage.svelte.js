import { api } from '../services/api.js';
import { toastState } from './toast.svelte.js';
import { authState } from './auth.svelte.js';

class GarageState {
  machines = $state([]);
  isLoading = $state(false);
  isCollecting = $state(false);

  async loadMachines() {
    try {
      this.isLoading = true;
      const res = await api.get('/garage/machines');
      this.machines = res.machines || res || [];
    } catch (err) {
      console.warn('[Garage] Erro ao carregar máquinas:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async collectYield(machineId) {
    if (this.isCollecting) return;
    try {
      this.isCollecting = true;
      const res = await api.post(`/garage/collect/${machineId}`);

      toastState.success(res.message || 'Coleta realizada com sucesso!');

      if (res.user) {
        authState.updateUser(res.user);
      } else if (res.newBalance !== undefined) {
        authState.updateUser({
          balance: res.newBalance,
          coletasCompleted: res.coletasCompleted,
        });
      }

      // Atualiza o estado da máquina localmente
      const idx = this.machines.findIndex((m) => m.id === machineId);
      if (idx !== -1) {
        this.machines[idx] = {
          ...this.machines[idx],
          state: res.state || 'aguardando',
          secondsRemaining: 86400, // Reinicia o relógio de 24h
        };
      }

      return res;
    } catch (err) {
      toastState.error(err.message || 'Erro ao coletar rendimento.');
      // Recarrega para sincronizar estado real
      await this.loadMachines();
      throw err;
    } finally {
      this.isCollecting = false;
    }
  }

  async levelUp() {
    try {
      this.isLoading = true;
      const res = await api.post('/garage/levelup');
      toastState.success(res.message || '🎉 Nível promovido com sucesso!');

      if (res.user) {
        authState.updateUser(res.user);
      } else if (res.newLevel) {
        authState.updateUser({
          nivel: res.newLevel,
          coletasCompleted: 0,
          tratorRented: false,
        });
      }
      return true;
    } catch (err) {
      toastState.error(err.message || 'Requisitos de promoção de nível não atendidos.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }
}

export const garageState = new GarageState();
