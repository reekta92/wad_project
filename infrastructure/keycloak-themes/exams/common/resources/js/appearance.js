(() => {
  const root = document.documentElement;
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  let theme = 'system';
  try {
    const saved = localStorage.getItem('exams-theme');
    if (['system', 'light', 'dark'].includes(saved)) theme = saved;
  } catch { /* Appearance still works when browser storage is unavailable. */ }

  function apply() {
    root.dataset.theme = theme;
    root.classList.toggle('pf-v5-theme-dark', theme === 'dark' || (theme === 'system' && media.matches));
  }
  apply();
  media.addEventListener('change', apply);

  function controls() {
    const header = document.getElementById('kc-header');
    if (!header) return;
    const container = document.createElement('div');
    container.className = 'exams-appearance';
    const label = document.createElement('label');
    label.htmlFor = 'exams-theme';
    label.textContent = 'Color theme';
    const select = document.createElement('select');
    select.id = 'exams-theme';
    for (const [value, text] of [['system', 'Device theme'], ['light', 'Light'], ['dark', 'Dark']]) {
      const option = document.createElement('option');
      option.value = value;
      option.textContent = text;
      select.append(option);
    }
    select.value = theme;
    select.addEventListener('change', () => {
      theme = select.value;
      try { localStorage.setItem('exams-theme', theme); } catch { /* Keep the current page usable. */ }
      apply();
    });
    container.append(label, select);
    header.append(container);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', controls, { once: true });
  else controls();
})();
