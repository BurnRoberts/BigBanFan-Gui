<script>
  import { onMount, onDestroy } from "svelte";
  import { connected, logs } from "./stores/app.js";
  import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime.js";
  import {
    SubscribeLogs,
    UnsubscribeLogs,
    GetLogs,
  } from "../wailsjs/go/main/App.js";
  import Connect from "./views/Connect.svelte";
  import Dashboard from "./views/Dashboard.svelte";
  import BanList from "./views/BanList.svelte";
  import Peers from "./views/Peers.svelte";
  import Logs from "./views/Logs.svelte";

  let view = "connect";
  let logCount = 0;

  function nav(v) {
    view = v;
    if (v === "logs") logCount = 0;
  }

  onMount(() => {
    EventsOn("connection:up", async () => {
      connected.set(true);
      view = "dashboard";
      // Start live stream immediately (fire-and-forget), then back-fill history.
      SubscribeLogs("info").catch(() => {});
      try {
        const lines = await GetLogs();
        if (lines && lines.length) {
          // Prepend history into the store — live lines will continue appending.
          logs.update((l) => [...lines, ...l].slice(-2000));
        }
      } catch (_) {
        /* node may not support GET_LOGS — ignore */
      }
    });
    EventsOn("connection:down", () => {
      connected.set(false);
      view = "connect";
      UnsubscribeLogs().catch(() => {});
    });
    EventsOn("log:line", (line) => {
      logs.update((l) => [...l.slice(-1999), line]);
      if (view !== "logs") logCount++;
    });
  });

  onDestroy(() => {
    // Always clean up Wails event listeners. Without this, reconnecting within
    // the same app session stacks duplicate handlers — causing double LOG_SUBSCRIBE.
    EventsOff("connection:up");
    EventsOff("connection:down");
    EventsOff("log:line");
  });
</script>

<div class="shell">
  {#if $connected}
    <nav class="sidebar">
      <div class="brand">
        <span class="flame">🔥</span>
        <span>BigBanFan</span>
      </div>
      <button
        class:active={view === "dashboard"}
        on:click={() => nav("dashboard")}
      >
        <span class="ico">⬡</span> Dashboard
      </button>
      <button class:active={view === "bans"} on:click={() => nav("bans")}>
        <span class="ico">⊘</span> Ban List
      </button>
      <button class:active={view === "peers"} on:click={() => nav("peers")}>
        <span class="ico">⬡</span> Peers
      </button>
      <button class:active={view === "logs"} on:click={() => nav("logs")}>
        <span class="ico">≡</span> Logs
        {#if logCount > 0}<span class="badge"
            >{logCount > 99 ? "99+" : logCount}</span
          >{/if}
      </button>
      <div class="spacer"></div>
      <button
        class="disconnect"
        on:click={() =>
          import("../wailsjs/go/main/App.js").then((m) => m.Disconnect())}
      >
        Disconnect
      </button>
    </nav>
    <main class="content">
      {#if view === "dashboard"}<Dashboard />{/if}
      {#if view === "bans"}<BanList />{/if}
      {#if view === "peers"}<Peers />{/if}
      {#if view === "logs"}<Logs on:read={() => (logCount = 0)} />{/if}
    </main>
  {:else}
    <div class="connect-wrap">
      <Connect />
    </div>
  {/if}
</div>

<style>
  :global(*, *::before, *::after) {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
  }
  :global(body) {
    background: #0d1117;
    color: #e6edf3;
    font-family: "Inter", "Segoe UI", system-ui, sans-serif;
    font-size: 14px;
    height: 100vh;
    overflow: hidden;
  }

  .shell {
    display: flex;
    height: 100vh;
  }

  .sidebar {
    width: 200px;
    background: #161b22;
    border-right: 1px solid #21262d;
    display: flex;
    flex-direction: column;
    padding: 16px 0;
    gap: 2px;
    flex-shrink: 0;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 20px 20px;
    font-weight: 700;
    font-size: 15px;
    color: #e6edf3;
  }
  .flame {
    font-size: 20px;
  }

  .sidebar button {
    background: none;
    border: none;
    color: #8b949e;
    text-align: left;
    padding: 9px 20px;
    cursor: pointer;
    border-radius: 6px;
    margin: 0 8px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    transition:
      background 0.15s,
      color 0.15s;
    position: relative;
  }
  .sidebar button:hover {
    background: #21262d;
    color: #e6edf3;
  }
  .sidebar button.active {
    background: #1f2937;
    color: #f97316;
    font-weight: 600;
  }

  .badge {
    background: #f97316;
    color: #000;
    font-size: 10px;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 10px;
    position: absolute;
    right: 12px;
  }

  .spacer {
    flex: 1;
  }

  .disconnect {
    color: #f85149 !important;
    font-size: 12px;
    margin-bottom: 4px;
  }
  .disconnect:hover {
    background: #2d1b1b !important;
  }

  .content {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    position: relative;
    min-height: 0;
  }

  .connect-wrap {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #0d1117;
  }
</style>
