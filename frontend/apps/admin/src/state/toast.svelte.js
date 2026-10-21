class ToastState {
  toasts = $state([]);

  show(message, type = 'info', duration = 3500) {
    const id = Date.now() + Math.random();
    this.toasts.push({ id, message, type });

    setTimeout(() => {
      this.remove(id);
    }, duration);
  }

  success(message) {
    this.show(message, 'success');
  }

  error(message) {
    this.show(message, 'error', 4500);
  }

  warning(message) {
    this.show(message, 'warning');
  }

  info(message) {
    this.show(message, 'info');
  }

  remove(id) {
    this.toasts = this.toasts.filter((t) => t.id !== id);
  }
}

export const toastState = new ToastState();
