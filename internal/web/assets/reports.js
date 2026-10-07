function initializeReports(container, saved = {}) {
  const plots = new Map();
  container.querySelectorAll('[data-chart-type="line"]').forEach(figure => {
    if (figure.querySelector('.chart-data')) plots.set(figure.dataset.blockKey, initializeLineChart(figure, saved[figure.dataset.blockKey]));
  });
  return {
    draw: () => plots.forEach(plot => { plot.draw(); plot.restoreScroll(); }),
    capture: () => Object.fromEntries([...plots].map(([key, plot]) => [key, plot.capture()])),
    captureFocus: () => {
      const focused = document.activeElement;
      if (!container.contains(focused)) return null;
      return {
        id: focused.closest('.report-view')?.id,
        key: focused.closest('[data-block-key]')?.dataset.blockKey,
        chart: focused.matches('.chart-area'), summary: focused.matches('summary'),
        table: focused.matches('.table-scroll'), back: focused.matches('[data-report-back]'),
        href: focused.getAttribute('href')
      };
    },
    restoreFocus: focus => {
      if (!focus) return;
      const report = [...container.querySelectorAll('.report-view')].find(report => report.id === focus.id);
      const block = [...(report?.querySelectorAll('[data-block-key]') || [])].find(block => block.dataset.blockKey === focus.key);
      const target = focus.chart ? block?.querySelector('.chart-area') : focus.summary ? block?.querySelector('summary') : focus.table ? block?.querySelector('.table-scroll') : focus.back ? report?.querySelector('[data-report-back]') : focus.href ? [...(report?.querySelectorAll('a') || [])].find(link => link.getAttribute('href') === focus.href) : report?.querySelector('h1');
      (target || report?.querySelector('h1'))?.focus({preventScroll: true});
    },
    dispose: () => plots.forEach(plot => plot.dispose())
  };
}
