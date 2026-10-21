import { api } from '../services/api.js';
import { toastState } from './toast.svelte.js';
import { authState } from './auth.svelte.js';
import { garageState } from './garage.svelte.js';

class RouletteState {
  slices = $state([]);
  isLoading = $state(false);
  isSpinning = $state(false);
  lastWin = $state(null);

  async loadConfig() {
    try {
      this.isLoading = true;
      const res = await api.get('/roulette/config');
      this.slices = res.slices || res || [];
    } catch (err) {
      console.warn('[Roulette] Erro ao carregar fatias da roleta:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async spin() {
    if (this.isSpinning) return null;
    try {
      this.isSpinning = true;
      const res = await api.post('/roulette/spin');
      this.lastWin = res;

      if (res.user) {
        authState.updateUser(res.user);
      } else {
        authState.updateUser({ alreadySpunToday: true });
      }

      return res;
    } catch (err) {
      toastState.error(err.message || 'Erro ao girar a roleta.');
      throw err;
    } finally {
      this.isSpinning = false;
    }
  }

  async assembleParts() {
    try {
      this.isLoading = true;
      const res = await api.post('/roulette/assemble');
      toastState.success(res.message || '🎉 1x Escavadeira CAT 320 montada com sucesso!');

      if (res.user) {
        authState.updateUser(res.user);
      }
      if (res.newMachine) {
        garageState.machines.push(res.newMachine);
      }
      return true;
    } catch (err) {
      toastState.error(err.message || 'Peças insuficientes para montagem na oficina.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }
}

export const rouletteState = new RouletteState();
