<script>
  import { onMount, onDestroy } from "svelte";
  import { GetPeers, GetStatus } from "../../wailsjs/go/main/App.js";
  import { Connect as GoConnect } from "../../wailsjs/go/main/App.js";
  import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime.js";
  import { peers, activeProfile, connected } from "../stores/app.js";

  let loading = true;
  let error = "";
  let switching = "";
  let managedNodeId = "";

  // Raw peer list (all entries, not deduped)
  let rawPeers = [];

  // Deduped for the card list — one card per node_id, with combined direction label
  let peerCards = [];

  // Topology nodes — one entry per node_id with a Set of directions
  let topoNodes = [];

  async function load() {
    try {
      const result = await GetPeers();
      rawPeers = result?.peers ?? [];
      buildCards();
      buildTopo();
    } catch (e) {
      error = e.toString();
    } finally {
      loading = false;
    }
  }

  function buildCards() {
    // Dedup by node_id. If same node appears in+out, show one card with combined dir.
    const map = new Map();
    for (const p of rawPeers) {
      const id = p.node_id || p.addr;
      if (!map.has(id)) {
        map.set(id, { ...p, directions: new Set([p.direction]) });
      } else {
        const c = map.get(id);
        c.directions.add(p.direction);
        // Prefer outbound addr (has the configured hostname for dialing)
        if (p.direction === "outbound") {
          c.addr = p.addr;
          c.direction = "outbound";
        }
        // Mark connected if any connection is up
        if (p.connected) c.connected = true;
      }
    }
    peerCards = [...map.values()];
    peers.set(peerCards);
  }

  function buildTopo() {
    // One topo node per node_id, collecting ALL directions
    const map = new Map();
    for (const p of rawPeers) {
      const id = p.node_id || p.addr;
      if (!map.has(id)) {
        map.set(id, {
          node_id: p.node_id || id,
          directions: new Set([p.direction]),
          connected: p.connected,
          outAddr: null,
        });
      } else {
        const n = map.get(id);
        n.directions.add(p.direction);
        if (p.connected) n.connected = true;
      }
      if (p.direction === "outbound") map.get(id).outAddr = p.addr;
    }
    topoNodes = [...map.values()];
  }

  async function loadStatus() {
    try {
      const s = await GetStatus();
      managedNodeId = s?.node_id || $activeProfile?.name || "?";
    } catch (_) {
      managedNodeId = $activeProfile?.name || "?";
    }
  }

  onMount(() => {
    load();
    loadStatus();
    EventsOn("peer:up", load);
    EventsOn("peer:down", load);
  });
  onDestroy(() => {
    EventsOff("peer:up");
    EventsOff("peer:down");
  });

  function fmt(ts) {
    if (!ts) return "never";
    const diff = Math.round(Date.now() / 1000 - ts);
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    return `${Math.floor(diff / 3600)}h ago`;
  }

  function parseHost(addr) {
    if (!addr) return null;
    if (addr.startsWith("[")) {
      const e = addr.lastIndexOf("]");
      return e > 0 ? addr.slice(1, e) : null;
    }
    const l = addr.lastIndexOf(":");
    return l >= 0 ? addr.slice(0, l) : addr;
  }

  function bestHost(card) {
    // Prefer outbound addr (has the configured hostname the node dials from).
    if (card.directions?.has("outbound") || card.direction === "outbound")
      return parseHost(card.addr);
    // Inbound-only: addr is the source IP:ephemeral_port they connected FROM.
    // That IP is the real IP of the remote node — connect to it on mgmt port.
    return parseHost(card.addr) || null;
  }

  async function switchTo(card) {
    error = "";
    const prof = $activeProfile;
    if (!prof) {
      error = "Not connected";
      return;
    }
    const host = bestHost(card);
    if (!host) {
      error = `Cannot resolve address for ${card.node_id || card.addr}`;
      return;
    }
    switching = card.node_id || card.addr;
    try {
      const newProf = {
        name: card.node_id || host,
        host,
        port: prof.port ?? 7779,
        key_hex: prof.key_hex,
      };
      await GoConnect(newProf);
      activeProfile.set(newProf);
    } catch (e) {
      error = `Switch to ${host}:7779 failed: ${e}`;
    } finally {
      switching = "";
    }
  }

  function dirLabel(card) {
    const d = card.directions || new Set([card.direction]);
    if (d.has("inbound") && d.has("outbound")) return "both";
    if (d.has("outbound")) return "→ out";
    return "← in";
  }
  function dirClass(card) {
    const d = card.directions || new Set([card.direction]);
    if (d.has("inbound") && d.has("outbound")) return "dir-both";
    if (d.has("outbound")) return "dir-out";
    return "dir-in";
  }

  // ── Topology geometry ──────────────────────────────────────────────────────
  const CX_ELLIPSE = 300,
    CY_ELLIPSE = 140;
  const YOU = { x: 60, y: 45 };

  // Layout mode: 'ellipse' or 'grid'
  let topoMode = localStorage.getItem("topoMode") || "ellipse";
  $: localStorage.setItem("topoMode", topoMode);

  // ── Ellipse mode ──────────────────────────────────────────────────────────
  // Auto-scale radii so nodes don't overlap as cluster grows.
  // Minimum: 175×100. Each extra node adds a bit.
  $: ellipseRx = Math.max(175, topoNodes.length * 22);
  $: ellipseRy = Math.max(100, topoNodes.length * 13);
  $: CX = topoMode === "ellipse" ? CX_ELLIPSE : 0; // unused in grid
  // svgH/svgW/viewBox only used by the ellipse SVG — tree has its own viewBox.
  $: svgH = Math.max(280, CY_ELLIPSE + ellipseRy + 60);
  $: svgW = Math.max(600, (CX_ELLIPSE + ellipseRx + 50) * 2);
  $: viewBox = `0 0 ${svgW} ${svgH}`;

  function ellipsePos(i, total) {
    const angle = -Math.PI / 2 + (i / Math.max(total, 1)) * 2 * Math.PI;
    return {
      x: CX_ELLIPSE + ellipseRx * Math.cos(angle),
      y: CY_ELLIPSE + ellipseRy * Math.sin(angle),
    };
  }

  // ── Tree (list) mode ──────────────────────────────────────────────────────
  const TREE_PY0 = 130; // Y of first peer row
  const TREE_ROW_H = 76; // vertical spacing per peer
  const TREE_MX = 90; // managed node X
  const TREE_MY = 60; // managed node Y
  const TREE_PEER_X = 290; // peer circle center X

  $: treeSvgH = Math.max(220, TREE_PY0 + topoNodes.length * TREE_ROW_H + 30);
  $: treeTrunkEnd =
    topoNodes.length > 0
      ? TREE_PY0 + (topoNodes.length - 1) * TREE_ROW_H
      : TREE_MY + 32;

  // peerPos and managedPos only used by ellipse mode.
  function peerPos(i, total) {
    return ellipsePos(i, total);
  }
  $: managedPos = { x: CX_ELLIPSE, y: CY_ELLIPSE };

  // Offset a line endpoint slightly perpendicular for double-line visual
  function offsetLine(x1, y1, x2, y2, dx, dy) {
    const len = Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2) || 1;
    const nx = -(y2 - y1) / len,
      ny = (x2 - x1) / len;
    return {
      x1: x1 + nx * dx,
      y1: y1 + ny * dy,
      x2: x2 + nx * dx,
      y2: y2 + ny * dy,
    };
  }

  // Status color: green=all up, yellow=some up (degraded), red=all down
  function nodeColor(n) {
    if (!n.connected) return "#f85149";
    return "#3fb950";
  }

  // Managed node ring: reflects overall cluster health from our perspective
  $: managedColor = (() => {
    if (!$connected) return "#f85149";
    if (!topoNodes.length) return "#3fb950";
    const up = topoNodes.filter((n) => n.connected).length;
    if (up === topoNodes.length) return "#3fb950"; // all peers up
    if (up > 0) return "#e3b341"; // some peers down
    return "#f85149"; // all peers down
  })();
