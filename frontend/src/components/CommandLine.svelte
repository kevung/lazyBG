<script>
   import { onMount, onDestroy } from 'svelte';
   import { currentPositionIndexStore, commandTextStore, previousModeStore, statusBarModeStore, showCommandStore, statusBarTextStore } from '../stores/uiStore';
   import { positionsStore } from '../stores/positionStore';
   import { showMetadataModalStore } from '../stores/uiStore';
   import { databaseLoadedStore } from '../stores/databaseStore';
   import { commandHistoryStore } from '../stores/commandHistoryStore';

   export let onToggleHelp;
   export let onNewDatabase;
   export let onOpenDatabase;
   export let exitApp;
   let inputEl;

   let initialized = false;

   // Subscribe to the stores
   let positions = [];
   positionsStore.subscribe(value => positions = value);

   let databaseLoaded = false;
   databaseLoadedStore.subscribe(value => databaseLoaded = value);

   let commandHistory = [];
   let historyIndex = -1;

   commandHistoryStore.subscribe(value => commandHistory = value);

   showCommandStore.subscribe(async value => {
      if (value) {
         previousModeStore.set($statusBarModeStore);
         statusBarModeStore.set('COMMAND');
         commandTextStore.set('');
         setTimeout(() => {
            inputEl?.focus();
         }, 0);
         window.addEventListener('click', handleClickOutside);

         // Database functionality removed - command history not loaded
         historyIndex = -1; // Start at the end of the history
      } else {
         statusBarModeStore.set($previousModeStore); // Restore the previous mode
         window.removeEventListener('click', handleClickOutside);
      }
   });

   async function handleKeyDown(event) {
      event.stopPropagation();

      if ($showCommandStore) {
         if (event.code === 'ArrowUp') {
            if (historyIndex < commandHistory.length - 1) {
               historyIndex++;
               commandTextStore.set(commandHistory[historyIndex]);
               // Move cursor to the end without delay
               requestAnimationFrame(() => {
                  inputEl.setSelectionRange(inputEl.value.length, inputEl.value.length);
               });
            }
         } else if (event.code === 'ArrowDown') {
            if (historyIndex > 0) {
               historyIndex--;
               commandTextStore.set(commandHistory[historyIndex]);
               // Move cursor to the end without delay
               requestAnimationFrame(() => {
                  inputEl.setSelectionRange(inputEl.value.length, inputEl.value.length);
               });
            } else {
               historyIndex = -1;
               commandTextStore.set('');
            }
         } else if (event.code === 'Backspace' && inputEl.value === '') {
            showCommandStore.set(false);
         } else if (event.code === 'Escape') {
            showCommandStore.set(false);
         } else if (event.code === 'Enter') {
            const command = inputEl.value.trim();
            console.log('Command entered:', command); // Debugging log
            if (command) {
               commandHistoryStore.update(history => {
                  history = history || []; // Ensure history is an array
                  history.unshift(command); // Add the new command to the beginning
                  return history;
               });
               historyIndex = -1; // Reset the history index
               // Database functionality removed - command not saved
            }
            const match = command.match(/^(\d+)$/);
            if (match) {
               const positionNumber = parseInt(match[1], 10);
               let index;
               if (positionNumber < 1) {
                  index = 0;
               } else if (positionNumber > positions.length) {
                  index = positions.length - 1;
               } else {
                  index = positionNumber - 1;
               }
               currentPositionIndexStore.set(index);
            } else if (command === 'new' || command === 'ne' || command === 'n') {
               onNewDatabase();
            } else if (command === 'open' || command === 'op' || command === 'o') {
               onOpenDatabase();
            } else if (command === 'quit' || command === 'q') {
               exitApp();
            } else if (command === 'help' || command === 'he' || command === 'h') {
               onToggleHelp();
            } else if (command === 'meta') {
               if (databaseLoaded) {
                  showMetadataModalStore.set(true);
               } else {
                  statusBarTextStore.set('No transcription loaded.');
               }
            } else if (command === 'cl' || command === 'clear') {
               try {
                  // Database functionality removed
                  commandHistoryStore.set([]);
                  statusBarTextStore.set('Command history cleared.');
               } catch (error) {
                  console.error('Error clearing command history:', error);
                  statusBarTextStore.set('Error clearing command history.');
               }
            }
            showCommandStore.set(false); // Hide the command line after processing the command
         } else if (event.ctrlKey && event.code === 'KeyH') {
            onToggleHelp();
         }
      }
   }

   function handleClickOutside(event) {
      if ($showCommandStore && !inputEl.contains(event.target)) {
         showCommandStore.set(false);
      }
   }

   function handleGlobalKeyDown(event) {
      if (!initialized) {
         initialized = true;
         window.addEventListener('keydown', handleKeyDown);
      }
   }

   onMount(() => {
      window.addEventListener('keydown', handleGlobalKeyDown);
   });

   onDestroy(() => {
      window.removeEventListener('click', handleClickOutside);
      window.removeEventListener('keydown', handleGlobalKeyDown);
   });
</script>

{#if $showCommandStore}
   <input
         type="text"
         bind:this={inputEl}
         bind:value={$commandTextStore}
         class="command-input"
         placeholder=" Type your command here. "
         on:keydown={handleKeyDown}
         />
{/if}

<style>
    input {
        position: fixed;
        top: 350px;
        left: 50%;
        transform: translateX(-50%);
        z-index: 1000;
        width: 70%;
        padding: 8px;
        border: 1px solid rgba(0, 0, 0, 0.3); /* Subtle border */
        border-radius: 1px;
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0);
        outline: none;
        background-color: white; /* Ensure background is opaque */
        font-size: 18px;
    }
</style>
