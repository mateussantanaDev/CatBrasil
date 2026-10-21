import { api } from '../services/api.js';
import { toastState } from './toast.svelte.js';
import { authState } from './auth.svelte.js';
import { garageState } from './garage.svelte.js';

class ShopState {
  catalog = $state([]);
  isLoading = $state(false);
  isRenting = $state(false);
  selectedCategory = $state('TODAS'); // 'TODAS' | 'ESCAVADEIRAS' | 'PESADA' | 'REINVEST'

  filteredCatalog = $derived(() => {
    if (this.selectedCategory === 'TODAS') return this.catalog;
    if (this.selectedCategory === 'REINVEST') return this.catalog.filter((m) => m.reinvestOnly);
    return this.catalog.filter((m) => m.category?.toUpperCase() === this.selectedCategory && !m.reinvestOnly);
  });

  async loadCatalog() {
    try {
      this.isLoading = true;
      const res = await api.get('/shop/catalog');
      this.catalog = res.catalog || res || [];
    } catch (err) {
      console.warn('[Shop] Erro ao carregar catálogo de frotas:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async rentMachine(catalogId) {
    if (this.isRenting) return false;
    try {
      this.isRenting = true;
      const res = await api.post(`/shop/rent/${catalogId}`);

      toastState.success(res.message || 'Contrato de locação firmado com sucesso!');

      if (res.user) {
        authState.updateUser(res.user);
      } else if (res.balance !== undefined) {
        authState.updateUser({
          balance: res.balance,
        });
      }

      // Adiciona máquina recém-alugada na garagem
      if (res.newMachine) {
        garageState.machines.push(res.newMachine);
      }

      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao contratar máquina.');
      return false;
    } finally {
      this.isRenting = false;
    }
  }
}

export const shopState = new ShopState();
