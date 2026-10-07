function initializeLineChart(figure, saved = {}) {
  const data = JSON.parse(figure.querySelector('.chart-data').textContent);
  const area = figure.querySelector('.chart-area');
  const readout = figure.querySelector('.chart-readout');
  const legend = figure.querySelector('.chart-legend');
  const table = figure.querySelector('.chart-table');
  const listeners = new AbortController();
  const xRange = Charts.domain(data.x);
  const yRange = Charts.domain(data.series.flatMap(series => series.values));
  let frame = null, crosshair = null, dots = [], selected = saved.selected ?? null;
  let pendingScroll = saved;
  let lastScroll = {left: saved.left || 0, top: saved.top || 0};
  area.hidden = false;
  readout.hidden = false;
  legend.hidden = data.series.length < 2;
  table.open = saved.open ?? false;
  area.setAttribute('role', 'group');
  area.setAttribute('aria-label', `${figure.querySelector('h2').textContent}. Use left and right arrows to inspect points, Home and End for first and last, Escape to clear. Data table follows.`);
  function inspect(index) {
    selected = index === null ? null : Math.max(0, Math.min(data.x.length - 1, index));
    if (!frame) return;
    crosshair.toggleAttribute('hidden', selected === null);
    dots.forEach(dot => dot.setAttribute('hidden', ''));
    if (selected === null) {
      readout.textContent = 'Hover or use arrow keys to inspect values.';
      return;
    }
    crosshair.setAttribute('x1', frame.x(data.x[selected]));
    crosshair.setAttribute('x2', frame.x(data.x[selected]));
    const values = [`${area.dataset.chartX}: ${Charts.format(data.x[selected])}`];
    data.series.forEach((series, i) => {
      const value = series.values[selected];
      if (value !== null) {
        dots[i].removeAttribute('hidden');
        dots[i].setAttribute('cx', frame.x(data.x[selected]));
        dots[i].setAttribute('cy', frame.y(value));
      }
      values.push(`${series.name}: ${value === null ? 'missing' : Charts.format(value)}`);
    });
    readout.textContent = values.join(' · ');
  }
  function draw() {
    if (!area.clientWidth) return;
    frame = Charts.frame(area, xRange, yRange, area.dataset.chartX);
    data.series.forEach((series, i) => {
      let path = '', connected = false;
      series.values.forEach((value, j) => {
        if (value === null) { connected = false; return; }
        path += `${connected ? 'L' : 'M'} ${frame.x(data.x[j])} ${frame.y(value)} `;
        connected = true;
      });
      frame.root.append(Charts.svg('path', {d: path, class: `chart-series series-${i}`}));
      series.values.forEach((value, j) => {
        if (value !== null) frame.root.append(Charts.svg('circle', {cx: frame.x(data.x[j]), cy: frame.y(value), r: 4, class: `chart-dot chart-point series-${i}`}));
      });
    });
    crosshair = Charts.svg('line', {y1: frame.plot.top, y2: frame.plot.bottom, class: 'chart-crosshair'});
    frame.root.append(crosshair);
    dots = data.series.map((_, i) => {
      const dot = Charts.svg('circle', {r: 4, class: `chart-dot series-${i}`});
      frame.root.append(dot);
      return dot;
    });
    inspect(selected);
  }
  area.addEventListener('pointermove', event => {
    if (!frame) return;
    const px = event.clientX - area.getBoundingClientRect().left;
    let nearest = 0, distance = Infinity;
    data.x.forEach((value, i) => {
      const next = Math.abs(frame.x(value) - px);
      if (next < distance) { nearest = i; distance = next; }
    });
    inspect(nearest);
  }, {signal: listeners.signal});
  area.addEventListener('pointerleave', () => { if (document.activeElement !== area) inspect(null); }, {signal: listeners.signal});
  area.addEventListener('keydown', event => {
    let index;
    if (event.key === 'ArrowRight') index = selected === null ? 0 : selected + 1;
    else if (event.key === 'ArrowLeft') index = selected === null ? data.x.length - 1 : selected - 1;
    else if (event.key === 'Home') index = 0;
    else if (event.key === 'End') index = data.x.length - 1;
    else if (event.key === 'Escape') index = null;
    else return;
    event.preventDefault();
    inspect(index);
  }, {signal: listeners.signal});
  const observer = new ResizeObserver(draw);
  observer.observe(area);
  draw();
  return {
    draw,
    capture: () => {
      const scroller = table.querySelector('.table-scroll');
      if (scroller.clientWidth) lastScroll = {left: scroller.scrollLeft, top: scroller.scrollTop};
      return {selected, open: table.open, ...lastScroll};
    },
    restoreScroll: () => {
      const scroller = table.querySelector('.table-scroll');
      if (pendingScroll && scroller.clientWidth) { scroller.scrollTo(pendingScroll.left || 0, pendingScroll.top || 0); pendingScroll = null; }
    },
    dispose: () => { listeners.abort(); observer.disconnect(); }
  };
}