</script>

<div class="page">
  <h1>Peer Nodes</h1>
  {#if error}<div class="err">{error}</div>{/if}

  <!-- ── Peer cards ────────────────────────────────────────────────────── -->
  <div class="section">
    <div class="peer-grid">
      {#if loading}
        <div class="muted center col-all">Loading…</div>
      {:else if !peerCards.length}
        <div class="muted center col-all">No peers connected</div>
      {:else}
        {#each peerCards as card}
          <div class="peer-card" class:connected={card.connected}>
            <div class="peer-top">
              <span
                class="dot"
                class:green={card.connected}
                class:red={!card.connected}
              ></span>
              <span class="node-id">{card.node_id || "unknown"}</span>
              <span class="dir {dirClass(card)}">{dirLabel(card)}</span>
            </div>
            <div class="peer-addr mono">{card.addr}</div>
            <div class="peer-meta muted">Last seen: {fmt(card.last_seen)}</div>
            <button
              class="switch-btn"
              class:is-loading={switching === (card.node_id || card.addr)}
              disabled={!bestHost(card) || !!switching}
              on:click={() => switchTo(card)}
              title="Connect GUI → {bestHost(card) || card.node_id}:7779"
            >
              {switching === (card.node_id || card.addr)
                ? "Switching…"
                : "Switch →"}
            </button>
          </div>
        {/each}
      {/if}
    </div>
  </div>

  <!-- ── Topology ──────────────────────────────────────────────────────── -->
  <div class="section topology">
    <h2>
      Topology
      <span class="legend">(from {managedNodeId}'s perspective)</span>
      <div class="mode-select-wrap">
        <select class="mode-select" bind:value={topoMode}>
          <option value="ellipse">Ellipse</option>
          <option value="tree">Tree (list)</option>
        </select>
        <span class="mode-chevron">▾</span>
      </div>
    </h2>
    <div class="topo-canvas">
      {#if topoMode === "ellipse"}
        <svg
          {viewBox}
          width="100%"
          height={svgH}
          xmlns="http://www.w3.org/2000/svg"
        >
          <defs>
            <marker
              id="a-out"
              markerWidth="6"
              markerHeight="6"
              refX="5"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L6,3 z" fill="#f97316" />
            </marker>
            <marker
              id="a-in"
              markerWidth="6"
              markerHeight="6"
              refX="5"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L6,3 z" fill="#58a6ff" />
            </marker>
            <marker
              id="a-you"
              markerWidth="6"
              markerHeight="6"
              refX="5"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L6,3 z" fill="#a78bfa" />
            </marker>
            <marker
              id="a-disc"
              markerWidth="6"
              markerHeight="6"
              refX="5"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L6,3 z" fill="#30363d" />
            </marker>
          </defs>

          <!-- YOU → managed node -->
          {#if topoMode === "ellipse"}
            <line
              x1={YOU.x + 30}
              y1={YOU.y + 5}
              x2={managedPos.x - 32}
              y2={managedPos.y - 18}
              stroke="#a78bfa"
              stroke-width="1.5"
              marker-end="url(#a-you)"
            />
          {/if}

          <!-- Peer connection lines -->
          {#each topoNodes as n, i}
            {@const pos = peerPos(i, topoNodes.length)}
            {#if n.directions.has("outbound") && n.directions.has("inbound")}
              {@const lo = offsetLine(
                managedPos.x,
                managedPos.y,
                pos.x,
                pos.y,
                4,
                4,
              )}
              {@const li = offsetLine(
                pos.x,
                pos.y,
                managedPos.x,
                managedPos.y,
                4,
                4,
              )}
              <line
                x1={lo.x1}
                y1={lo.y1}
                x2={lo.x2}
                y2={lo.y2}
                stroke={n.connected ? "#f97316" : "#30363d"}
                stroke-width="1.5"
                marker-end={n.connected ? "url(#a-out)" : "url(#a-disc)"}
              />
              <line
                x1={li.x1}
                y1={li.y1}
                x2={li.x2}
                y2={li.y2}
                stroke={n.connected ? "#58a6ff" : "#30363d"}
                stroke-width="1.5"
                marker-end={n.connected ? "url(#a-in)" : "url(#a-disc)"}
              />
            {:else if n.directions.has("outbound")}
              <line
                x1={managedPos.x}
                y1={managedPos.y}
                x2={pos.x}
                y2={pos.y}
                stroke={n.connected ? "#f97316" : "#30363d"}
                stroke-width="2"
                stroke-dasharray={n.connected ? "" : "5,3"}
                marker-end={n.connected ? "url(#a-out)" : "url(#a-disc)"}
              />
            {:else}
              <line
                x1={pos.x}
                y1={pos.y}
                x2={managedPos.x}
                y2={managedPos.y}
                stroke={n.connected ? "#58a6ff" : "#30363d"}
                stroke-width="2"
                stroke-dasharray={n.connected ? "" : "5,3"}
                marker-end={n.connected ? "url(#a-in)" : "url(#a-disc)"}
              />
            {/if}
          {/each}

          <!-- Peer node circles -->
          {#each topoNodes as n, i}
            {@const pos = peerPos(i, topoNodes.length)}
            <circle
              cx={pos.x}
              cy={pos.y}
              r="28"
              fill="#1c2128"
              stroke={nodeColor(n)}
              stroke-width="2.5"
            />
            <text
              x={pos.x}
              y={pos.y - 2}
              text-anchor="middle"
              fill={n.connected ? "#e6edf3" : "#8b949e"}
              font-size="10"
              font-weight="600"
              font-family="Inter,system-ui,sans-serif">{n.node_id || "?"}</text
            >
            <text
              x={pos.x}
              y={pos.y + 11}
              text-anchor="middle"
              font-size="8"
              font-family="Inter,system-ui,sans-serif"
              fill={n.directions.has("inbound") && n.directions.has("outbound")
                ? "#e3b341"
                : n.directions.has("outbound")
                  ? "#f97316"
                  : "#58a6ff"}
            >
              {n.directions.has("inbound") && n.directions.has("outbound")
                ? "both"
                : n.directions.has("outbound")
                  ? "outbound"
                  : "inbound"}</text
            >
          {/each}

          <!-- Managed node -->
          <circle
            cx={managedPos.x}
            cy={managedPos.y}
            r="32"
            fill="#1e2937"
            stroke={managedColor}
            stroke-width="2.5"
          />
          <text
            x={managedPos.x}
            y={managedPos.y - 3}
            text-anchor="middle"
            fill={managedColor}
            font-size="11"
            font-weight="bold"
            font-family="Inter,system-ui,sans-serif">{managedNodeId}</text
          >
          <text
            x={managedPos.x}
            y={managedPos.y + 11}
            text-anchor="middle"
            fill="#8b949e"
            font-size="8"
            font-family="Inter,system-ui,sans-serif">managed</text
          >

          <!-- YOU node (diamond) -->
          {#if topoMode === "ellipse"}
            <polygon
              points="{YOU.x},{YOU.y - 28} {YOU.x + 28},{YOU.y} {YOU.x},{YOU.y +
                28} {YOU.x - 28},{YOU.y}"
              fill="#1a1330"
              stroke="#a78bfa"
              stroke-width="2"
            />
            <text
              x={YOU.x}
              y={YOU.y - 1}
              text-anchor="middle"
              dominant-baseline="middle"
              fill="#a78bfa"
              font-size="9"
              font-weight="bold"
              font-family="Inter,system-ui,sans-serif">YOU</text
            >
            <text
              x={YOU.x}
              y={YOU.y + 10}
              text-anchor="middle"
              dominant-baseline="middle"
              fill="#8b949e"
              font-size="7"
              font-family="Inter,system-ui,sans-serif">GUI</text
            >
          {/if}

          <!-- Legend -->
          <line
            x1="20"
            y1={svgH - 18}
            x2="48"
            y2={svgH - 18}
            stroke="#58a6ff"
            stroke-width="1.5"
            marker-end="url(#a-in)"
          />
          <text
            x="54"
            y={svgH - 14}
            fill="#8b949e"
            font-size="9"
            font-family="Inter,system-ui,sans-serif">inbound</text
          >
          <line
            x1="120"
            y1={svgH - 18}
            x2="148"
            y2={svgH - 18}
            stroke="#f97316"
            stroke-width="1.5"
            marker-end="url(#a-out)"
          />
          <text
            x="154"
            y={svgH - 14}
            fill="#8b949e"
            font-size="9"
            font-family="Inter,system-ui,sans-serif">outbound</text
          >
          <line
            x1="230"
            y1={svgH - 18}
            x2="258"
            y2={svgH - 18}
            stroke="#a78bfa"
            stroke-width="1.5"
            marker-end="url(#a-you)"
          />
          <text
            x="264"
            y={svgH - 14}
            fill="#8b949e"
            font-size="9"
            font-family="Inter,system-ui,sans-serif">you→node</text
          >
        </svg>
      {:else}
        <!-- ── Tree (list) topology ────────────────────────────────────────── -->
        <svg
          viewBox="0 0 580 {treeSvgH}"
          width="100%"
          height={treeSvgH}
          xmlns="http://www.w3.org/2000/svg"
        >
          <defs>
            <marker
              id="t-out"
              markerWidth="6"
              markerHeight="6"
              refX="5"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L6,3 z" fill="#f97316" />
            </marker>
            <marker
              id="t-in"
              markerWidth="6"
              markerHeight="6"
              refX="5"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L6,3 z" fill="#58a6ff" />
            </marker>
            <marker
              id="t-disc"
              markerWidth="6"
              markerHeight="6"
              refX="5"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L6,3 z" fill="#30363d" />
            </marker>
          </defs>

          <!-- Managed node -->
          <circle
            cx={TREE_MX}
            cy={TREE_MY}
            r="32"
            fill="#1e2937"
            stroke={managedColor}
            stroke-width="2.5"
          />
          <text
            x={TREE_MX}
            y={TREE_MY - 3}
            text-anchor="middle"
            fill={managedColor}
            font-size="11"
            font-weight="bold"
            font-family="Inter,system-ui,sans-serif">{managedNodeId}</text
          >
          <text
            x={TREE_MX}
            y={TREE_MY + 11}
            text-anchor="middle"
            fill="#8b949e"
            font-size="8"
            font-family="Inter,system-ui,sans-serif">managed</text
          >

          <!-- Trunk (vertical line down from managed bottom) -->
          <line
            x1={TREE_MX}
            y1={TREE_MY + 32}
            x2={TREE_MX}
            y2={treeTrunkEnd}
            stroke="#30363d"
            stroke-width="2"
          />

          <!-- Peer rows -->
          {#each topoNodes as n, i}
            {@const py = TREE_PY0 + i * TREE_ROW_H}
            {@const edge = TREE_PEER_X - 28}
            <!-- peer circle left edge -->

            <!-- Junction dot on trunk -->
            <circle cx={TREE_MX} cy={py} r="3" fill="#484f58" />

            <!-- Direction lines: branch goes from trunk to peer circle edge -->
            {#if n.directions.has("outbound") && n.directions.has("inbound")}
              <!-- Both: two offset horizontal lines -->
              <line
                x1={TREE_MX + 3}
                y1={py - 4}
                x2={edge}
                y2={py - 4}
                stroke={n.connected ? "#f97316" : "#30363d"}
                stroke-width="1.5"
                marker-end={n.connected ? "url(#t-out)" : "url(#t-disc)"}
              />
              <line
                x1={edge}
                y1={py + 4}
                x2={TREE_MX + 3}
                y2={py + 4}
                stroke={n.connected ? "#58a6ff" : "#30363d"}
                stroke-width="1.5"
                marker-end={n.connected ? "url(#t-in)" : "url(#t-disc)"}
              />
            {:else if n.directions.has("outbound")}
              <line
                x1={TREE_MX + 3}
                y1={py}
                x2={edge}
                y2={py}
                stroke={n.connected ? "#f97316" : "#30363d"}
                stroke-width="2"
                stroke-dasharray={n.connected ? "" : "4,3"}
                marker-end={n.connected ? "url(#t-out)" : "url(#t-disc)"}
              />
            {:else}
              <!-- Inbound: arrow points BACK toward managed -->
              <line
                x1={edge}
                y1={py}
                x2={TREE_MX + 3}
                y2={py}
                stroke={n.connected ? "#58a6ff" : "#30363d"}
                stroke-width="2"
                stroke-dasharray={n.connected ? "" : "4,3"}
                marker-end={n.connected ? "url(#t-in)" : "url(#t-disc)"}
              />
            {/if}

            <!-- Peer circle -->
            <circle
              cx={TREE_PEER_X}
              cy={py}
              r="28"
              fill="#1c2128"
              stroke={nodeColor(n)}
              stroke-width="2.5"
            />
            <text
              x={TREE_PEER_X}
              y={py - 2}
              text-anchor="middle"
              fill={n.connected ? "#e6edf3" : "#8b949e"}
              font-size="10"
              font-weight="600"
              font-family="Inter,system-ui,sans-serif">{n.node_id || "?"}</text
            >

            <!-- Direction label inside circle (below node name) -->
            <text
              x={TREE_PEER_X}
              y={py + 11}
              text-anchor="middle"
              font-size="8"
              font-family="Inter,system-ui,sans-serif"
              fill={n.directions.has("inbound") && n.directions.has("outbound")
                ? "#e3b341"
                : n.directions.has("outbound")
                  ? "#f97316"
                  : "#58a6ff"}
              >{n.directions.has("inbound") && n.directions.has("outbound")
                ? "both"
                : n.directions.has("outbound")
                  ? "outbound"
                  : "inbound"}</text
            >
          {/each}
        </svg>
      {/if}
    </div>
  </div>
</div>

<style>
  .page {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    padding: 20px 28px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 16px;
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
    font-size: 18px;
    font-weight: 700;
  }
  h2 {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 12px;
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .legend {
    font-weight: 400;
    font-size: 12px;
    color: #8b949e;
  }
  .muted {
    color: #8b949e;
  }
  .center {
    text-align: center;
    padding: 24px;
  }
  .mono {
    font-family: monospace;
  }
  .col-all {
    grid-column: 1 / -1;
  }
  .err {
    background: #2d1b1b;
    border: 1px solid #f85149;
    border-radius: 6px;
    padding: 8px 12px;
    color: #f85149;
    font-size: 12px;
  }
  .section {
    background: #161b22;
    border: 1px solid #21262d;
    border-radius: 10px;
    padding: 20px;
  }
  .peer-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 12px;
  }
  .peer-card {
    background: #0d1117;
    border: 1px solid #30363d;
    border-radius: 8px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .peer-card.connected {
    border-color: #21262d;
  }
  .peer-top {
    display: flex;
    align-items: center;
    gap: 8px;
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
  }
  .node-id {
    font-weight: 600;
    font-size: 13px;
    flex: 1;
  }
  .dir {
    font-size: 10px;
    padding: 2px 7px;
    border-radius: 10px;
    font-weight: 600;
  }
  .dir-out {
    background: #2d1a05;
    color: #f97316;
  }
  .dir-in {
    background: #0a1929;
    color: #58a6ff;
  }
  .dir-both {
    background: #2a2000;
    color: #e3b341;
  }
  .peer-addr {
    font-size: 11px;
    color: #8b949e;
    word-break: break-all;
  }
  .peer-meta {
    font-size: 11px;
    color: #8b949e;
  }
  .switch-btn {
    background: none;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #8b949e;
    cursor: pointer;
    padding: 5px 10px;
    font-size: 12px;
    align-self: flex-start;
    margin-top: 4px;
    transition:
      border-color 0.15s,
      color 0.15s;
  }
  .switch-btn:hover:not(:disabled) {
    border-color: #f97316;
    color: #f97316;
  }
  .switch-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .switch-btn.is-loading {
    opacity: 0.6;
  }
  .mode-select {
    background: #0d1117;
    border: 1px solid #30363d;
    color: #8b949e;
    border-radius: 4px;
    padding: 3px 22px 3px 7px; /* room for chevron */
    font-size: 11px;
    cursor: pointer;
    -webkit-appearance: none;
    appearance: none;
    width: 100%;
  }
  .mode-select:focus {
    outline: none;
    border-color: #f97316;
  }
  .mode-select-wrap {
    position: relative;
    margin-left: auto;
  }
  .mode-chevron {
    position: absolute;
    right: 6px;
    top: 50%;
    transform: translateY(-50%);
    color: #8b949e;
    font-size: 11px;
    pointer-events: none;
    line-height: 1;
  }
  .topo-canvas {
    overflow-x: auto;
  }
</style>
