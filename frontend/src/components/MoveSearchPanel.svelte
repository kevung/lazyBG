<script>
    import { onMount, onDestroy } from 'svelte';
    import { slide } from 'svelte/transition';
    import {
        showMoveSearchModalStore,
        searchQueryStore,
        searchResultsStore,
        currentSearchResultIndexStore,
        executeSearch,
        nextSearchResult,
        previousSearchResult,
        clearSearch
    } from '../stores/moveSearchStore';
    import { statusBarTextStore } from '../stores/uiStore';

    export let visible = false;
    export let onClose;

    let searchInput = '';
    let inputEl;
    let searchResults = [];
    let currentResultIndex = -1;

    // Subscribe to stores
    searchResultsStore.subscribe(value => {
        searchResults = value;
    });

    currentSearchResultIndexStore.subscribe(value => {
        currentResultIndex = value;
    });

    searchQueryStore.subscribe(value => {
        searchInput = value;
    });

    function handleSearch() {
        if (!searchInput.trim()) {
            statusBarTextStore.set('Please enter a search query');
            return;
        }

        executeSearch(searchInput);

        if (searchResults.length === 0) {
            statusBarTextStore.set(`No results found for "${searchInput}"`);
        } else {
            statusBarTextStore.set(`Found ${searchResults.length} result(s) for "${searchInput}"`);
        }
    }

    function handleNext() {
        if (searchResults.length === 0) {
            statusBarTextStore.set('No search results');
            return;
        }
        nextSearchResult();
        statusBarTextStore.set(`Result ${currentResultIndex + 1} of ${searchResults.length}`);
    }

    function handlePrevious() {
        if (searchResults.length === 0) {
            statusBarTextStore.set('No search results');
            return;
        }
        previousSearchResult();
        statusBarTextStore.set(`Result ${currentResultIndex + 1} of ${searchResults.length}`);
    }

    function handleClose() {
        onClose();
    }

    function handleKeyDown(event) {
        event.stopPropagation();

        if (event.key === 'Escape') {
            handleClose();
        } else if (event.ctrlKey && event.code === 'KeyF') {
            event.preventDefault();
            handleClose();
        } else if (event.key === 'Enter') {
            // If we have results, go to next. Otherwise, search.
            if (searchResults.length > 0) {
                handleNext();
            } else {
                handleSearch();
            }
        } else if (event.ctrlKey && event.key === ']') {
            event.preventDefault();
            handleNext();
        } else if (event.ctrlKey && event.key === '[') {
            event.preventDefault();
            handlePrevious();
        }
    }

    let hasAutoFocused = false;
    let panelEl;

    function handleClickOutside(event) {
        if (panelEl && !panelEl.contains(event.target)) {
            // Click outside the panel - blur the input
            inputEl?.blur();
        }
    }

    function focusSearchPanel() {
        inputEl?.focus();
    }

    onMount(() => {
        if (visible && !hasAutoFocused) {
            setTimeout(() => {
                inputEl?.focus();
                hasAutoFocused = true;
            }, 50);
        }
        document.addEventListener('mousedown', handleClickOutside);
        window.addEventListener('focusSearchPanel', focusSearchPanel);
    });

    onDestroy(() => {
        document.removeEventListener('mousedown', handleClickOutside);
        window.removeEventListener('focusSearchPanel', focusSearchPanel);
    });

    $: if (visible && !hasAutoFocused) {
        setTimeout(() => {
            inputEl?.focus();
            hasAutoFocused = true;
        }, 50);
    } else if (!visible) {
        hasAutoFocused = false;
    }
</script>

{#if visible}
    <div class="search-panel" transition:slide={{ duration: 200 }} bind:this={panelEl}>
        <div class="panel-content">
            <input
                type="text"
                bind:this={inputEl}
                bind:value={searchInput}
                on:keydown={handleKeyDown}
                placeholder="Search moves..."
                class="search-input"
            />
            <button on:click={handleSearch} class="search-button" title="Search" aria-label="Search">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="button-icon">
                    <path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
                </svg>
            </button>
            
            {#if searchResults.length > 0}
                <div class="results-badge">
                    {currentResultIndex + 1}/{searchResults.length}
                </div>
                <button on:click={handlePrevious} class="nav-button" title="Previous result (Ctrl-[)" aria-label="Previous result">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="button-icon">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
                    </svg>
                </button>
                <button on:click={handleNext} class="nav-button" title="Next result (Ctrl-])" aria-label="Next result">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="button-icon">
                        <path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
                    </svg>
                </button>
            {/if}
            
            <button class="close-button" on:click={handleClose} title="Close (Esc)" aria-label="Close">×</button>
        </div>
    </div>
{/if}

<style>
    .search-panel {
        position: fixed;
        bottom: 32px; /* Above status bar */
        left: 0;
        right: 0;
        background-color: white;
        border-top: 1px solid #ccc;
        box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.1);
        z-index: 100;
    }

    .panel-content {
        display: flex;
        gap: 8px;
        align-items: center;
        padding: 8px 15px;
    }

    .search-input {
        flex: 1;
        padding: 6px 10px;
        font-size: 14px;
        border: 1px solid #ccc;
        border-radius: 4px;
        outline: none;
    }

    .search-input:focus {
        outline: none;
        border-color: #999;
        box-shadow: none;
    }

    .search-button {
        display: flex;
        align-items: center;
        gap: 4px;
        padding: 6px 10px;
        font-size: 14px;
        background-color: #888;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .search-button:hover {
        background-color: #777;
    }

    .results-badge {
        background-color: #f8f9fa;
        color: #495057;
        padding: 6px 10px;
        border-radius: 4px;
        font-size: 13px;
        font-weight: 500;
        border: 1px solid #dee2e6;
    }

    .nav-button {
        background-color: #888;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        padding: 6px;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background-color 0.2s;
    }

    .nav-button:hover {
        background-color: #777;
    }

    .close-button {
        background: none;
        border: none;
        font-size: 20px;
        font-weight: bold;
        color: #666;
        cursor: pointer;
        padding: 0 6px;
        line-height: 1;
        margin-left: 8px;
    }

    .close-button:hover {
        color: #000;
    }

    .button-icon {
        width: 16px;
        height: 16px;
    }
</style>
