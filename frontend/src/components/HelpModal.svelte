<!-- HelpModal.svelte -->
<script>

    import { onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import { metaStore } from '../stores/metaStore'; // Import metaStore

    export let visible = false;
    export let onClose;
    export let handleGlobalKeydown;

    let activeTab = "manual"; // Default active tab
    const tabs = ['manual', 'shortcuts', 'commands', 'about'];
    let contentArea;

    let databaseVersion = '';
    let applicationVersion = '';

    // Subscribe to the metaStore
    metaStore.subscribe(value => {
        applicationVersion = value.applicationVersion;
    });

    onMount(async () => {
        // Database functionality removed
        databaseVersion = 'N/A';
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
                        <li><strong>EDIT mode:</strong> Modify the board position using the mouse</li>
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
                    <p>EDIT mode allows you to modify board positions using the mouse. Activate it by pressing <strong>TAB</strong>. You can adjust checker positions, cube ownership, dice, and score.</p>
                    
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
                                <td>Previous Move</td>
                            </tr>
                            <tr>
                                <td>Right, j</td>
                                <td>Next Move</td>
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
                                <td>Ctrl + L</td>
                                <td>Toggle Candidate Moves Panel</td>
                            </tr>
                            <tr>
                                <td>Ctrl + H, ?</td>
                                <td>Open Help</td>
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
                                <td>Go to a specific move by index</td>
                            </tr>
                            <tr>
                                <td>clear, cl</td>
                                <td>Clear command history</td>
                            </tr>
                        </tbody>
                    </table>
                {/if}

                {#if activeTab === 'about'}
                    
                    <h3>Version</h3>
                    <p>Application version: {applicationVersion}</p>
                    <p>Transcription format version: {databaseVersion}</p>
                    
                    <h3>Author</h3>
                    <p><strong>Kévin Unger &lt;blunderdb@proton.me&gt;</strong></p>
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

    .tab-content p, .tab-content ul, .tab-content h2, .tab-content h3 {
        margin: 0 20px 20px 20px; /* Add bottom margin for spacing */
        text-align: justify;
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

