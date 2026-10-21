import { adminApi } from '../services/api.js';
import { toastState } from './toast.svelte.js';

class AdminDashboardState {
  dre = $state(null);
  withdrawals = $state([]);
  rouletteMatrix = $state([]);
  catalog = $state([]);
  users = $state([]);
  announcements = $state([]);
  config = $state(null);

  isLoading = $state(false);

  async loadDRE() {
    try {
      this.isLoading = true;
      const res = await adminApi.get('/admin/dre');
      this.dre = res.metrics || res;
    } catch (err) {
      console.warn('[Admin] Erro ao carregar DRE:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async loadWithdrawals() {
    try {
      this.isLoading = true;
      const res = await adminApi.get('/admin/withdrawals?status=pendente');
      this.withdrawals = Array.isArray(res.data) ? res.data : [];
    } catch (err) {
      console.warn('[Admin] Erro ao carregar fila de saques:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async approveWithdrawal(txId) {
    try {
      const res = await adminApi.post(`/admin/withdrawals/${txId}/approve`);
      toastState.success(res.message || 'Saque aprovado com sucesso!');
      this.withdrawals = this.withdrawals.filter((w) => w.id !== txId);
      this.loadDRE();
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao aprovar saque.');
      return false;
    }
  }

  async rejectWithdrawal(txId, reason, lockAccount = false) {
    try {
      const res = await adminApi.post(`/admin/withdrawals/${txId}/reject`, {
        reason,
        lockAccount,
      });
      toastState.success(res.message || 'Saque rejeitado e saldo estornado!');
      this.withdrawals = this.withdrawals.filter((w) => w.id !== txId);
      this.loadDRE();
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao rejeitar saque.');
      return false;
    }
  }

  async loadRouletteMatrix() {
    try {
      this.isLoading = true;
      const res = await adminApi.getRouletteMatrix();
      this.rouletteMatrix = res.slices || res.data?.slices || (Array.isArray(res) ? res : (Array.isArray(res.data) ? res.data : []));
    } catch (err) {
      console.warn('[Admin] Erro ao carregar roleta:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async saveRouletteMatrix(slices) {
    try {
      this.isLoading = true;
      const res = await adminApi.updateRouletteMatrix(slices);
      toastState.success(res.message || 'Matriz de probabilidades da roleta salva!');
      this.rouletteMatrix = slices;
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro: A soma das probabilidades deve totalizar 100%.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }

  async loadCatalog() {
    try {
      this.isLoading = true;
      const res = await adminApi.get('/shop/catalog');
      this.catalog = Array.isArray(res) ? res : (res.data || res.catalog || []);
    } catch (err) {
      console.warn('[Admin] Erro ao carregar catálogo:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async saveCatalogItem(item) {
    try {
      this.isLoading = true;
      const existingIdx = this.catalog.findIndex((c) => c.id === item.id);
      const updated = existingIdx >= 0
        ? this.catalog.map((c, i) => (i === existingIdx ? { ...c, ...item } : c))
        : [...this.catalog, item];

      const res = await adminApi.saveFleetCatalog(updated);
      toastState.success(res.message || 'Modelo de frota salvo com sucesso!');
      this.catalog = updated;
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao salvar modelo de frota.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }

  async deleteCatalogItem(id) {
    try {
      const updated = this.catalog.filter((c) => c.id !== id);
      await adminApi.saveFleetCatalog(updated);
      toastState.success('Modelo de frota removido.');
      this.catalog = updated;
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao remover modelo.');
      return false;
    }
  }

  async loadUsers(query = '', status = '') {
    try {
      this.isLoading = true;
      const q = encodeURIComponent(query);
      const s = encodeURIComponent(status);
      const res = await adminApi.get(`/admin/users?query=${q}&status=${s}`);
      this.users = res.data || res.users || (Array.isArray(res) ? res : []);
    } catch (err) {
      console.warn('[Admin] Erro ao carregar operadores:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async adjustUserBalance(userId, deltaBalance) {
    try {
      const res = await adminApi.adjustUserBalance(userId, Math.abs(deltaBalance), deltaBalance < 0 ? 'subtract' : 'add', 'Ajuste manual pelo painel administrativo');
      toastState.success(res.message || 'Saldo do operador ajustado!');
      await this.loadUsers();
      this.loadDRE();
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao ajustar saldo.');
      return false;
    }
  }

  async updateUserStatus(userId, updates) {
    try {
      if (updates.level !== undefined) {
        await adminApi.updateUserLevel(userId, updates.level);
      }
      if (updates.withdrawLocked !== undefined) {
        await adminApi.toggleWithdrawLock(userId);
      }
      if (updates.commissionLocked !== undefined) {
        await adminApi.toggleCommissionLock(userId);
      }
      toastState.success('Cadastro do operador atualizado com sucesso!');
      await this.loadUsers();
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao atualizar operador.');
      return false;
    }
  }
      toastState.error(err.message || 'Erro ao atualizar operador.');
      return false;
    }
  }

  async loadAnnouncements() {
    try {
      this.isLoading = true;
      const res = await adminApi.get('/admin/announcements');
      this.announcements = res.announcements || res || [];
    } catch (err) {
      console.warn('[Admin] Erro ao carregar comunicados:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async createAnnouncement(ann) {
    try {
      const res = await adminApi.post('/admin/announcements', ann);
      toastState.success(res.message || 'Comunicado oficial publicado no mural!');
      await this.loadAnnouncements();
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao publicar comunicado.');
      return false;
    }
  }

  async deleteAnnouncement(id) {
    try {
      await adminApi.delete(`/admin/announcements/${id}`);
      toastState.success('Comunicado removido do mural.');
      this.announcements = this.announcements.filter((a) => a.id !== id);
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao deletar comunicado.');
      return false;
    }
  }

  async loadConfig() {
    try {
      this.isLoading = true;
      const res = await adminApi.get('/admin/config');
      this.config = res.config || res;
    } catch (err) {
      console.warn('[Admin] Erro ao carregar configurações:', err);
    } finally {
      this.isLoading = false;
    }
  }

  async saveConfig(cfg) {
    try {
      this.isLoading = true;
      if (cfg.gateway) {
        await adminApi.updateGatewaySettings(cfg.gateway);
      }
      if (cfg.globalWithdrawalsLocked !== undefined) {
        await adminApi.toggleGlobalWithdrawLock(cfg.globalWithdrawalsLocked);
      }
      if (cfg.autoApproveEnabled !== undefined) {
        await adminApi.updateAutoApproveSettings(cfg.autoApproveEnabled, cfg.autoApproveWithdrawalsUnder || 0);
      }
      if (cfg.referrals) {
        await adminApi.updateReferralsSettings(cfg.referrals);
      }
      toastState.success('Configurações globais salvas!');
      this.config = cfg;
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao salvar configurações.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }
}

export const adminDashboardState = new AdminDashboardState();
