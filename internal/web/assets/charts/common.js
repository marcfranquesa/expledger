// Plot primitives share geometry and formatting; visual identity lives in CSS.
const Charts = (() => {
  function svg(tag, attributes = {}, text) {
    const node = document.createElementNS('http://www.w3.org/2000/svg', tag);
    Object.entries(attributes).forEach(([key, value]) => node.setAttribute(key, value));
    if (text !== undefined) node.textContent = text;
    return node;
  }
  function domain(values) {
    let min = Infinity, max = -Infinity;
    values.forEach(value => {
      if (Number.isFinite(value)) { min = Math.min(min, value); max = Math.max(max, value); }
    });
    return Number.isFinite(min) ? {min, max} : {min: 0, max: 1};
  }
  function fraction(value, range) {
    if (range.min === range.max) return .5;
    const span = range.max - range.min;
    // Halving first prevents overflow for a domain spanning extreme finite values.
    return Number.isFinite(span) ? (value - range.min) / span : (value / 2 - range.min / 2) / (range.max / 2 - range.min / 2);
  }
  function ticks(range, count) {
    if (range.min === range.max) return [range.min];
    return Array.from({length: count}, (_, i) => {
      const t = i / (count - 1);
      return range.min * (1 - t) + range.max * t;
    });
  }
  function format(value) {
    if (value === 0) return '0';
    if (Math.abs(value) >= 1e5 || Math.abs(value) < 1e-3) return value.toExponential(2).replace(/\.0+(?=e)/, '');
    return Number(value.toPrecision(4)).toString();
  }
  function frame(area, xRange, yRange, xLabel) {
    const width = area.clientWidth;
    if (!width) return null;
    const height = area.clientHeight;
    const plot = {left: 62, top: 14, right: Math.max(78, width - 12), bottom: height - 42};
    const root = svg('svg', {viewBox: `0 0 ${width} ${height}`, 'aria-hidden': 'true'});
    const x = value => plot.left + fraction(value, xRange) * (plot.right - plot.left);
    const y = value => plot.bottom - fraction(value, yRange) * (plot.bottom - plot.top);
    ticks(yRange, 5).forEach(value => {
      root.append(svg('line', {x1: plot.left, x2: plot.right, y1: y(value), y2: y(value), class: 'chart-grid'}));
      root.append(svg('text', {x: plot.left - 10, y: y(value) + 4, 'text-anchor': 'end', class: 'chart-label'}, format(value)));
    });
    ticks(xRange, width < 360 ? 3 : 5).forEach(value => {
      root.append(svg('text', {x: x(value), y: plot.bottom + 18, 'text-anchor': 'middle', class: 'chart-label'}, format(value)));
    });
    root.append(svg('text', {x: (plot.left + plot.right) / 2, y: height - 4, 'text-anchor': 'middle', class: 'chart-label'}, xLabel));
    area.replaceChildren(root);
    return {root, plot, x, y};
  }
  return {svg, domain, fraction, format, frame};
})();
