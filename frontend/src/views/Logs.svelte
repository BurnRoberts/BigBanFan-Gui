<script>
  import { onMount, onDestroy, tick, createEventDispatcher } from "svelte";
  import { SubscribeLogs, UnsubscribeLogs } from "../../wailsjs/go/main/App.js";
  import { logs } from "../stores/app.js";

  const dispatch = createEventDispatcher();
  let level = "info";
  let autoScroll = true;
  let logEl;
  // subscribed reflects the server-side state. Starts true because App.svelte
  // subscribes on connect. Pause/resume toggle it from here.
  let subscribed = true;

  // Custom scrollbar
  let sbThumb = /** @type {HTMLElement|null} */ (null);
  let sbTrack = /** @type {HTMLElement|null} */ (null);
  let dragging = false;
  let dragStartY = 0;
  let dragStartTop = 0;

  function updateThumb() {
    if (!logEl || !sbThumb || !sbTrack) return;
    const { scrollTop, scrollHeight, clientHeight } = logEl;
    if (scrollHeight <= clientHeight) {
      sbThumb.style.display = "none";
      return;
    }
    sbThumb.style.display = "block";
    const trackH = sbTrack.clientHeight;
    const thumbH = Math.max(30, (clientHeight / scrollHeight) * trackH);
    const thumbTop =
      (scrollTop / (scrollHeight - clientHeight)) * (trackH - thumbH);
    sbThumb.style.height = thumbH + "px";
    sbThumb.style.top = thumbTop + "px";
  }

  function onTermScroll() {
    updateThumb();
    // Automatically disable auto-scroll when the user scrolls up;
    // re-enable when they reach the bottom (within 40px threshold).
    if (logEl) {
      const atBottom =
        logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight < 40;
      autoScroll = atBottom;
    }
  }

  function onThumbMouseDown(e) {
    e.preventDefault();
    dragging = true;
    dragStartY = e.clientY;
    dragStartTop = parseFloat(sbThumb.style.top) || 0;
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
  }
  function onMouseMove(e) {
    if (!dragging || !logEl || !sbTrack || !sbThumb) return;
    const dy = e.clientY - dragStartY;
    const trackH = sbTrack.clientHeight;
    const thumbH = sbThumb.clientHeight;
    if (trackH <= thumbH) return; // guard: thumb fills track, no scroll range
    const newTop = Math.max(0, Math.min(trackH - thumbH, dragStartTop + dy));
    logEl.scrollTop =
      (newTop / (trackH - thumbH)) * (logEl.scrollHeight - logEl.clientHeight);
  }
  function onMouseUp() {
    dragging = false;
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("mouseup", onMouseUp);
  }

  let resizeObs;
  let unsubLog; // store unsubscribe handle
  let destroyed = false; // ML-4: flag so async onMount can abort if component unmounted mid-await

  // Pause stops the server from sending more lines.
  async function pause() {
    await UnsubscribeLogs();
    subscribed = false;
  }
  // Resume re-subscribes at the current level.
  async function resume() {
    await SubscribeLogs(level);
    subscribed = true;
  }
  function clearLog() {
    logs.set([]);
  }

  onMount(async () => {
    dispatch("read");
    await tick();
    // If the component was destroyed during the await (rare but possible),
    // abort here — otherwise the store subscription would leak.
    if (destroyed) return;
    resizeObs = new ResizeObserver(updateThumb);
    if (logEl) resizeObs.observe(logEl);
    updateThumb();

    // Auto-scroll via store subscription + rAF.
    // rAF fires AFTER the DOM has updated and AFTER any synchronous scroll
    // events (which set autoScroll=false) have already been processed.
    // This avoids the race condition where $logs updates beat the user's
    // own scroll event and snap the view back to the bottom.
    unsubLog = logs.subscribe(() => {
      requestAnimationFrame(() => {
        if (!autoScroll || !logEl) return;
        logEl.scrollTop = logEl.scrollHeight;
        updateThumb();
      });
    });
  });
  onDestroy(() => {
    destroyed = true;
    resizeObs?.disconnect();
    unsubLog?.();
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("mouseup", onMouseUp);
    // If user paused and then navigated away, re-subscribe so
    // logs keep collecting in the background.
    if (!subscribed) SubscribeLogs("info");
  });

  // Thumb stays in sync via updateThumb() calls in rAF + onTermScroll.

  function color(line) {
    if (line.includes("[error]") || line.includes("[ERROR]")) return "red";
    if (line.includes("[warn]") || line.includes("[WARN]")) return "yellow";
    return "dim";
  }

  async function changeLevel() {
    // Unsubscribe first so the server doesn't stack a second subscription.
    await UnsubscribeLogs();
    await SubscribeLogs(level);
    subscribed = true;
  }
