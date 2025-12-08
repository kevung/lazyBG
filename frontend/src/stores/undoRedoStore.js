import { writable, get } from 'svelte/store';
import { transcriptionStore } from './transcriptionStore';

// Maximum undo history size
const MAX_HISTORY_SIZE = 100;

/**
 * Simple snapshot-based undo/redo system.
 * Takes full snapshots of transcription state after each operation.
 * This is simple, reliable, and works for all operations.
 */
const createUndoRedoStore = () => {
    const { subscribe, set, update } = writable({
        undoStack: [],     // Array of previous states
        redoStack: [],     // Array of undone states
    });

    return {
        subscribe,
        
        /**
         * Save current transcription state to undo stack.
         * Call this BEFORE an operation executes to save the state we can undo to.
         */
        saveSnapshot: () => {
            const currentTranscription = get(transcriptionStore);
            
            // Don't save if there's no transcription
            if (!currentTranscription || !currentTranscription.games) {
                console.log('[undoRedoStore] No transcription to save');
                return;
            }
            
            // Deep clone to prevent mutations
            const snapshot = JSON.parse(JSON.stringify(currentTranscription));
            
            update(state => {
                // Add to undo stack
                const newUndoStack = [...state.undoStack, snapshot];
                
                // Limit stack size
                while (newUndoStack.length > MAX_HISTORY_SIZE) {
                    newUndoStack.shift();
                }
                
                console.log('[undoRedoStore] Snapshot saved. Undo stack size:', newUndoStack.length);
                
                // Clear redo stack when new snapshot is saved
                return {
                    undoStack: newUndoStack,
                    redoStack: [],
                };
            });
        },
        
        /**
         * Undo: restore previous state and save current state to redo stack
         */
        undo: () => {
            const state = get({ subscribe });
            
            if (state.undoStack.length === 0) {
                console.log('[undoRedoStore] Cannot undo - stack is empty');
                return false;
            }
            
            // Save current state to redo stack before undoing
            const currentTranscription = get(transcriptionStore);
            const currentSnapshot = JSON.parse(JSON.stringify(currentTranscription));
            
            // Get the previous state
            const previousState = state.undoStack[state.undoStack.length - 1];
            
            console.log('[undoRedoStore] Undoing. Undo stack size:', state.undoStack.length);
            
            // Restore previous state
            transcriptionStore.set(JSON.parse(JSON.stringify(previousState)));
            
            // Update stacks
            update(s => ({
                undoStack: s.undoStack.slice(0, -1),
                redoStack: [...s.redoStack, currentSnapshot],
            }));
            
            console.log('[undoRedoStore] Undo completed');
            return true;
        },
        
        /**
         * Redo: restore next state and save current state to undo stack
         */
        redo: () => {
            const state = get({ subscribe });
            
            if (state.redoStack.length === 0) {
                console.log('[undoRedoStore] Cannot redo - stack is empty');
                return false;
            }
            
            // Save current state to undo stack before redoing
            const currentTranscription = get(transcriptionStore);
            const currentSnapshot = JSON.parse(JSON.stringify(currentTranscription));
            
            // Get the next state
            const nextState = state.redoStack[state.redoStack.length - 1];
            
            console.log('[undoRedoStore] Redoing. Redo stack size:', state.redoStack.length);
            
            // Restore next state
            transcriptionStore.set(JSON.parse(JSON.stringify(nextState)));
            
            // Update stacks
            update(s => ({
                undoStack: [...s.undoStack, currentSnapshot],
                redoStack: s.redoStack.slice(0, -1),
            }));
            
            console.log('[undoRedoStore] Redo completed');
            return true;
        },
        
        /**
         * Check if undo is available
         */
        canUndo: () => {
            const state = get({ subscribe });
            return state.undoStack.length > 0;
        },
        
        /**
         * Check if redo is available
         */
        canRedo: () => {
            const state = get({ subscribe });
            return state.redoStack.length > 0;
        },
        
        /**
         * Clear all undo/redo history
         */
        clear: () => {
            console.log('[undoRedoStore] Clearing undo/redo stacks');
            set({
                undoStack: [],
                redoStack: [],
            });
        },
    };
};

export const undoRedoStore = createUndoRedoStore();
