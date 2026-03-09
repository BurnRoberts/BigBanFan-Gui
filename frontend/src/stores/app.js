import { writable } from 'svelte/store';

// connection state — activeProfile holds the full profile ({name,host,port,key_hex})
export const connected = writable(false);
export const activeProfile = writable(null);

// data stores (refreshed by views)
export const bans = writable([]);
export const peers = writable([]);
export const status = writable(null);
export const stats = writable(null);
export const logs = writable([]);