</script>

<div class="page">
  <div class="toolbar">
    <h1>Log Stream</h1>
    <div class="spacer"></div>
    <label class="row">
      Level:
      <select bind:value={level} on:change={changeLevel}>
        <option value="info">info</option>
        <option value="warn">warn</option>
        <option value="error">error</option>
      </select>
    </label>
    <label class="row toggle">
      <input type="checkbox" bind:checked={autoScroll} />
      <span>Auto-scroll</span>
    </label>
    <button class="btn-sm" on:click={clearLog}>Clear</button>
    {#if subscribed}
      <button class="btn-sm active" on:click={pause}>⏸ Pause</button>
    {:else}
      <button class="btn-sm" on:click={resume}>▶ Resume</button>
    {/if}
  </div>

  <div class="scroll-host">
    <div class="terminal" bind:this={logEl} on:scroll={onTermScroll}>
      {#each $logs as line}
        <div class="line {color(line)}">{line}</div>
      {:else}
        <div class="line dim">
          Waiting for log lines… (LOG_SUBSCRIBE active)
        </div>
      {/each}
    </div>
    <div class="sb-track" bind:this={sbTrack}>
      <div
        class="sb-thumb"
        bind:this={sbThumb}
        on:mousedown={onThumbMouseDown}
      ></div>
    </div>
  </div>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 20px 28px;
    gap: 12px;
    overflow: hidden;
  }
  h1 {
    font-size: 18px;
    font-weight: 700;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }
  .spacer {
    flex: 1;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: #8b949e;
  }
  .toggle {
    cursor: pointer;
  }
  select {
    background: #0d1117;
    border: 1px solid #30363d;
    color: #e6edf3;
    border-radius: 4px;
    padding: 4px 6px;
    font-size: 12px;
    -webkit-appearance: none;
    appearance: none;
  }
  .btn-sm {
    background: #21262d;
    border: 1px solid #30363d;
    color: #8b949e;
    border-radius: 4px;
    padding: 4px 10px;
    font-size: 12px;
    cursor: pointer;
  }
  .btn-sm.active {
    border-color: #f97316;
    color: #f97316;
  }
  .scroll-host {
    flex: 1;
    display: flex;
    min-height: 0;
    border: 1px solid #21262d;
    border-radius: 8px;
    overflow: hidden;
    background: #010409;
  }
  .terminal {
    flex: 1;
    background: #010409;
    padding: 12px 16px;
    overflow-y: scroll;
    overflow-x: auto;
    scrollbar-width: none;
    min-height: 0;
    font-family: "Courier New", Consolas, monospace;
    font-size: 12px;
    line-height: 1.7;
  }
  .terminal::-webkit-scrollbar {
    display: none;
  }
  .sb-track {
    width: 10px;
    background: #010409;
    position: relative;
    flex-shrink: 0;
    border-radius: 0 8px 8px 0;
  }
  .sb-thumb {
    position: absolute;
    left: 1px;
    width: 8px;
    background: #30363d;
    border-radius: 2px;
    cursor: pointer;
    transition: background 0.15s;
    display: none;
  }
  .sb-thumb:hover {
    background: #484f58;
  }
  .line {
    white-space: pre-wrap;
    word-break: break-all;
    text-align: left;
  }
  .dim {
    color: #b1bac4;
  }
  .red {
    color: #f85149;
  }
  .yellow {
    color: #e3b341;
  }
</style>
