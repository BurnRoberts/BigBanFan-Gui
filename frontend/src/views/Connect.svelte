<script>
  import { onMount } from "svelte";
  import {
    Connect as GoConnect,
    ListProfiles,
    SaveProfile,
    DeleteProfile,
  } from "../../wailsjs/go/main/App.js";
  import { activeProfile } from "../stores/app.js";

  let profiles = [];
  let selected = "";
  let host = "",
    port = 7779,
    keyHex = "",
    name = "";
  let saving = false;
  let connecting = false;
  let error = "";

  async function load() {
    error = "";
    profiles = await ListProfiles();
    if (profiles.length && !selected) selected = profiles[0].name;
  }

  function pickProfile() {
    error = "";
    if (!profiles.length) return;
    const p = profiles.find((p) => p.name === selected);
    if (p) {
      host = p.host;
      port = p.port;
      keyHex = p.key_hex;
      name = p.name;
    }
  }
  $: selected, pickProfile();

  async function connect() {
    error = "";
    connecting = true;
    try {
      const prof = { name, host, port: Number(port), key_hex: keyHex };
      await GoConnect(prof);
      activeProfile.set(prof); // store full profile including key_hex for switchTo
    } catch (e) {
      error = e.toString();
    } finally {
      connecting = false;
    }
  }

  async function save() {
    saving = true;
    try {
      await SaveProfile({ name, host, port: Number(port), key_hex: keyHex });
      await load();
    } catch (e) {
      error = e.toString();
    } finally {
      saving = false;
    }
  }

  async function del(n) {
    if (!confirm(`Delete saved connection "${n}"?\nThis cannot be undone.`))
      return;
    try {
      await DeleteProfile(n);
      profiles = profiles.filter((p) => p.name !== n);
      if (selected === n) selected = "";
    } catch (e) {
      error = e.toString();
    }
  }

  onMount(() => load());
</script>

<div class="card">
  <div class="logo">🔥<span>BigBanFan</span></div>
  <p class="sub">Connect to a management port</p>

  {#if profiles.length}
    <div class="field">
      <label for="saved-conn">Saved connections</label>
      <div class="row">
        <div class="select-wrap">
          <select id="saved-conn" bind:value={selected}>
            {#each profiles as p}<option value={p.name}>{p.name}</option>{/each}
          </select>
          <span class="chevron">▾</span>
        </div>
        <button
          class="icon-btn danger"
          title="Delete this saved connection"
          on:click={() => del(selected)}>✕</button
        >
      </div>
    </div>
  {/if}

  <div class="field">
    <label for="profile-name">Profile name</label>
    <input id="profile-name" bind:value={name} placeholder="e.g. cdn12-prod" />
  </div>
  <div class="field">
    <label for="conn-host">Host</label>
    <input
      id="conn-host"
      bind:value={host}
      placeholder="cdn12.example.com or 1.2.3.4 or ::1"
    />
  </div>
  <div class="row gap">
    <div class="field" style="flex:1">
      <label for="conn-port">Port</label>
      <input
        id="conn-port"
        type="number"
        bind:value={port}
        min="1"
        max="65535"
      />
    </div>
  </div>
  <div class="field">
    <label for="conn-key"
      >Client key <span class="hint">(64 hex chars)</span></label
    >
    <input
      id="conn-key"
      bind:value={keyHex}
      type="password"
      placeholder="client_key from config.yaml"
      autocomplete="off"
    />
  </div>

  {#if error}<div class="error">{error}</div>{/if}

  <div class="actions">
    <button class="btn-secondary" on:click={save} disabled={saving || !name}>
      {saving ? "Saving…" : "💾 Save"}
    </button>
    <button
      class="btn-primary"
      on:click={connect}
      disabled={connecting || !host || !keyHex}
    >
      {connecting ? "Connecting…" : "Connect →"}
    </button>
  </div>
</div>

<style>
  .card {
    background: #161b22;
    border: 1px solid #21262d;
    border-radius: 12px;
    padding: 36px 40px;
    width: 420px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .logo {
    font-size: 22px;
    font-weight: 700;
    display: flex;
    align-items: center;
    gap: 8px;
    color: #f97316;
  }
  .sub {
    color: #8b949e;
    font-size: 13px;
    margin-top: -6px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  label {
    font-size: 12px;
    color: #8b949e;
  }
  .hint {
    font-style: italic;
    opacity: 0.7;
  }
  input,
  select {
    background: #0d1117;
    border: 1px solid #30363d;
    border-radius: 6px;
    color: #e6edf3;
    padding: 8px 10px;
    font-size: 13px;
    width: 100%;
    transition: border-color 0.15s;
    -webkit-appearance: none;
    appearance: none;
    overflow: hidden;
  }
  input:focus,
  select:focus {
    outline: none;
    border-color: #f97316;
  }
  select {
    cursor: pointer;
  }
  /* Custom dropdown arrow (appearance:none strips the native one) */
  .select-wrap {
    position: relative;
    flex: 1;
  }
  .select-wrap select {
    padding-right: 28px; /* room for chevron */
  }
  .chevron {
    position: absolute;
    right: 9px;
    top: 50%;
    transform: translateY(-50%);
    color: #8b949e;
    font-size: 13px;
    pointer-events: none;
    line-height: 1;
  }
  /* Hide number input spinner arrows */
  input[type="number"]::-webkit-inner-spin-button,
  input[type="number"]::-webkit-outer-spin-button {
    -webkit-appearance: none;
    appearance: none;
    margin: 0;
  }
  input[type="number"] {
    -moz-appearance: textfield;
    appearance: textfield;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .gap {
    gap: 12px;
  }
  .icon-btn {
    background: none;
    border: 1px solid #30363d;
    border-radius: 6px;
    color: #8b949e;
    cursor: pointer;
    padding: 7px 10px;
    font-size: 12px;
    flex-shrink: 0;
  }
  .icon-btn.danger:hover {
    border-color: #f85149;
    color: #f85149;
  }
  .error {
    background: #2d1b1b;
    border: 1px solid #f85149;
    border-radius: 6px;
    padding: 8px 12px;
    color: #f85149;
    font-size: 12px;
  }
  .actions {
    display: flex;
    gap: 10px;
    margin-top: 4px;
  }
  .btn-primary {
    flex: 1;
    background: #f97316;
    color: #000;
    border: none;
    border-radius: 6px;
    padding: 10px;
    font-weight: 700;
    cursor: pointer;
    font-size: 13px;
    transition: background 0.15s;
  }
  .btn-primary:hover:not(:disabled) {
    background: #ea6c0b;
  }
  .btn-primary:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .btn-secondary {
    background: #21262d;
    color: #e6edf3;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 10px 16px;
    cursor: pointer;
    font-size: 13px;
    transition: background 0.15s;
  }
  .btn-secondary:hover:not(:disabled) {
    background: #30363d;
  }
  .btn-secondary:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
</style>
