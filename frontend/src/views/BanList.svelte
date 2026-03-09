<script>
  import { onMount, onDestroy } from "svelte";
  import { GetBans, BanIP, UnbanIP } from "../../wailsjs/go/main/App.js";
  import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime.js";

  // Custom scrollbar
  let tableWrap;
  let sbThumb;
  let sbTrack;
  let dragging = false;
  let dragStartY = 0;
  let dragStartTop = 0;

  function updateThumb() {
    if (!tableWrap || !sbThumb || !sbTrack) return;
    const { scrollTop, scrollHeight, clientHeight } = tableWrap;
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

  function onWrapScroll() {
    updateThumb();
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
    if (!dragging || !tableWrap || !sbTrack || !sbThumb) return;
    const dy = e.clientY - dragStartY;
    const trackH = sbTrack.clientHeight;
    const thumbH = sbThumb.clientHeight;
    if (trackH <= thumbH) return; // guard: thumb fills track, no scroll range
    const newTop = Math.max(0, Math.min(trackH - thumbH, dragStartTop + dy));
    const ratio = newTop / (trackH - thumbH);
    tableWrap.scrollTop =
      ratio * (tableWrap.scrollHeight - tableWrap.clientHeight);
  }

  function onMouseUp() {
    dragging = false;
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("mouseup", onMouseUp);
  }

  let bans = [],
    total = 0,
    page = 1,
    pageSize = Number(localStorage.getItem("banPageSize")) || 25;
  $: localStorage.setItem("banPageSize", String(pageSize));
  let search = "",
    filterSource = "",
    activeOnly = true;
  let searchTimer;
  let loading = false;
  let error = "";
  let selected = new Set();
  let unbanning = false;
  let unbanProgress = "";
  let addIP = "";
  let addReason = "";
  let adding = false;

  async function load(resetPage = false) {
    if (resetPage) page = 1;
    loading = true;
    try {
      const result = await GetBans(
        page,
        pageSize,
        search,
        filterSource,
        activeOnly,
      );
      bans = result.bans ?? [];
      total = result.total ?? 0;
      selected = new Set(); // clear selection on page change
    } catch (e) {
      error = e.toString();
    } finally {
      loading = false;
    }
  }

  let resizeObs;
  onMount(() => {
    load();
    EventsOn("ban:new", () => load());
    EventsOn("ban:removed", () => load());
    // Keep thumb in sync when content changes height
    resizeObs = new ResizeObserver(updateThumb);
    if (tableWrap) resizeObs.observe(tableWrap);
  });
  onDestroy(() => {
    clearTimeout(searchTimer);
    EventsOff("ban:new");
    EventsOff("ban:removed");
    resizeObs?.disconnect();
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("mouseup", onMouseUp);
  });

  // Re-sync thumb after bans reload
  $: bans, setTimeout(updateThumb, 0);

  function onSearch() {
    clearTimeout(searchTimer);
    if (search.length === 0 || search.length >= 3) {
      searchTimer = setTimeout(() => load(true), 400);
    }
  }

  async function banAdd() {
    if (!addIP.trim()) return;
    adding = true;
    try {
      await BanIP(addIP.trim(), addReason.trim());
      addIP = "";
      addReason = "";
      await load();
    } catch (e) {
      error = e.toString();
    } finally {
      adding = false;
    }
  }

  async function unban(ip) {
    try {
      await UnbanIP(ip);
    } catch (e) {
      error = e.toString();
    }
  }

  async function unbanSelected() {
    const ips = [...selected];
    unbanning = true;
    error = "";
    const failed = [];
    for (let i = 0; i < ips.length; i++) {
      unbanProgress = `Unbanning ${i + 1} of ${ips.length}…`;
      try {
        await UnbanIP(ips[i]);
      } catch (e) {
        failed.push(ips[i]);
      }
    }
    unbanning = false;
    unbanProgress = "";
    if (failed.length) {
      error = `Failed to unban ${failed.length} IP(s): ${failed.join(", ")}`;
    }
    // Remove successfully unbanned IPs from selection regardless of partial failure.
    selected = new Set(failed);
    await load();
  }

  function toggleRow(ip) {
    const s = new Set(selected);
    s.has(ip) ? s.delete(ip) : s.add(ip);
    selected = s;
  }

  $: allChecked = bans.length > 0 && bans.every((b) => selected.has(b.ip));
  function toggleAll() {
    if (allChecked) selected = new Set();
    else selected = new Set(bans.map((b) => b.ip));
  }

  $: totalPages = Math.ceil(total / pageSize);
  function goPage(p) {
    page = p;
    load();
  }

  function fmt(ts) {
    if (!ts) return "—";
    return new Date(ts * 1000).toLocaleString();
  }
  function expiresIn(ts) {
    const diff = ts - Date.now() / 1000;
    if (diff <= 0) return "Expired";
    const h = Math.floor(diff / 3600);
    const m = Math.floor((diff % 3600) / 60);
    return h > 0 ? `${h}h` : `${m}m`;
  }
</script>

<div class="page">
  <div class="toolbar">
    <h1>
      Ban List <span class="muted small"
        >({total.toLocaleString()} matching)</span
      >
    </h1>
    <div class="spacer"></div>
    {#if selected.size > 0}
      <button class="btn-danger" on:click={unbanSelected} disabled={unbanning}>
        {unbanning ? unbanProgress : `Unban ${selected.size} selected`}
      </button>
    {/if}
  </div>

  <div class="filters">
    <input
      class="search"
      bind:value={search}
      on:input={onSearch}
      placeholder="🔍 Search IPs… (min 3 chars)"
    />
    <input
      bind:value={filterSource}
      on:change={() => load(true)}
      placeholder="Filter by source node"
      title="Filter by the node that submitted the ban"
      style="width:180px"
    />
    <label
      class="toggle"
      title="When checked, hides bans that have already expired"
    >
      <input
        type="checkbox"
        bind:checked={activeOnly}
        on:change={() => load(true)}
      />
      <span>Hide expired</span>
    </label>
    <div class="add-row">
      <input
        bind:value={addIP}
        placeholder="IP or CIDR to ban…"
        on:keydown={(e) => e.key === "Enter" && banAdd()}
        style="width:190px"
      />
      <input
        bind:value={addReason}
        placeholder="Reason (optional)"
        maxlength="1024"
        style="width:200px"
      />
      <button
        class="btn-orange"
        on:click={banAdd}
        disabled={adding || !addIP.trim()}
      >
        {adding ? "…" : "+ Ban"}
      </button>
    </div>
  </div>

  {#if error}<div class="err">{error}</div>{/if}

  <div class="scroll-host">
    <div class="table-wrap" bind:this={tableWrap} on:scroll={onWrapScroll}>
      <table>
        <thead>
          <tr>
            <th
              ><input
                type="checkbox"
                checked={allChecked}
                on:change={toggleAll}
              /></th
            >
            <th>IP / CIDR</th>
            <th>Reason</th>
            <th>Banned at</th>
            <th>Expires in</th>
            <th>Source</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#if loading}
            <tr><td colspan="7" class="center muted">Loading…</td></tr>
          {:else if !bans.length}
            <tr><td colspan="7" class="center muted">No bans found</td></tr>
          {:else}
            {#each bans as ban}
              <tr class:sel={selected.has(ban.ip)}>
                <td
                  ><input
                    type="checkbox"
                    checked={selected.has(ban.ip)}
                    on:change={() => toggleRow(ban.ip)}
                  /></td
                >
                <td class="ip mono">{ban.ip}</td>
                <td class="reason muted small" title={ban.reason || ""}
                  >{ban.reason || "—"}</td
                >
                <td class="muted small">{fmt(ban.banned_at)}</td>
                <td
                  class="small"
                  class:expired={ban.expires_at < Date.now() / 1000}
                  >{expiresIn(ban.expires_at)}</td
                >
                <td class="muted small">{ban.source}</td>
                <td
                  ><button class="unban-btn" on:click={() => unban(ban.ip)}
                    >Unban</button
                  ></td
                >
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
    <div class="sb-track" bind:this={sbTrack}>
      <div
        class="sb-thumb"
        bind:this={sbThumb}
        on:mousedown={onThumbMouseDown}
      ></div>
    </div>
  </div>

  {#if total > 0}
    <div class="pagination">
      <button on:click={() => goPage(page - 1)} disabled={page <= 1}>←</button>
      <span class="muted small">Page {page} of {totalPages}</span>
      <button on:click={() => goPage(page + 1)} disabled={page >= totalPages}
        >→</button
      >
      <select bind:value={pageSize} on:change={() => load(true)}>
        <option value={25}>25 / page</option>
        <option value={50}>50 / page</option>
        <option value={100}>100 / page</option>
      </select>
    </div>
  {/if}
</div>

<style>
  .page {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    flex-direction: column;
    padding: 20px 28px;
    gap: 12px;
    overflow: hidden;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  h1 {
    font-size: 18px;
    font-weight: 700;
  }
  .small {
    font-size: 12px;
  }
  .muted {
    color: #8b949e;
  }
  .spacer {
    flex: 1;
  }
  .filters {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }
  .search {
    flex: 1;
    min-width: 200px;
  }
  input:not([type="checkbox"]):not([type="number"]),
  .search {
    background: #0d1117;
    border: 1px solid #30363d;
    border-radius: 6px;
    color: #e6edf3;
    padding: 7px 10px;
    font-size: 13px;
  }
  input:focus {
    outline: none;
    border-color: #f97316;
  }
  /* ── Custom themed checkboxes ── */
  input[type="checkbox"] {
    -webkit-appearance: none;
    appearance: none;
    width: 15px;
    height: 15px;
    border: 1.5px solid #3d4b5c;
    border-radius: 3px;
    background: #0d1117;
    cursor: pointer;
    flex-shrink: 0;
    vertical-align: middle;
    transition:
      background 0.15s,
      border-color 0.15s;
  }
  input[type="checkbox"]:hover {
    border-color: #f97316;
  }
  input[type="checkbox"]:checked {
    background: #f97316;
    border-color: #f97316;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cpath fill='none' stroke='%23000' stroke-width='2.5' stroke-linecap='round' stroke-linejoin='round' d='M3.5 8l3 3 6-6'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: center;
    background-size: 11px;
  }
  input[type="checkbox"]:focus {
    outline: none;
    border-color: #f97316;
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: 5px;
    font-size: 12px;
    color: #8b949e;
    cursor: pointer;
  }
  .add-row {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .btn-orange {
    background: #f97316;
    color: #000;
    border: none;
    border-radius: 6px;
    padding: 7px 14px;
    font-weight: 700;
    cursor: pointer;
    font-size: 13px;
  }
  .btn-orange:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .btn-danger {
    background: #2d1b1b;
    border: 1px solid #f85149;
    color: #f85149;
    border-radius: 6px;
    padding: 7px 14px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }
  .btn-danger:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .err {
    background: #2d1b1b;
    border: 1px solid #f85149;
    border-radius: 6px;
    padding: 8px 12px;
    color: #f85149;
    font-size: 12px;
  }
  .scroll-host {
    flex: 1;
    display: flex;
    min-height: 0;
    background: #161b22;
    border: 1px solid #21262d;
    border-radius: 8px;
    overflow: hidden;
    position: relative;
  }
  .table-wrap {
    flex: 1;
    overflow-y: scroll;
    overflow-x: auto;
    scrollbar-width: none; /* Firefox */
    min-height: 0;
  }
  .table-wrap::-webkit-scrollbar {
    display: none; /* Chrome/WebKit */
  }
  .sb-track {
    width: 10px;
    background: #161b22;
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
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  thead th {
    background: #1c2128;
    padding: 10px 12px;
    text-align: left;
    font-size: 11px;
    color: #8b949e;
    text-transform: uppercase;
    position: sticky;
    top: 0;
    border-bottom: 1px solid #21262d;
  }
  tbody tr {
    border-bottom: 1px solid #21262d;
    transition: background 0.1s;
  }
  tbody tr:hover {
    background: #1c2128;
  }
  tbody tr.sel {
    background: #1f2937;
  }
  td {
    padding: 9px 12px;
  }
  .ip {
    font-family: "Courier New", monospace;
    font-weight: 600;
    color: #e6edf3;
  }
  .mono {
    font-family: monospace;
  }
  .reason {
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .center {
    text-align: center;
    padding: 24px;
  }
  .expired {
    color: #8b949e;
  }
  .unban-btn {
    background: none;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #8b949e;
    cursor: pointer;
    padding: 3px 8px;
    font-size: 11px;
  }
  .unban-btn:hover {
    border-color: #f85149;
    color: #f85149;
  }
  .pagination {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .pagination button {
    background: #21262d;
    border: 1px solid #30363d;
    color: #e6edf3;
    border-radius: 4px;
    padding: 5px 10px;
    cursor: pointer;
  }
  .pagination button:disabled {
    opacity: 0.3;
  }
  .pagination select {
    background: #0d1117;
    border: 1px solid #30363d;
    color: #e6edf3;
    border-radius: 4px;
    padding: 4px 6px;
    font-size: 12px;
    -webkit-appearance: none;
    appearance: none;
  }
</style>
