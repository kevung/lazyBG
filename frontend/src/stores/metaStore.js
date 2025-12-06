import { writable } from 'svelte/store';

export const metaStore = writable({
    applicationVersion: __APP_VERSION__,
});
