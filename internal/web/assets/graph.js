function initializeGraph(graph, saved = {}) {
  const canvas = graph.querySelector('.graph-canvas');
  const rows = graph.querySelector('.graph-rows');
  const scroll = graph.querySelector('.graph-scroll');
  const stage = graph.querySelector('.graph-stage');
  const details = graph.querySelector('.graph-details');
  const nodes = [...graph.querySelectorAll('[data-node]')];
  const paths = [...graph.querySelectorAll('path[data-from]')];
  const byID = new Map(nodes.map(node => [node.dataset.node, node]));
  let selected = null, zoom = saved.zoom ?? .75;
  let pendingPosition = saved;
  const listeners = new AbortController();
  // Separate disconnected lineages so unrelated roots do not widen every rank.
  const neighbors = new Map(nodes.map(node => [node, []]));
  const parents = new Map(nodes.map(node => [node, []]));
  paths.forEach(path => {
    const from = byID.get(path.dataset.from), to = byID.get(path.dataset.to);
    parents.get(to).push(from);
    neighbors.get(from).push(to);
    neighbors.get(to).push(from);
  });
  const visited = new Set(), components = [];
  nodes.forEach(node => {
    if (visited.has(node)) return;
    const members = [], pending = [node];
    visited.add(node);
    while (pending.length) {
      const next = pending.pop();
      members.push(next);
      neighbors.get(next).forEach(neighbor => {
        if (!visited.has(neighbor)) { visited.add(neighbor); pending.push(neighbor); }
      });
    }
    components.push(members);
  });
  const created = node => Date.parse(node.querySelector('time')?.dateTime || '') || 0;
  const newestFirst = (a, b) => created(b) - created(a) || +a.dataset.node - +b.dataset.node;
  const latestDescendant = members => Math.max(...members.filter(node => node.dataset.terminal === 'true').map(created));
  components.sort((a, b) => latestDescendant(b) - latestDescendant(a) || +a[0].dataset.node - +b[0].dataset.node);
  if (rows) {
    rows.replaceChildren();
    components.forEach(members => {
      const component = document.createElement('div');
      component.className = 'graph-component';
      const ranks = new Map();
      members.sort(newestFirst).forEach(node => {
        const rank = +node.dataset.rank;
        if (!ranks.has(rank)) ranks.set(rank, []);
        ranks.get(rank).push(node);
      });
      const positions = new Map();
      const parentPosition = node => {
        const known = parents.get(node).filter(parent => positions.has(parent));
        return known.length ? known.reduce((sum, parent) => sum + positions.get(parent), 0) / known.length : Infinity;
      };
      [...ranks].sort((a, b) => a[0] - b[0]).forEach(([, members]) => {
        members.sort((a, b) => parentPosition(a) - parentPosition(b));
        members.forEach((node, index) => positions.set(node, index));
        const row = document.createElement('div');
        row.className = 'graph-row';
        row.append(...members);
        component.prepend(row);
      });
      rows.append(component);
    });
  }
  function setZoom(value, centerNode) {
    if (!canvas) return;
    const old = zoom;
    const cx = (scroll.scrollLeft + scroll.clientWidth / 2) / old;
    const cy = (scroll.scrollTop + scroll.clientHeight / 2) / old;
    zoom = Math.max(.02, Math.min(2, value));
    canvas.style.transform = `scale(${zoom})`;
    canvas.style.left = `${Math.max(0, (scroll.clientWidth - canvas.offsetWidth * zoom) / 2)}px`;
    stage.style.width = `${Math.max(scroll.clientWidth, canvas.offsetWidth * zoom)}px`;
    stage.style.height = `${Math.max(scroll.clientHeight, canvas.offsetHeight * zoom)}px`;
    graph.querySelector('.zoom-level').textContent = `${Math.round(zoom * 100)}%`;
    if (centerNode) {
      const r = centerNode.getBoundingClientRect(), c = canvas.getBoundingClientRect();
      scroll.scrollLeft = r.left - c.left + r.width / 2 - scroll.clientWidth / 2;
      scroll.scrollTop = r.top - c.top + r.height / 2 - scroll.clientHeight / 2;
    } else {
      scroll.scrollLeft = cx * zoom - scroll.clientWidth / 2;
      scroll.scrollTop = cy * zoom - scroll.clientHeight / 2;
    }
  }
  function drawGraph() {
    if (graph.hidden || !canvas) return;
    canvas.style.transform = 'none';
    const groups = [...rows.children];
    const perRow = Math.max(2, Math.min(5, Math.floor((scroll.clientWidth / .75 - 112) / 184)));
    graph.querySelectorAll('.graph-row').forEach(row => { row.style.width = `${Math.min(perRow, row.children.length) * 184 - 16}px`; });
    const columnWidth = Math.max(...groups.map(group => group.offsetWidth));
    const targetColumns = Math.floor((scroll.clientWidth / .75 - 16) / (columnWidth + 32));
    const columns = Math.min(groups.length, Math.max(1, targetColumns));
    const bottoms = Array(columns).fill(24);
    groups.forEach(group => {
      const column = bottoms.indexOf(Math.min(...bottoms));
      group.style.left = `${24 + column * (columnWidth + 32)}px`;
      group.style.top = `${bottoms[column]}px`;
      bottoms[column] += group.offsetHeight + 32;
    });
    rows.style.width = `${columns * (columnWidth + 32) + 16}px`;
    rows.style.height = `${Math.max(...bottoms)}px`;
    const origin = canvas.getBoundingClientRect();
    const box = node => { const r = node.getBoundingClientRect(); return {x:r.left-origin.left, y:r.top-origin.top, w:r.width, h:r.height}; };
    const boxes = new Map(nodes.map(node => [node, box(node)]));

    const rightEdges = new Map();
    nodes.forEach(node => {
      const group = node.closest('.graph-component'), r = boxes.get(node);
      rightEdges.set(group, Math.max(rightEdges.get(group) || 0, r.x + r.w));
    });
    const rowGap = parseFloat(getComputedStyle(graph.querySelector('.graph-row')).rowGap);
    paths.forEach(path => {
      const from = byID.get(path.dataset.from), to = byID.get(path.dataset.to);
      const a = boxes.get(from), b = boxes.get(to), group = from.closest('.graph-component');
      const x = a.x+a.w/2, y = a.y, tx = b.x+b.w/2, end = b.y+b.h;
      const sameRow = from.dataset.rank === to.dataset.rank;
      path.classList.toggle('cycle', sameRow);
      let d;
      if (from === to) {
        const bottom = a.y + a.h;
        d = `M ${x-8} ${bottom} C ${x-30} ${bottom+12}, ${x+30} ${bottom+12}, ${x+8} ${bottom}`;
      } else if (sameRow && a.y === b.y) {
        const bottom = a.y + a.h, gutter = bottom + 8;
        d = `M ${x-6} ${bottom} V ${gutter} H ${tx+6} V ${end}`;
      } else if (sameRow || +to.dataset.rank > +from.dataset.rank + 1 || y - end > rowGap + 1) {
        const right = rightEdges.get(group);
        // Shared trunks collapse parallel routes; each edge keeps its own endpoints for highlighting.
        const side = right + 16;
        const start = sameRow ? a.y + a.h : y;
        const gutter = sameRow ? start + 8 : start - 16;
        const targetGutter = end + (sameRow ? 8 : 16);
        d = `M ${x} ${start} V ${gutter} H ${side} V ${targetGutter} H ${tx} V ${end}`;
      } else {
        const mid = (y + end) / 2;
        d = `M ${x} ${y} V ${mid} H ${tx} V ${end}`;
      }
      path.setAttribute('d', d);
    });
    setZoom(zoom);
    if (pendingPosition) {
      scroll.scrollTo(pendingPosition.left || 0, pendingPosition.top || 0);
      details.scrollTop = pendingPosition.detailsTop || 0;
      pendingPosition = null;
    }
  }
  function highlight(node) {
    if (!canvas) return;
    canvas.classList.toggle('highlighting', !!node);
    const related = new Set(node ? [node.dataset.node] : []);
    paths.forEach(path => {
      const incident = node && (path.dataset.from === node.dataset.node || path.dataset.to === node.dataset.node);
      path.classList.toggle('related', !!incident);
      if (incident) { related.add(path.dataset.from); related.add(path.dataset.to); path.parentNode.append(path); }
    });
    nodes.forEach(card => {
      card.classList.toggle('related', related.has(card.dataset.node));
      card.classList.toggle('highlighted', card === node);
    });
  }
  function selectNode(node, center = false) {
    if (search) search.value = '';
    selected = node;
    nodes.forEach(card => card.setAttribute('aria-pressed', card === node));
    details.replaceChildren();
    if (node) {
      const title = document.createElement('h2');
      title.textContent = node.querySelector('h2').textContent;
      const metadata = node.querySelector('.node-details').cloneNode(true);
      metadata.hidden = false;
      metadata.querySelectorAll('[id]').forEach(element => element.removeAttribute('id'));
      details.append(title, metadata);
      for (const [label, end, other] of [['Parents', 'to', 'from'], ['Children', 'from', 'to']]) {
        const links = paths.filter(path => path.dataset[end] === node.dataset.node);
        if (!links.length) continue;
        const heading = document.createElement('h3');
        heading.textContent = `${label} · ${links.length}`;
        details.append(heading);
        links.forEach(path => {
          const target = byID.get(path.dataset[other]);
          const button = document.createElement('button');
          button.type = 'button';
          button.textContent = target.querySelector('h2').textContent;
          button.title = target.querySelector('.id').textContent;
          button.dataset.focusId = target.dataset.id;
          button.addEventListener('click', () => { selectNode(target, true); target.focus({preventScroll: true}); });
          details.append(button);
        });
      }
      if (center) { setZoom(Math.max(.75, zoom), node); }
    } else {
      const p = document.createElement('p');
      p.textContent = 'Select an experiment';
      details.append(p);
    }
    highlight(node);
  }
  nodes.forEach(node => {
    node.title = node.querySelector('h2').textContent + '\n' + node.querySelector('.id').textContent;
    node.addEventListener('dblclick', () => { selectNode(node); setZoom(1, node); });
    node.addEventListener('click', () => selectNode(selected === node ? null : node));
    node.addEventListener('keydown', event => {
      if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectNode(selected === node ? null : node, true); }
    });
    node.addEventListener('focus', () => highlight(node));
    node.addEventListener('blur', restoreHighlight);
    node.addEventListener('mouseenter', () => highlight(node));
    node.addEventListener('mouseleave', restoreHighlight);
  });
  graph.addEventListener('keydown', event => {
    if (event.key !== 'Escape' || !details) return;
    const inSearch = event.target === search;
    const target = inSearch || search?.value.trim() ? selected : null;
    selectNode(target);
    if (!inSearch) (target || scroll).focus({preventScroll: true});
  }, {signal: listeners.signal});
  graph.querySelectorAll('[data-zoom]').forEach(button => button.addEventListener('click', () => {
    setZoom(button.dataset.zoom === 'actual' ? 1 : zoom * (button.dataset.zoom === 'in' ? 1.4 : 1 / 1.4), selected);
  }));
  let drag = null, dragged = false;
  if (scroll) {
    scroll.addEventListener('wheel', event => {
      if (!event.ctrlKey && !event.metaKey) return;
      event.preventDefault();
      const before = canvas.getBoundingClientRect();
      const x = (event.clientX - before.left) / zoom;
      const y = (event.clientY - before.top) / zoom;
      const unit = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? scroll.clientHeight : 1;
      const delta = Math.max(-240, Math.min(240, event.deltaY * unit));
      setZoom(zoom * Math.exp(-delta * .002));
      const after = canvas.getBoundingClientRect();
      scroll.scrollLeft += after.left + x * zoom - event.clientX;
      scroll.scrollTop += after.top + y * zoom - event.clientY;
    }, {passive: false});
    scroll.addEventListener('click', event => {
      if (!dragged && !event.target.closest('[data-node]')) { selectNode(null); scroll.focus({preventScroll: true}); }
    });
    scroll.addEventListener('pointerdown', event => {
      dragged = false;
      if (event.button !== 0 || event.pointerType !== 'mouse' || event.target.closest('[data-node]')) return;
      const bounds = scroll.getBoundingClientRect();
      if (event.clientX - bounds.left >= scroll.clientWidth || event.clientY - bounds.top >= scroll.clientHeight) return;
      drag = {x: event.clientX, y: event.clientY, left: scroll.scrollLeft, top: scroll.scrollTop};
      scroll.setPointerCapture(event.pointerId);
      event.preventDefault();
    });
    scroll.addEventListener('pointermove', event => {
      if (!drag) return;
      if (Math.abs(event.clientX - drag.x) + Math.abs(event.clientY - drag.y) > 4) dragged = true;
      scroll.scrollLeft = drag.left + drag.x - event.clientX;
      scroll.scrollTop = drag.top + drag.y - event.clientY;
    });
    scroll.addEventListener('lostpointercapture', () => { drag = null; });
  }
  const search = graph.querySelector('input[type="search"]');
  function searchMatches() {
    const query = search.value.trim().toLowerCase();
    return nodes.filter(node => (node.querySelector('h2').textContent + ' ' + node.querySelector('.id').textContent).toLowerCase().includes(query));
  }
  function restoreHighlight() {
    if (!search?.value.trim()) { highlight(selected); return; }
    highlight(null);
    canvas.classList.add('highlighting');
    searchMatches().forEach(node => node.classList.add('related'));
  }
  function renderSearch() {
    if (!search.value.trim()) { selectNode(selected); return; }
    details.replaceChildren();
    const matches = searchMatches();
    restoreHighlight();
    const heading = document.createElement('h2');
    heading.textContent = `${matches.length} experiment${matches.length === 1 ? '' : 's'}`;
    details.append(heading);
    matches.forEach(node => {
      const button = document.createElement('button');
      button.type = 'button';
      button.textContent = node.querySelector('h2').textContent;
      button.dataset.focusId = node.dataset.id;
      button.addEventListener('click', () => { selectNode(node, true); node.focus({preventScroll: true}); });
      details.append(button);
    });
  }
  if (search) search.addEventListener('input', renderSearch);

  if (details) {
    selectNode(nodes.find(node => node.dataset.id === saved.selectedID) || null);
    search.value = saved.query || '';
    if (search.value.trim()) renderSearch();
  }
  const observer = new ResizeObserver(drawGraph);
  if (scroll) observer.observe(scroll);
  drawGraph();
  return {
    draw: drawGraph,
    dragging: () => drag !== null,
    capture: () => {
      pendingPosition = {
        selectedID: selected?.dataset.id, query: search?.value ?? saved.query ?? '', zoom,
        left: graph.hidden ? pendingPosition?.left ?? 0 : scroll?.scrollLeft ?? 0,
        top: graph.hidden ? pendingPosition?.top ?? 0 : scroll?.scrollTop ?? 0,
        detailsTop: graph.hidden ? pendingPosition?.detailsTop ?? 0 : details?.scrollTop ?? 0
      };
      return pendingPosition;
    },
    dispose: () => { observer.disconnect(); listeners.abort(); }
  };
}
