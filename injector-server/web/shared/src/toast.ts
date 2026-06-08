export type ToastType = 'success' | 'error' | 'warning';

export class ToastHost {
  private readonly element: HTMLElement;
  private timer: number | undefined;

  constructor(id = 'toast') {
    const existing = document.getElementById(id);
    if (existing) {
      this.element = existing;
    } else {
      this.element = document.createElement('div');
      this.element.id = id;
      this.element.className = 'toast';
      document.body.appendChild(this.element);
    }
  }

  show(message: string, type: ToastType = 'success'): void {
    window.clearTimeout(this.timer);
    this.element.textContent = message;
    this.element.className = `toast ${type} show`;
    this.timer = window.setTimeout(() => {
      this.element.className = 'toast';
    }, 3500);
  }
}
