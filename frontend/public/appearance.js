(() => {
  let theme = 'system';
  try {
    const saved = localStorage.getItem('exams-theme');
    if (['system', 'light', 'dark'].includes(saved)) theme = saved;
  } catch {
    // Storage can be unavailable in private or restricted browsing.
  }
  document.documentElement.dataset.theme = theme;
})();
