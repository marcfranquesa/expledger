(() => {
  const controls = document.querySelector('.view-controls');
  const graph = document.getElementById('graph-view');
  const list = document.getElementById('list-view');
  const reports = document.getElementById('reports');
  const initialSnapshot = typeof initializeLive === 'function' ? {list: list.innerHTML, graph: graph.innerHTML, reports: reports.innerHTML} : null;
  let graphUI = initializeGraph(graph);
  let reportUI = initializeReports(reports);
  let catalogView = 'list-view', activeReport = null, catalogPosition = {x: 0, y: 0}, returnFocus = null;
  function showCatalog(view) {
    catalogView = view;
    controls.querySelectorAll('button').forEach(control => {
      const selected = control.dataset.view === view;
      control.setAttribute('aria-pressed', selected);
      document.getElementById(control.dataset.view).hidden = !selected;
    });
    document.body.classList.toggle('graph-active', view === 'graph-view');
    if (view === 'graph-view') graphUI.draw();
  }
  function routeReport(focus = false) {
    const fragment = document.getElementById(location.hash.slice(1));
    const catalogRoute = !location.hash || location.hash === '#list-view' || location.hash === '#graph-view';
    const current = activeReport && document.getElementById(activeReport.id);
    const target = (fragment && reports.contains(fragment) && fragment.closest('.report-view')) || (!catalogRoute && current && reports.contains(current) ? current : null);
    const enteringReport = target && target.id !== activeReport?.id;
    // Hidden tables report zero scroll offsets; capture before changing views.
    reportUI.capture();
    const leavingReport = activeReport && !target;
    activeReport = target;
    reports.hidden = !target;
    reports.querySelectorAll('.report-view').forEach(report => report.hidden = report !== target);
    document.body.classList.toggle('report-active', !!target);
    if (target) {
      list.hidden = true;
      graph.hidden = true;
      document.body.classList.remove('graph-active');
      target.querySelector('[data-report-back]').href = '#' + catalogView;
      reportUI.draw();
      if (focus && enteringReport) { window.scrollTo(0, 0); target.querySelector('h1').focus({preventScroll: true}); }
    } else {
      const view = location.hash === '#graph-view' ? 'graph-view' : location.hash === '#list-view' ? 'list-view' : catalogView;
      showCatalog(view);
      if (leavingReport) {
        window.scrollTo(catalogPosition.x, catalogPosition.y);
        const scope = returnFocus?.graph ? graph.querySelector('.graph-details') : list;
        const link = [...(scope?.querySelectorAll('[data-report-link]') || [])].find(link => link.getAttribute('href') === returnFocus?.href);
        (link || controls.querySelector('[aria-pressed="true"]'))?.focus({preventScroll: true});
      }
    }
  }
  document.addEventListener('click', event => {
    const link = event.target.closest('[data-report-link]');
    if (!link || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    graphUI.capture();
    catalogPosition = {x: window.scrollX, y: window.scrollY};
    returnFocus = {href: link.getAttribute('href'), graph: !graph.hidden};
  });
  window.addEventListener('hashchange', () => routeReport(true));
  routeReport();
  controls.hidden = false;
  controls.addEventListener('click', event => {
    const button = event.target.closest('button');
    if (!button || button.getAttribute('aria-pressed') === 'true') return;
    // Hidden elements may report zero scroll offsets; save them before toggling.
    graphUI.capture();
    showCatalog(button.dataset.view);
  });
  document.fonts.ready.then(() => { graphUI.draw(); reportUI.draw(); });
  function captureFocus(view) {
    const active = document.activeElement;
    if (!view.contains(active)) return null;
    return {
      id: active.closest('[data-id]')?.dataset.id,
      link: active.matches('a'), scroller: active.matches('.graph-scroll'),
      search: active.matches('input[type="search"]'),
      start: active.selectionStart, end: active.selectionEnd,
      zoom: active.dataset.zoom, targetID: active.dataset.focusId,
      details: !!active.closest('.graph-details'),
      linkSelector: active.matches('.remote-link') ? '.remote-link' : active.matches('[data-report-link]') ? '[data-report-link]' : null,
      href: active.getAttribute('href')
    };
  }
  function restoreFocus(view, focus) {
    if (!focus) return;
    const record = [...view.querySelectorAll('[data-id]')].find(node => node.dataset.id === focus.id);
    const findLink = scope => scope && ([...scope.querySelectorAll('a')].find(link => link.getAttribute('href') === focus.href) || (focus.linkSelector && scope.querySelector(focus.linkSelector)));
    let target = record && (focus.link && findLink(record) || record);
    if (focus.scroller) target = view.querySelector('.graph-scroll');
    if (focus.search) target = view.querySelector('input[type="search"]');
    if (focus.zoom) target = [...view.querySelectorAll('[data-zoom]')].find(node => node.dataset.zoom === focus.zoom);
    if (focus.targetID) target = [...view.querySelectorAll('[data-focus-id]')].find(node => node.dataset.focusId === focus.targetID);
    if (focus.details && focus.link) target = findLink(view.querySelector('.graph-details'));
    target ||= view.querySelector('[aria-pressed="true"][data-node]') || view.querySelector('.graph-scroll') || controls.querySelector('[aria-pressed="true"]');
    target?.focus({preventScroll: true});
    if (focus.search && target?.matches('input') && focus.start != null) target.setSelectionRange(focus.start, focus.end);
  }
  if (typeof initializeLive === 'function') {
    let previousList = initialSnapshot.list;
    let previousGraph = initialSnapshot.graph;
    let previousReports = initialSnapshot.reports;
    initializeLive(next => {
      const nextList = next.getElementById('list-view');
      const nextGraph = next.getElementById('graph-view');
      const nextReports = next.getElementById('reports');
      if (!nextList || !nextGraph || !nextReports) throw new Error('Invalid snapshot');
      // Keep pointer capture on the live scroller until the drag ends.
      if (graphUI.dragging()) return false;
      const position = {x: window.scrollX, y: window.scrollY};
      const reportID = activeReport?.id;
      if (nextList.innerHTML !== previousList) {
        const focus = captureFocus(list);
        previousList = nextList.innerHTML;
        list.innerHTML = previousList;
        restoreFocus(list, focus);
      }
      if (nextGraph.innerHTML !== previousGraph) {
        const saved = graphUI.capture(), focus = captureFocus(graph);
        graphUI.dispose();
        previousGraph = nextGraph.innerHTML;
        graph.innerHTML = previousGraph;
        graphUI = initializeGraph(graph, saved);
        restoreFocus(graph, focus);
      }
      if (nextReports.innerHTML !== previousReports) {
        const saved = reportUI.capture();
        const focus = reportUI.captureFocus();
        reportUI.dispose();
        previousReports = nextReports.innerHTML;
        reports.innerHTML = previousReports;
        reportUI = initializeReports(reports, saved);
        if (activeReport) routeReport();
        reportUI.restoreFocus(focus);
      }
      if (!reportID || activeReport?.id === reportID) window.scrollTo(position.x, position.y);
    });
  }
})();
