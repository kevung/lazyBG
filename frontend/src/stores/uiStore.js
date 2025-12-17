import { writable, derived } from 'svelte/store';

export const statusBarTextStore = writable('');
export const statusBarModeStore = writable('NORMAL');

export const commandTextStore = writable('');

export const currentPositionIndexStore = writable(0);

export const showMetadataModalStore = writable(false);
export const showMetadataPanelStore = writable(false);

export const showCommandStore = writable(false);

export const showHelpStore = writable(false);
export const showGoToMoveModalStore = writable(false);
export const showMoveSearchModalStore = writable(false);

export const previousModeStore = writable('NORMAL');

export const showCandidateMovesStore = writable(false); // Candidate moves panel closed by default
export const showMovesTableStore = writable(false); // Moves table closed by default

export const showInitialPositionStore = writable(true); // Show initial position with arrows by default, false = final, true = initial

// Store for candidate move preview (when cycling through candidates with j/k)
// When set, this overrides the move text displayed on the board
export const candidatePreviewMoveStore = writable(null);

export const isAnyModalOrPanelOpenStore = derived(
  [
    showMetadataModalStore,
    showCommandStore,
    showHelpStore,
    showGoToMoveModalStore,
    showMoveSearchModalStore,
    showCandidateMovesStore,
    showMovesTableStore
  ],
  ([
    showMetadataModal,
    showCommand,
    showHelp,
    showGoToMoveModal,
    showMoveSearchModal,
    showCandidateMoves,
    showMovesTable
  ]) => {
    return (
      showMetadataModal ||
      showCommand ||
      showHelp ||
      showGoToMoveModal ||
      showMoveSearchModal ||
      showCandidateMoves ||
      showMovesTable
    );
  }
);

export const isAnyModalOpenStore = derived(
  [
    showMetadataModalStore,
    showGoToMoveModalStore,
    showHelpStore,
    showCommandStore
  ],
  ([
    showMetadataModal,
    showGoToMoveModal,
    showHelp,
    showCommand
  ]) => {
    return (
      showMetadataModal ||
      showGoToMoveModal ||
      showHelp ||
      showCommand
    );
  }
);

export const isAnyPanelOpenStore = derived(
  [
    showCandidateMovesStore,
    showMovesTableStore
  ],
  ([
    showCandidateMoves,
    showMovesTable
  ]) => {
    return (
      showCandidateMoves ||
      showMovesTable
    );
  }
);
