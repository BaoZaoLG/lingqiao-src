import { qsa, qs } from './dom';

export interface PageRegistry {
  [name: string]: () => void | Promise<void>;
}

export class AppShell {
  constructor(private readonly loaders: PageRegistry) {}

  bind(): void {
    qsa<HTMLElement>('[data-page-target]').forEach((item) => {
      item.addEventListener('click', () => {
        this.switchPage(item.dataset.pageTarget ?? '', item);
      });
    });

    document.addEventListener('keydown', (event) => {
      if ((event.ctrlKey || event.metaKey) && event.key === 'k') {
        event.preventDefault();
        const input = document.querySelector<HTMLInputElement>('.page.active input[type="text"]');
        input?.focus();
        input?.select();
      }
      if (event.key === 'Escape') {
        qsa<HTMLElement>('.modal-overlay.show').forEach((modal) => modal.classList.remove('show'));
      }
    });
  }

  showApp(authSelector: string, appSelector: string): void {
    qs<HTMLElement>(authSelector).style.display = 'none';
    qs<HTMLElement>(appSelector).classList.add('show');
  }

  showAuth(authSelector: string, appSelector: string): void {
    qs<HTMLElement>(authSelector).style.display = 'flex';
    qs<HTMLElement>(appSelector).classList.remove('show');
  }

  switchPage(name: string, navItem?: HTMLElement): void {
    qsa<HTMLElement>('.nav-item').forEach((item) => item.classList.remove('active'));
    const activeNav = navItem ?? document.querySelector<HTMLElement>(`[data-page-target="${name}"]`);
    activeNav?.classList.add('active');
    qsa<HTMLElement>('.page').forEach((page) => page.classList.remove('active'));
    qs<HTMLElement>(`#page-${name}`).classList.add('active');
    localStorage.setItem('lingqiao:last-admin-page', name);
    void this.loaders[name]?.();
    if (window.innerWidth <= 768) {
      document.querySelector('.sidebar')?.classList.remove('open');
      document.getElementById('sidebarOverlay')?.classList.remove('show');
    }
  }

  restoreLastPage(defaultName = 'dashboard'): void {
    const name = localStorage.getItem('lingqiao:last-admin-page') ?? defaultName;
    if (document.getElementById(`page-${name}`)) {
      this.switchPage(name);
      return;
    }
    this.switchPage(defaultName);
  }
}
