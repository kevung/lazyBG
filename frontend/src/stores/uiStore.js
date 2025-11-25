import { writable, derived } from 'svelte/store';

export const statusBarTextStore = writable('');
export const statusBarModeStore = writable('NORMAL');

export const commandTextStore = writable('');

export const currentPositionIndexStore = writable(0);

export const showMetadataModalStore = writable(false);
export const showMetadataPanelStore = writable(false);

export const showCommandStore = writable(false);

export const showHelpStore = writable(false);
export const showGoToPositionModalStore = writable(false);

export const previousModeStore = writable('NORMAL');

export const showCandidateMovesStore = writable(false); // Candidate moves panel closed by default
export const showMovesTableStore = writable(false); // Moves table closed by default

export const isAnyModalOrPanelOpenStore = derived(
  [
    showMetadataModalStore,
    showCommandStore,
    showHelpStore,
    showGoToPositionModalStore,
    showCandidateMovesStore,
    showMovesTableStore
  ],
  ([
    showMetadataModal,
    showCommand,
    showHelp,
    showGoToPositionModal,
    showCandidateMoves,
    showMovesTable
  ]) => {
    return (
      showMetadataModal ||
      showCommand ||
      showHelp ||
      showGoToPositionModal ||
      showCandidateMoves ||
      showMovesTable
    );
  }
);

export const isAnyModalOpenStore = derived(
  [
    showMetadataModalStore,
    showGoToPositionModalStore,
    showHelpStore,
    showCommandStore
  ],
  ([
    showMetadataModal,
    showGoToPositionModal,
    showHelp,
    showCommand
  ]) => {
    return (
      showMetadataModal ||
      showGoToPositionModal ||
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
