import { api } from '../services/api.js';
import { ws } from '../services/ws.js';
import { toastState } from './toast.svelte.js';

class AuthState {
  user = $state(null);
  token = $state((typeof localStorage !== 'undefined' ? (localStorage.getItem('cb_token') || localStorage.getItem('catbrasil_token')) : '') || '');
  isLoading = $state(false);

  isLoggedIn = $derived(!!this.token && !!this.user);
  isAdmin = $derived(this.user?.role === 'admin' || this.user?.role === 'diretoria' || this.user?.role === 'gestor');

  constructor() {
    if (this.token) {
      api.setToken(this.token);
      this.fetchProfile();
    }

    window.addEventListener('auth:unauthorized', () => {
      this.logout('Sessão expirada. Faça login novamente.');
    });

    const handleBalance = (data) => {
      if (this.user && (data.userId === this.user.id || !data.userId)) {
        if (data.balance !== undefined) this.user.balance = data.balance;
        if (data.newBalance !== undefined) this.user.balance = data.newBalance;
        if (data.yieldBalanceAvailable !== undefined) {
          this.user.yieldBalanceAvailable = data.yieldBalanceAvailable;
        }
      }
    };
    ws.on('balance_updated', handleBalance);
  }

  async fetchProfile() {
    try {
      this.isLoading = true;
      const res = await api.get('/user/profile');
      this.user = res.user || res;
      ws.connect(this.token);
    } catch (err) {
      console.warn('[Auth] Falha ao carregar perfil inicial:', err);
      // Se não autenticado, limpa
      if (err.message.includes('401') || err.message.includes('inválido') || err.message.includes('expirada')) {
        this.token = '';
        api.setToken('');
      }
    } finally {
      this.isLoading = false;
    }
  }

  async login(username, password, twoFactorCode = '', fingerprint = '', ipAddress = '') {
    try {
      this.isLoading = true;
      const res = await api.post('/auth/login', {
        username,
        password,
        twoFactorCode: (twoFactorCode || '').trim(),
        fingerprint,
        ipAddress,
      });

      this.token = res.token;
      this.user = res.user;
      api.setToken(res.token);
      ws.connect(res.token);

      toastState.success(res.message || 'Login efetuado com sucesso!');
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao realizar login.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }

  async register(username, password, phone, inviteCode = '', fingerprint = '', ipAddress = '') {
    try {
      this.isLoading = true;
      const res = await api.post('/auth/register', {
        username,
        password,
        phone,
        inviteCode,
        fingerprint,
        ipAddress,
      });

      this.token = res.token;
      this.user = res.user;
      api.setToken(res.token);
      ws.connect(res.token);

      toastState.success(res.message || 'Cadastro realizado com sucesso!');
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao realizar cadastro.');
      return false;
    } finally {
      this.isLoading = false;
    }
  }

  async saveSecurityPassword(password, currentPassword = '', accountPassword = '', twoFactorCode = '') {
    try {
      await api.saveSecurityPassword(this.token, password, currentPassword, accountPassword, twoFactorCode);
      if (this.user) {
        this.user.hasSecurityPassword = true;
      }
      toastState.success('Senha financeira de 6 dígitos gravada com sucesso!');
      return true;
    } catch (err) {
      toastState.error(err.message || 'Erro ao salvar senha financeira.');
      return false;
    }
  }

  logout(message = 'Você saiu da sua conta.') {
    this.user = null;
    this.token = '';
    api.setToken('');
    ws.disconnect();
    toastState.info(message);
  }

  updateUser(updatedFields) {
    if (this.user) {
      this.user = { ...this.user, ...updatedFields };
    }
  }
}

export const authState = new AuthState();
