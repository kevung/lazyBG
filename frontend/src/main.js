import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'

console.log('main.js: Starting application');
console.log('main.js: Target element:', document.getElementById("app"));

let app;
try {
    app = mount(App, { target: document.getElementById("app") });
    console.log('main.js: App mounted successfully', app);
} catch (error) {
    console.error('main.js: Error mounting app:', error);
    console.error('main.js: Error stack:', error.stack);
    throw error;
}

export default app;
