// clipboardStore.js
// Store for managing copy/cut/paste operations on decisions

import { writable } from 'svelte/store';

function createClipboardStore() {
    const { subscribe, set, update } = writable({
        decisions: [],
        isCut: false
    });

    return {
        subscribe,
        
        // Copy decisions to clipboard
        copy: (decisions) => {
            console.log('[clipboardStore.copy] Copying decisions:', decisions);
            set({
                decisions: JSON.parse(JSON.stringify(decisions)), // Deep copy
                isCut: false
            });
        },
        
        // Cut decisions to clipboard
        cut: (decisions) => {
            console.log('[clipboardStore.cut] Cutting decisions:', decisions);
            set({
                decisions: JSON.parse(JSON.stringify(decisions)), // Deep copy
                isCut: true
            });
        },
        
        // Clear clipboard
        clear: () => {
            console.log('[clipboardStore.clear] Clearing clipboard');
            set({
                decisions: [],
                isCut: false
            });
        },
        
        // Check if clipboard has content
        hasContent: (state) => {
            return state.decisions && state.decisions.length > 0;
        },
        
        // Get clipboard content
        getContent: (state) => {
            return {
                decisions: state.decisions,
                isCut: state.isCut
            };
        }
    };
}

export const clipboardStore = createClipboardStore();
