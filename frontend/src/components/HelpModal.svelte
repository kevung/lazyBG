<!-- HelpModal.svelte -->
<script>

    import { onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import { metaStore } from '../stores/metaStore'; // Import metaStore
    import { LAZYBG_VERSION } from '../stores/transcriptionStore'; // Import transcription format version

    export let visible = false;
    export let onClose;
    export let handleGlobalKeydown;

    let activeTab = "manual"; // Default active tab
    const tabs = ['manual', 'shortcuts', 'commands', 'about'];
    let contentArea;

    let applicationVersion = '';

    // Subscribe to the metaStore
    metaStore.subscribe(value => {
        applicationVersion = value.applicationVersion;
    });

    function switchTab(tab) {
        activeTab = tab;
    }

    function activateGlobalShortcuts() {
        window.addEventListener('keydown', handleGlobalKeydown);
    }

    function deactivateGlobalShortcuts() {
        window.removeEventListener('keydown', handleGlobalKeydown);
    }


    // Close on Esc key
    function handleKeyDown(event) {

        // Prevent default action and stop event propagation
        event.stopPropagation();

        if(visible) {
            event.preventDefault();
            if (event.key === 'Escape') {
                onClose();
            } else if (event.ctrlKey && event.code === 'KeyH') {
                onClose();
            } else if (!event.ctrlKey && event.key === '?') {
                onClose();
            } else if (!event.ctrlKey && event.key === 'ArrowRight') {
                navigateTabs(1); // Move to the next tab
            } else if (!event.ctrlKey && event.key === 'ArrowLeft') {
                navigateTabs(-1); // Move to the previous tab
            } else if (!event.ctrlKey && event.key === 'l') {
                navigateTabs(1); // Move to the next tab
            } else if (!event.ctrlKey && event.key === 'h') {
                navigateTabs(-1); // Move to the previous tab
            } else if (!event.ctrlKey && event.key === 'ArrowDown') {
                scrollContent(1); // Scroll down
            } else if (!event.ctrlKey && event.key === 'ArrowUp') {
                scrollContent(-1); // Scroll up
            } else if (!event.ctrlKey && event.key === 'j') {
                scrollContent(1); // Scroll down
            } else if (!event.ctrlKey && event.key === 'k') {
                scrollContent(-1); // Scroll up
            } else if (!event.ctrlKey && event.key === 'PageDown') {
                scrollContent('bottom'); // Go to the bottom of the page
            } else if (!event.ctrlKey && event.key === 'PageUp') {
                scrollContent('top'); // Go to the top of the page
            } else if (!event.ctrlKey && event.key === ' ') { // Space key
                scrollContent('page'); // Scroll down by the height of the content
            }
        }
    }

    function navigateTabs(direction) {
        const currentIndex = tabs.indexOf(activeTab);
        const newIndex = (currentIndex + direction + tabs.length) % tabs.length;
        switchTab(tabs[newIndex]);
    }

    
    function scrollContent(direction) {
        if (contentArea) {
            const scrollAmount = 60; // Pixels to scroll per key press

            if (direction === 1) { // Arrow down
                contentArea.scrollTop += scrollAmount;
            } else if (direction === -1) { // Arrow up
                contentArea.scrollTop -= scrollAmount;
            } else if (direction === 'bottom') { // PageDown
                contentArea.scrollTop = contentArea.scrollHeight; // Go to bottom
            } else if (direction === 'top') { // PageUp
                contentArea.scrollTop = 0; // Go to top
            } else if (direction === 'page') { // Space key
                contentArea.scrollTop += contentArea.clientHeight; // Scroll down by the visible area height
            }
        }
    }

    // Close the modal when clicking outside of it
    function handleClickOutside(event) {
        const modalContent = document.getElementById('modalContent');
        if (modalContent && !modalContent.contains(event.target)) {
            onClose(); // Close the help modal if the click is outside of it
        }
    }

    onMount(() => {
        window.addEventListener('keydown', handleKeyDown);
        window.addEventListener('click', handleClickOutside); // Add click event listener
        deactivateGlobalShortcuts();
    });

    onDestroy(() => {
        window.removeEventListener('keydown', handleKeyDown);
        window.removeEventListener('click', handleClickOutside); // Remove click event listener
        activateGlobalShortcuts();
    });

    // Focus modal content when visible and listen for Esc key
    $: if (visible) {
        setTimeout(() => {
            const helpModal = document.getElementById('helpModal');
            if (helpModal) {
                helpModal.focus();
            }
        }, 0);
        window.addEventListener('keydown', handleKeyDown);
        window.addEventListener('click', handleClickOutside); // Add click event listener
        deactivateGlobalShortcuts();
    } else {
        window.removeEventListener('keydown', handleKeyDown);
        window.removeEventListener('click', handleClickOutside); // Remove click event listener
        activateGlobalShortcuts();
    }

</script>

{#if visible}
    <div class="modal-overlay" id="helpModal" tabindex="0" transition:fade={{ duration: 30 }}>
        <div class="modal-content" id="modalContent">
            <div class="close-button" on:click={onClose} on:keydown={handleKeyDown}>×</div>

            <!-- Tabs -->
            <div class="tab-header">
                <button class="{activeTab === 'manual' ? 'active' : ''}" on:click={() => switchTab('manual')}>Manual</button>
                <button class="{activeTab === 'shortcuts' ? 'active' : ''}" on:click={() => switchTab('shortcuts')}>Shortcut</button>
                <button class="{activeTab === 'commands' ? 'active' : ''}" on:click={() => switchTab('commands')}>Command Line</button>
                <button class="{activeTab === 'about' ? 'active' : ''}" on:click={() => switchTab('about')}>About</button>
            </div>

            <!-- Tab Content -->
            <div class="tab-content" bind:this={contentArea}>
                {#if activeTab === 'manual'}
                    <h3>Introduction</h3>
                    <p>I created lazyBG to provide a lightweight and efficient tool for quickly transcribing backgammon matches. It helps you record and review match moves, providing a simple interface to navigate through games and positions.</p>
                    <p>Transcriptions are stored in files with a .lbg extension.</p>
                    
                    <h3>Main Features</h3>
                    <p>The main interactions possible with lazyBG are:</p>
                    <ul>
                        <li>Creating a new match transcription</li>
                        <li>Opening an existing transcription</li>
                        <li>Navigating through games and moves</li>
                        <li>Editing match metadata (players, event, date, etc.)</li>
                        <li>Viewing and comparing candidate moves</li>
                    </ul>
                    
                    <h3>Modes</h3>
                    <p>lazyBG has three main modes:</p>
                    <ul>
                        <li><strong>NORMAL mode:</strong> Navigate and view positions</li>
                        <li><strong>EDIT mode:</strong> Modify backgammon decisions (dice and moves) using the keyboard</li>
                        <li><strong>COMMAND mode:</strong> Execute commands via the command line interface</li>
                    </ul>
                    
                    <h3>Interface Layout</h3>
                    <p>The lazyBG interface is organized as follows:</p>
                    <ul>
                        <li><strong>Top:</strong> Toolbar with main operations</li>
                        <li><strong>Middle:</strong> Main display area with three sections:
                            <ul>
                                <li>Left panel: Moves table showing transcribed moves</li>
                                <li>Center: Backgammon board with position display</li>
                                <li>Right panel: Candidate moves for current position</li>
                            </ul>
                        </li>
                        <li><strong>Bottom:</strong> Status bar showing current mode and position info</li>
                    </ul>
                    
                    <h3>NORMAL Mode</h3>
                    <p>NORMAL mode is the default mode. Use it to:</p>
                    <ul>
                        <li>Navigate through games and moves</li>
                        <li>View the moves table and candidate moves</li>
                        <li>Review match transcriptions</li>
                    </ul>
                    
                    <h3>EDIT Mode</h3>
                    <p>EDIT mode allows you to modify decisions in your match transcription. Activate it by pressing <strong>TAB</strong>.</p>
                    
                    <h4>Edit Mode with Moves Table Closed</h4>
                    <p>When the moves table is closed, EDIT mode provides a streamlined keyboard-driven workflow:</p>
                    <ul>
                        <li><strong>Dice Entry:</strong> Type dice values directly (1-6 digits) or decision letters:
                            <ul>
                                <li><strong>d</strong> = double, <strong>t</strong> = take, <strong>p</strong> = pass</li>
                                <li><strong>r</strong> = resign, <strong>g</strong> = resign gammon, <strong>b</strong> = resign backgammon</li>
                            </ul>
                        </li>
                        <li><strong>j/k Navigation:</strong> After entering dice, press <strong>j</strong> (down) or <strong>k</strong> (up) to cycle through gnubg candidate moves. The board updates in real-time to preview each move.</li>
                        <li><strong>Cannot Move:</strong> Press <strong>f</strong> to indicate the player cannot move with the current dice (automatically saves and advances).</li>
                        <li><strong>Manual Entry:</strong> Press <strong>Space</strong> to open a text input bar where you can type move notation manually (e.g., "24/20 13/8"). Press <strong>Enter</strong> to confirm or <strong>Esc</strong> to cancel.</li>
                        <li><strong>Validate:</strong> Press <strong>Enter</strong> to save the current decision and automatically move to the next decision.</li>
                        <li><strong>Cancel:</strong> Press <strong>Esc</strong> to exit EDIT mode without saving changes.</li>
                    </ul>
                    
                    <h4>Edit Mode with Moves Table Open</h4>
                    <p>When the moves table is visible, you can edit decisions inline or right-click on any move to access a context menu with options to insert new decisions before or after the selected move.</p>
                    
                    <h3>Inserting Games</h3>
                    <p>You can insert new empty games into the transcription:</p>
                    <ul>
                        <li>Press <strong>n</strong> to insert a new game after the current game</li>
                        <li>Press <strong>N</strong> (Shift+n) to insert a new game before the current game</li>
                        <li>Click the game document icon button in the toolbar (between redo and insert decision)</li>
                    </ul>
                    <p>In the insert panel, choose whether to insert before or after the current game, then press Enter. The new game will be empty and inherit the score from the adjacent game. You can then add moves to this game using the normal editing workflow.</p>
                    
                    <h3>Deleting Games</h3>
                    <p>You can delete the current game from the transcription:</p>
                    <ul>
                        <li>Press <strong>D</strong> (Shift+d) to delete the current game</li>
                        <li>Click the game document with minus icon button in the toolbar (between insert game and insert decision)</li>
                    </ul>
                    <p>When a game is deleted, all subsequent games are renumbered automatically. You cannot delete the last remaining game in a transcription. The position cache is automatically invalidated and recalculated. This action can be undone using the undo feature.</p>
                    
                    <h3>Inserting Decisions</h3>
                    <p>You can insert new empty decisions (player-specific moves) into the transcription:</p>
                    <ul>
                        <li>Press <strong>o</strong> or <strong>O</strong> to open the insert panel</li>
                        <li>In EDIT mode, right-click on a move in the moves table to show a context menu with insert options</li>
                        <li>Click the <strong>+</strong> button in the toolbar to open the insert panel</li>
                    </ul>
                    <p>In the insert panel, choose whether to insert before or after the current decision, then press Enter. The new decision will be empty and flagged by validation. Use Tab to enter edit mode and fill in the move details.</p>
                    <p>The insertion respects whether you're on a player 1 or player 2 decision, ensuring the game flow remains consistent.</p>
                    
                    <h3>Deleting Decisions</h3>
                    <p>You can delete one or multiple decisions using various methods:</p>
                    
                    <h4>Single Decision Deletion</h4>
                    <ul>
                        <li>Press <strong>dd</strong> (vim-like) to cut/delete the current decision (copies to clipboard then deletes)</li>
                        <li>Press <strong>Del</strong> or <strong>Delete</strong> to delete the current decision (also copies to clipboard)</li>
                        <li>Right-click on a move in the moves table to show a context menu with delete option</li>
                        <li>Click the <strong>−</strong> (minus) button in the toolbar to delete the current decision</li>
                    </ul>
                    
                    <h4>Multi-Decision Selection and Deletion</h4>
                    <p>You can select multiple consecutive decisions using any of these methods:</p>
                    <ul>
                        <li><strong>Click and drag</strong> - Click on a decision and drag down or up to select a range</li>
                        <li><strong>Shift+Click</strong> - Click one decision, then hold <strong>Shift</strong> and click another to select everything in between</li>
                        <li><strong>Shift+J/K</strong> - Hold <strong>Shift</strong> and press <strong>J</strong> (down) or <strong>K</strong> (up) to extend the selection</li>
                    </ul>
                    <p>Selected decisions are highlighted with a light blue background. Once multiple decisions are selected, you can delete them all at once using:</p>
                    <ul>
                        <li>Press <strong>dd</strong> or <strong>Del</strong> to delete all selected decisions</li>
                        <li>Right-click anywhere in the selection and choose "Delete" from the context menu</li>
                        <li>Click the <strong>−</strong> button in the toolbar</li>
                    </ul>
                    <p><strong>Note:</strong> Navigating to a different decision (j/k/arrows) will automatically clear any multi-selection.</p>
                    
                    <p>When decisions are deleted, all subsequent decisions are shifted up to fill the gap, maintaining the game flow. The position cache is automatically invalidated and recalculated from the deletion point onwards.</p>
                    
                    <h3>Copy, Cut, and Paste Decisions</h3>
                    <p>lazyBG supports copying, cutting, and pasting decisions, allowing you to efficiently duplicate or reorganize match transcriptions:</p>
                    
                    <h4>Copy Decisions</h4>
                    <p>Copy one or more decisions to the clipboard without removing them from the transcription:</p>
                    <ul>
                        <li>Press <strong>Ctrl+C</strong> or <strong>y</strong> (vim-like yank) to copy the currently selected decision(s)</li>
                        <li>Click the <strong>copy</strong> button in the toolbar (clipboard icon with two documents)</li>
                        <li>Right-click on a decision and select "Copy" from the context menu</li>
                    </ul>
                    <p>You can copy a single decision or a multi-selection range. The copied decisions remain in the clipboard until you copy or cut something else.</p>
                    
                    <h4>Cut Decisions</h4>
                    <p>Cut decisions to move them to another location:</p>
                    <ul>
                        <li>Press <strong>Ctrl+X</strong> to cut the currently selected decision(s)</li>
                        <li>Press <strong>dd</strong> (vim-like) to cut the currently selected decision(s)</li>
                        <li>Click the <strong>cut</strong> button in the toolbar (scissors icon)</li>
                        <li>Right-click on a decision and select "Cut" from the context menu</li>
                    </ul>
                    <p>Cut decisions are removed from the transcription and placed in the clipboard. All subsequent decisions shift up to fill the gap. The clipboard is automatically cleared after you paste cut decisions.</p>
                    <p><strong>Note:</strong> The <strong>dd</strong> command is now unified for both cut and delete - it cuts the decision (removes it and places it in the clipboard). The <strong>Del</strong> key also copies decisions to clipboard before deleting.</p>
                    
                    <h4>Paste Decisions</h4>
                    <p>Paste decisions from the clipboard to a new location:</p>
                    <ul>
                        <li>Press <strong>Ctrl+V</strong> or <strong>p</strong> to paste after the current decision</li>
                        <li>Press <strong>P</strong> (Shift+p) to paste before the current decision</li>
                        <li>Click the <strong>paste</strong> button in the toolbar to open the paste panel (choose before/after)</li>
                        <li>Right-click on a decision and select "Paste Before" or "Paste After" from the context menu (pastes directly)</li>
                    </ul>
                    <p>The keyboard shortcuts <strong>Ctrl+V</strong>, <strong>p</strong>, and <strong>P</strong> paste immediately at the specified position.</p>
                    <p>When you use the toolbar button, a paste panel appears at the bottom of the screen where you can choose whether to paste before or after the current decision. Press <strong>Enter</strong> to confirm or <strong>Esc</strong> to cancel.</p>
                    <p>When you use the context menu, the paste operation executes immediately at the selected position.</p>
                    <p>All decisions in the clipboard are inserted sequentially, preserving their relative order. The position cache is automatically recalculated after pasting.</p>
                    
                    <h3>Undo/Redo</h3>
                    <p>lazyBG includes a comprehensive undo/redo system that tracks all changes to your transcription:</p>
                    <ul>
                        <li><strong>Undo</strong> - Press <strong>Ctrl+Z</strong> or <strong>u</strong> (vim-like, when not in EDIT mode) to undo the last change</li>
                        <li><strong>Redo</strong> - Press <strong>Ctrl+Y</strong> or <strong>Ctrl+R</strong> to redo a previously undone change</li>
                        <li>Click the <strong>undo</strong> or <strong>redo</strong> buttons in the toolbar (located before the insert decision button)</li>
                        <li>Right-click on any move to access undo/redo from the context menu</li>
                    </ul>
                    <p>The undo/redo system maintains a history of up to 100 states and tracks all modifications including:</p>
                    <ul>
                        <li>Inserting and deleting decisions</li>
                        <li>Editing move details (dice, moves, cube actions)</li>
                        <li>Modifying game metadata</li>
                    </ul>
                    <p>When you undo or redo, the position cache is automatically cleared and recalculated to ensure accurate board positions. The undo/redo buttons are disabled when no undo or redo actions are available.</p>
                    
                    <h3>Candidate Moves Analysis (gnubg)</h3>
                    <p>lazyBG integrates the powerful gnubg backgammon engine to provide move analysis. When you select a decision with dice values, the candidate moves panel automatically displays the top 10 moves suggested by gnubg, ranked from best to worst.</p>
                    
                    <h4>Features</h4>
                    <ul>
                        <li><strong>Automatic Analysis:</strong> When dice values are defined for a position, gnubg instantly analyzes and displays candidate moves</li>
                        <li><strong>Best Move Highlighting:</strong> The best move is shown in green with its equity value</li>
                        <li><strong>Current Move Highlighting:</strong> If your current move matches one of gnubg's suggestions, it's highlighted in yellow</li>
                        <li><strong>Move Statistics:</strong> Each candidate shows equity, equity difference from best, and probability statistics (Win, Win Gammon, Lose, Lose Gammon)</li>
                        <li><strong>Instant Updates:</strong> When you change dice values in edit mode, candidate moves update automatically</li>
                    </ul>
                    
                    <h4>Using Candidate Moves</h4>
                    <ul>
                        <li><strong>View Candidates:</strong> Press <strong>Ctrl+L</strong> to toggle the candidate moves panel</li>
                        <li><strong>Navigate Moves:</strong> Use <strong>j</strong>/<strong>k</strong> or <strong>↓</strong>/<strong>↑</strong> arrow keys to cycle through candidate moves</li>
                        <li><strong>Select a Move:</strong> Press <strong>Enter</strong> or click on a candidate move to apply it to the current decision</li>
                        <li><strong>In Edit Mode:</strong> After changing dice values, the move field is automatically cleared and set to the best move suggested by gnubg</li>
                    </ul>
                    
                    <h4>Move Notation</h4>
                    <p>Candidate moves use standard backgammon notation:</p>
                    <ul>
                        <li><strong>24/20 13/8</strong> - Move from point 24 to 20, and from 13 to 8</li>
                        <li><strong>bar/22</strong> - Enter from the bar to point 22</li>
                        <li><strong>6/off 5/off</strong> - Bear off checkers from points 6 and 5</li>
                    </ul>
                    
                    <p><strong>Note:</strong> The gnubg engine and all required data files are embedded in lazyBG, making it completely standalone with no external dependencies.</p>
                    
                    <h3>COMMAND Mode</h3>
                    <p>COMMAND mode provides quick access to common operations via a command line interface. Press <strong>SPACE</strong> to enter COMMAND mode, type your command, and press <strong>ENTER</strong> to execute it.</p>
                {/if}

                {#if activeTab === 'shortcuts'}
                    <h3>Transcription</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>Ctrl + N</td>
                                <td>New Transcription</td>
                            </tr>
                            <tr>
                                <td>Ctrl + O</td>
                                <td>Open Transcription</td>
                            </tr>
                            <tr>
                                <td>Ctrl + S</td>
                                <td>Export Match</td>
                            </tr>
                            <tr>
                                <td>Ctrl + Q</td>
                                <td>Exit lazyBG</td>
                            </tr>
                            <tr>
                                <td>Ctrl + M</td>
                                <td>Edit Metadata</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>Navigation</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>PageUp, h</td>
                                <td>Previous Game (First Move)</td>
                            </tr>
                            <tr>
                                <td>Left, k</td>
                                <td>Previous Decision</td>
                            </tr>
                            <tr>
                                <td>Right, j</td>
                                <td>Next Decision</td>
                            </tr>
                            <tr>
                                <td>PageDown, l</td>
                                <td>Next Game (First Move)</td>
                            </tr>
                            <tr>
                                <td>Ctrl + K</td>
                                <td>Go To Move</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>Modes</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>Tab</td>
                                <td>Toggle Edit Mode</td>
                            </tr>
                            <tr>
                                <td>Space</td>
                                <td>Command Mode</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>View</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>t</td>
                                <td>Toggle Initial/Final Position Display</td>
                            </tr>
                            <tr>
                                <td>Ctrl + L</td>
                                <td>Toggle Candidate Moves Panel</td>
                            </tr>
                            <tr>
                                <td>Ctrl + H, ?</td>
                                <td>Open Help</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>Game Manipulation</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>n</td>
                                <td>Insert New Game After current</td>
                            </tr>
                            <tr>
                                <td>N (Shift+n)</td>
                                <td>Insert New Game Before current</td>
                            </tr>
                            <tr>
                                <td>D (Shift+d)</td>
                                <td>Delete Current Game</td>
                            </tr>
                            <tr>
                                <td>Game Icon Button (Toolbar)</td>
                                <td>Open Insert Game Panel</td>
                            </tr>
                            <tr>
                                <td>Game Delete Button (Toolbar)</td>
                                <td>Delete Current Game</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>Decision Manipulation</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>o</td>
                                <td>Insert Empty Decision After current</td>
                            </tr>
                            <tr>
                                <td>O (Shift+o)</td>
                                <td>Insert Empty Decision Before current</td>
                            </tr>
                            <tr>
                                <td>dd</td>
                                <td>Cut Current Decision or Selection (deletes and copies to clipboard)</td>
                            </tr>
                            <tr>
                                <td>Del, Delete</td>
                                <td>Delete Current Decision or Selection</td>
                            </tr>
                            <tr>
                                <td>Ctrl+Z, u</td>
                                <td>Undo Last Change</td>
                            </tr>
                            <tr>
                                <td>Ctrl+Y, Ctrl+R</td>
                                <td>Redo Last Undone Change</td>
                            </tr>
                            <tr>
                                <td>Click+Drag</td>
                                <td>Select Multiple Decisions (range)</td>
                            </tr>
                            <tr>
                                <td>Shift+Click</td>
                                <td>Extend Selection to Clicked Decision</td>
                            </tr>
                            <tr>
                                <td>Shift+J / Shift+K</td>
                                <td>Extend Selection Down / Up</td>
                            </tr>
                            <tr>
                                <td>Ctrl+C, y</td>
                                <td>Copy Selected Decision(s)</td>
                            </tr>
                            <tr>
                                <td>Ctrl+X, dd</td>
                                <td>Cut Selected Decision(s)</td>
                            </tr>
                            <tr>
                                <td>Ctrl+V, p</td>
                                <td>Paste After current decision</td>
                            </tr>
                            <tr>
                                <td>P (Shift+p)</td>
                                <td>Paste Before current decision</td>
                            </tr>
                            <tr>
                                <td>+ Button (Toolbar)</td>
                                <td>Open Insert Decision Panel</td>
                            </tr>
                            <tr>
                                <td>− Button (Toolbar)</td>
                                <td>Delete Current Decision or Selection</td>
                            </tr>
                            <tr>
                                <td>Copy/Cut/Paste Buttons (Toolbar)</td>
                                <td>Copy, Cut, or Paste Selected Decision(s)</td>
                            </tr>
                            <tr>
                                <td>Right-click (Normal Mode)</td>
                                <td>Show Context Menu with Insert/Delete/Copy/Cut/Paste/Undo/Redo Options</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>Search</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>Ctrl + F</td>
                                <td>Open Search Modal</td>
                            </tr>
                            <tr>
                                <td>Ctrl + ]</td>
                                <td>Next Search Result</td>
                            </tr>
                            <tr>
                                <td>Ctrl + [</td>
                                <td>Previous Search Result</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>Command Line</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Shortcut</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>Up</td>
                                <td>Browse Command History Up</td>
                            </tr>
                            <tr>
                                <td>Down</td>
                                <td>Browse Command History Down</td>
                            </tr>
                            <tr>
                                <td>Escape</td>
                                <td>Exit Command Mode</td>
                            </tr>
                        </tbody>
                    </table>

                {/if}

                {#if activeTab === 'commands'}
                    <h3>Available Commands</h3>
                    <table>
                        <thead>
                            <tr>
                                <th>Command</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>new, ne, n</td>
                                <td>Create a new transcription</td>
                            </tr>
                            <tr>
                                <td>open, op, o</td>
                                <td>Open an existing transcription</td>
                            </tr>
                            <tr>
                                <td>export, e</td>
                                <td>Export match</td>
                            </tr>
                            <tr>
                                <td>quit, q</td>
                                <td>Exit lazyBG</td>
                            </tr>
                            <tr>
                                <td>meta</td>
                                <td>Edit transcription metadata</td>
                            </tr>
                            <tr>
                                <td>help, he, h</td>
                                <td>Open help window</td>
                            </tr>
                            <tr>
                                <td>[number]</td>
                                <td>Go to a specific move number</td>
                            </tr>
                            <tr>
                                <td>clear, cl</td>
                                <td>Clear command history</td>
                            </tr>
                            <tr>
                                <td>s [query]</td>
                                <td>Search for moves</td>
                            </tr>
                        </tbody>
                    </table>

                    <h3>Search Commands</h3>
                    <p>Use the command <code>s</code> followed by a query to search for moves:</p>
                    <table>
                        <thead>
                            <tr>
                                <th>Command</th>
                                <th>Description</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>s [2 digits]</td>
                                <td>Search for moves with specific dice (e.g., <code>s 32</code> or <code>s 65</code>)</td>
                            </tr>
                            <tr>
                                <td>s d</td>
                                <td>Search for cube doubles</td>
                            </tr>
                            <tr>
                                <td>s t</td>
                                <td>Search for cube takes</td>
                            </tr>
                            <tr>
                                <td>s p</td>
                                <td>Search for cube passes (drops)</td>
                            </tr>
                        </tbody>
                    </table>
                    <p><em>Note: After searching via command line, use <code>Ctrl + ]</code> and <code>Ctrl + [</code> to navigate between results.</em></p>
                {/if}

                {#if activeTab === 'about'}
                    
                    <h3>Version</h3>
                    <p>Application version: {applicationVersion}</p>
                    <p>Transcription format version: {LAZYBG_VERSION}</p>
                    
                    <h3>Author</h3>
                    <p><strong>Kévin Unger &lt;lazybg@proton.me&gt;</strong></p>
                    <p>You can also find me on Heroes under the nickname <strong>postmanpat</strong>.</p>
                    <p>I created lazyBG as a lightweight and efficient tool for quickly transcribing backgammon matches. Feel free to write to me to share your feedback.</p>
                    <p>Here are several ways to reach out:</p>
                    <ul>
                        <li>Discuss with me if we meet in a tournament,</li>
                        <li>Send me an email,</li>
                    </ul>
                    <h3>License</h3>
                    <p>lazyBG is licensed under the MIT License. This means you are free to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the software, provided that the original copyright notice and this permission notice are included in all copies or substantial portions of the software.</p>
                {/if}
            </div>
        </div>
    </div>
{/if}

<style>
    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.5);
        display: flex;
        justify-content: center;
        align-items: center;
        z-index: 1000;
    }

    .modal-content {
        background-color: white;
        padding: 0; /* Remove padding */
        border-radius: 4px;
        width: 80%;
        height: 70%; /* Fix height to 70% of the viewport */
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.2);
        position: relative;
        display: flex;
        flex-direction: column;
    }

    .close-button {
        position: absolute;
        top: -8px;
        right: 4px;
        font-size: 24px;
        font-weight: bold;
        color: #666;
        cursor: pointer;
        z-index: 10;
        transition: background-color 0.3s ease, opacity 0.3s ease;
    }

    .tab-header {
        display: flex;
        margin-bottom: 0; /* Remove bottom margin */
        height: auto; /* Adjust height */
        padding: 0; /* Remove padding */
    }

    .tab-header button {
        flex: 1;
        padding: 0; /* Remove padding */
        background-color: #eee;
        border: none;
        cursor: pointer;
        font-size: 16px;
        outline: none;
        display: flex; /* Use flexbox */
        justify-content: center; /* Center horizontally */
        align-items: center; /* Center vertically */
        text-align: center; /* Center text */
        line-height: 35px; /* Ensure text is centered vertically */
        height: 35px; /* Set a fixed height */
        border-radius: 4px 4px 0 0; /* Add rounded corners to the top */
    }

    .tab-header button.active {
        background-color: #ccc;
        font-weight: bold;
    }

    .tab-content {
        flex-grow: 1;
        overflow-y: auto;
        border-top: 1px solid #ddd;
        padding: 0; /* Remove padding */
        box-sizing: border-box;
        height: calc(100% - 50px); /* Adjust height to ensure uniform tab size */
    }

    .tab-content p, .tab-content ul, .tab-content h2, .tab-content h3, .tab-content h4 {
        margin: 0 20px 20px 20px; /* Add bottom margin for spacing */
        text-align: justify;
    }
    
    .tab-content h4 {
        font-size: 1.1em;
        margin-top: 15px;
        margin-bottom: 10px;
    }

    table {
        margin: 0 auto;
        width: 80%;
        border-collapse: collapse;
    }

    th, td {
        padding: 12px;
        text-align: center;
        border-bottom: 1px solid #ddd;
        width: 50%;
    }

    th {
        background-color: #f4f4f4;
    }

    tr:hover {
        background-color: #f1f1f1;
    }
</style>

