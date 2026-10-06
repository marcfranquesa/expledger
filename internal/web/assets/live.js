function initializeLive(applySnapshot) {
  const status = document.querySelector('.refresh-status');
  async function refresh() {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    try {
      const response = await fetch('/snapshot', {cache: 'no-store', signal: controller.signal});
      if (!response.ok) throw new Error((await response.text()).trim() || 'Snapshot unavailable');
      const next = new DOMParser().parseFromString(await response.text(), 'text/html');
      if (applySnapshot(next) === false) return;
      status.hidden = true;
    } catch (error) {
      status.textContent = `Refresh unavailable. Showing the last successful update; retrying shortly. ${error.message}`;
      status.hidden = false;
    } finally {
      clearTimeout(timeout);
      setTimeout(refresh, 3000);
    }
  }
  setTimeout(refresh, 3000);
}
