<script>
  import { onMount, onDestroy } from "svelte";
  import { GetStatus, GetStats, GetPeers } from "../../wailsjs/go/main/App.js";
  import { status, stats, peers } from "../stores/app.js";
  import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime.js";

  let loading = true;
  let error = "";
  let destroyed = false;
  // Rolling ban chart: last 60 entries (one per tick ~5s = 5min window)
  let banHistory = Array(60).fill(0);
  let lastBans = 0;

  async function refresh() {
    if (destroyed) return;
    try {
      const [s, st, p] = await Promise.all([
        GetStatus(),
        GetStats(),
        GetPeers(),
      ]);
      // Guard again: component may have been destroyed during the await.
      if (destroyed) return;
      status.set(s);
      stats.set(st);
      peers.set(p?.peers ?? []);
      // Update ban rate chart.
      const delta = (st?.bans_this_session ?? 0) - lastBans;
      lastBans = st?.bans_this_session ?? 0;
      banHistory = [...banHistory.slice(1), delta];
      error = "";
    } catch (e) {
      if (!destroyed) error = e.toString();
    } finally {
      if (!destroyed) loading = false;
    }
  }

  let interval;
  onMount(() => {
    refresh();
    interval = setInterval(refresh, 5000);
    EventsOn("ban:new", refresh);
    EventsOn("peer:up", refresh);
    EventsOn("peer:down", refresh);
  });
  onDestroy(() => {
    destroyed = true;
    clearInterval(interval);
    EventsOff("ban:new");
    EventsOff("peer:up");
    EventsOff("peer:down");
  });

  function uptime(sec) {
    if (!sec) return "—";
    const h = Math.floor(sec / 3600),
      m = Math.floor((sec % 3600) / 60);
    return `${h}h ${m}m`;
  }

  // Unique peer nodes (deduped by node_id — raw data can have both in+out for same node)
  $: uniquePeers = (() => {
    const seen = new Map();
    for (const p of $peers ?? []) {
      const id = p.node_id || p.addr;
      if (!seen.has(id)) seen.set(id, p);
      else if (p.direction === "outbound") seen.set(id, p); // prefer outbound entry
    }
    return [...seen.values()];
  })();

  $: outboundCount = ($peers ?? []).filter(
    (p) => p.direction === "outbound",
  ).length;
  $: inboundCount = ($peers ?? []).filter(
    (p) => p.direction === "inbound",
  ).length;

  $: maxBar = Math.max(...banHistory, 1);
</script>

<div class="page">
  <h1>Dashboard</h1>
  {#if loading}<div class="muted">Loading…</div>
  {:else if error}<div class="err">{error}</div>
  {:else}
    <div class="cards">
      <div class="stat-card">
        <div class="label">Node</div>
        <div class="val">{$status?.node_id ?? "—"}</div>
        <div class="sub2">
          v{$status?.version ?? "?"} · up {uptime($status?.uptime_sec)}
        </div>
      </div>
      <div class="stat-card orange">
        <div class="label">Active Bans</div>
        <div class="val">{($status?.ban_count ?? 0).toLocaleString()}</div>
        <div class="sub2">+{$stats?.bans_this_session ?? 0} this session</div>
      </div>
      <div class="stat-card">
        <div class="label">Peers</div>
        <div class="val">{uniquePeers.length}</div>
        <div class="sub2">{outboundCount} out · {inboundCount} in</div>
      </div>
      <div class="stat-card">
        <div class="label">Unbans</div>
        <div class="val">{$stats?.unbans_this_session ?? 0}</div>
        <div class="sub2">
          Scan detects: {$stats?.scan_detects_this_session ?? 0} · TCP accepts: {$stats?.connections_accepted ??
            0}
        </div>
      </div>
    </div>

    <div class="section">
      <h2>Ban rate <span class="muted small">(last 5 min)</span></h2>
      <div class="chart">
        {#each banHistory as val}
          <div
            class="bar"
            style="height:{Math.round((val / maxBar) * 100)}%"
            title="{val} bans"
          >
            {#if val > 0}<span class="bar-tip">{val}</span>{/if}
          </div>
        {/each}
      </div>
    </div>

    <div class="section">
      <h2>Peer nodes</h2>
      <div class="peer-list">
        {#each uniquePeers as p}
          <div class="peer">
            <span class="dot {p.connected ? 'green' : 'red'}"></span>
            <span class="peer-id">{p.node_id || "…"}</span>
            <span class="peer-addr muted">{p.addr}</span>
            <span class="peer-dir muted">{p.direction}</span>
          </div>
        {:else}<div class="muted">No peers registered</div>
        {/each}
      </div>
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
    padding: 28px 32px;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: #30363d #0d1117;
  }
  .page::-webkit-scrollbar {
    width: 6px;
  }
  .page::-webkit-scrollbar-track {
    background: #0d1117;
    border-radius: 3px;
  }
  .page::-webkit-scrollbar-thumb {
    background: #30363d;
    border-radius: 3px;
  }
  .page::-webkit-scrollbar-thumb:hover {
    background: #484f58;
  }
  h1 {
    font-size: 20px;
    font-weight: 700;
    margin-bottom: 20px;
  }
  h2 {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 12px;
    color: #e6edf3;
  }
  .muted {
    color: #8b949e;
  }
  .small {
    font-size: 12px;
  }
  .err {
    color: #f85149;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 14px;
    margin-bottom: 28px;
  }
  .stat-card {
    background: #161b22;
    border: 1px solid #21262d;
    border-radius: 10px;
    padding: 18px 20px;
  }
  .stat-card.orange {
    border-color: #f97316;
  }
  .label {
    font-size: 11px;
    color: #8b949e;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-bottom: 6px;
  }
  .val {
    font-size: 28px;
    font-weight: 700;
  }
  .stat-card.orange .val {
    color: #f97316;
  }
  .sub2 {
    font-size: 11px;
    color: #8b949e;
    margin-top: 4px;
  }
  .section {
    background: #161b22;
    border: 1px solid #21262d;
    border-radius: 10px;
    padding: 20px;
    margin-bottom: 20px;
  }
  .chart {
    display: flex;
    align-items: flex-end;
    gap: 3px;
    height: 80px;
  }
  .bar {
    flex: 1;
    background: #f97316;
    border-radius: 3px 3px 0 0;
    min-height: 2px;
    position: relative;
    transition: height 0.3s;
  }
  .bar-tip {
    position: absolute;
    top: -18px;
    left: 50%;
    transform: translateX(-50%);
    font-size: 10px;
    color: #f97316;
    white-space: nowrap;
  }
  .peer-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .peer {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .dot.green {
    background: #3fb950;
    box-shadow: 0 0 6px #3fb950;
  }
  .dot.red {
    background: #f85149;
    box-shadow: 0 0 6px #f85149;
  }
  .peer-id {
    font-weight: 600;
    min-width: 80px;
  }
  .peer-addr {
    font-size: 12px;
    font-family: monospace;
  }
  .peer-dir {
    font-size: 11px;
    margin-left: auto;
  }
</style>
