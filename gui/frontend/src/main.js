import { mount } from 'svelte'
import App from './App.svelte'

// Svelte 5 mounts with mount(); `new App(...)` is the removed Svelte 4 API
// (#29 migration). See https://svelte.dev/e/component_api_invalid_new
const app = mount(App, {
  target: document.getElementById('app'),
})

export default app
