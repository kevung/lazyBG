<script>
   import { onMount, onDestroy } from 'svelte';
   import { commandTextStore, previousModeStore, statusBarModeStore, showCommandStore, statusBarTextStore } from '../stores/uiStore';
   import { showMetadataModalStore } from '../stores/uiStore';
   import { commandHistoryStore } from '../stores/commandHistoryStore';
   import { transcriptionStore, selectedMoveStore } from '../stores/transcriptionStore';
   import { executeSearch, searchQueryStore, showMoveSearchModalStore } from '../stores/moveSearchStore';
   import { get } from 'svelte/store';

   export let onToggleHelp;
   export let onNewMatch;
   export let onOpenMatch;
   export let exitApp;
   let inputEl;

   let initialized = false;

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
               let targetMoveNumber = parseInt(match[1], 10);
               const transcription = get(transcriptionStore);
               const currentSelectedMove = get(selectedMoveStore);
               const currentGameIndex = currentSelectedMove.gameIndex;
               
               const game = transcription.games[currentGameIndex];
               if (game && game.moves && game.moves.length > 0) {
                  // Max move number is the last move's moveNumber
                  const lastMove = game.moves[game.moves.length - 1];
                  const maxMoveNumber = lastMove.moveNumber;
                  
                  // Clamp to valid range
                  if (targetMoveNumber < 1) {
                     targetMoveNumber = 1;
                  } else if (targetMoveNumber > maxMoveNumber) {
                     targetMoveNumber = maxMoveNumber;
                  }
                  
                  // Find the move with the target moveNumber
                  const moveIndex = game.moves.findIndex(m => m.moveNumber === targetMoveNumber);
                  
                  if (moveIndex !== -1) {
                     // Default to player 1
                     selectedMoveStore.set({ gameIndex: currentGameIndex, moveIndex, player: 1 });
                  }
               }
            } else if (command === 'new' || command === 'ne' || command === 'n') {
               onNewMatch();
            } else if (command === 'open' || command === 'op' || command === 'o') {
               onOpenMatch();
            } else if (command === 'quit' || command === 'q') {
               exitApp();
            } else if (command === 'help' || command === 'he' || command === 'h') {
               onToggleHelp();
            } else if (command === 'meta') {
               showMetadataModalStore.set(true);
            } else if (command === 'cl' || command === 'clear') {
               try {
                  // Database functionality removed
                  commandHistoryStore.set([]);
                  statusBarTextStore.set('Command history cleared.');
               } catch (error) {
                  console.error('Error clearing command history:', error);
                  statusBarTextStore.set('Error clearing command history.');
               }
            } else if (command.startsWith('s ')) {
               // Handle search command: s <query>
               const searchQuery = command.substring(2).trim();
               if (searchQuery) {
                  searchQueryStore.set(searchQuery);
                  showMoveSearchModalStore.set(true);
                  executeSearch(searchQuery);
                  statusBarTextStore.set(`Searching for: ${searchQuery}`);
               } else {
                  statusBarTextStore.set('Search query is required. Usage: s <dice|d|t|p>');
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
