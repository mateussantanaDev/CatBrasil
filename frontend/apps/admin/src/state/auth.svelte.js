import { adminApi } from '../services/api.js';
import { adminWs } from '../services/ws.js';
import { toastState } from './toast.svelte.js';

class AdminAuthState {
  user = $state(null);
  token = $state((typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('catbrasil_admin_token') : '') || '');
  isLoading = $state(false);

  isAuthenticated = $derived(!!this.token && !!this.user && (this.user.role === 'admin' || this.user.role === 'diretoria' || this.user.role === 'gestor'));

  constructor() {
    if (this.token) {
      this.fetchProfile();
    }

    window.addEventListener('admin:unauthorized', () => {
      this.logout('Sessão administrativa expirada.');
    });
  }

  async fetchProfile() {
    try {
      this.isLoading = true;
      const res = await adminApi.get('/admin/profile');
      const profile = res.user || res;
      if (!profile || !['admin', 'gestor', 'diretoria'].includes(profile.role) || profile.twoFactorEnabled !== true) {
        throw new Error('Sessão administrativa inválida.');
      }
      this.user = profile;
      adminWs.connect(this.token);
    } catch (err) {
      console.warn('[Admin Auth] Falha ao validar token:', err);
      this.user = null;
      this.token = '';
      adminApi.setToken('');
      adminWs.disconnect();
    } finally {
      this.isLoading = false;
    }
  }

  async login(username, password, pin = '', twoFactorCode = '') {
    try {
      this.isLoading = true;
      this.user = null;
      this.token = '';
      adminApi.setToken('');
      const res = await adminApi.post('/admin/auth/login', {
        username: (username || '').trim(),
        password,
        pin: (pin || '').trim(),
        twoFactorCode: (twoFactorCode || '').trim(),
      });

      const token = res?.data?.token;
      const gestor = res?.data?.gestor;
      if (!token || !gestor || !['admin', 'gestor', 'diretoria'].includes(gestor.role)) {
        throw new Error('Acesso de gestão não autorizado.');
      }
      this.token = token;
      this.user = gestor;
      adminApi.setToken(token);
      adminWs.connect(token);

      toastState.success(res.message || 'Painel de Gestão conectado com sucesso!');
      return true;
    } catch (err) {
      toastState.error(err.message || 'Acesso negado: Credenciais administrativas inválidas.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }

  logout(message = 'Sessão administrativa encerrada.') {
    adminApi.post('/admin/auth/logout').catch(() => {});
    this.user = null;
    this.token = '';
    adminApi.setToken('');
    adminWs.disconnect();
    toastState.info(message);
  }
}

export const adminAuthState = new AdminAuthState();
