import { useCallback, useEffect, useRef, useState, type SetStateAction } from 'react';

interface NavigationState {
  entries: string[];
  index: number;
}

interface UseBrowseHistoryNavigationParams {
  isSearchMode: boolean;
  selectedPath: string;
  selectedSpaceId?: number;
  incomingSearchQuery: string;
  onNavigate: (path: string) => void;
  onNavigateSearch: (query: string) => void;
}

interface UseBrowseHistoryNavigationResult {
  canGoBack: boolean;
  canGoForward: boolean;
  goBack: () => void;
  goForward: () => void;
}

export function useBrowseHistoryNavigation({
  isSearchMode,
  selectedPath,
  selectedSpaceId,
  incomingSearchQuery,
  onNavigate,
  onNavigateSearch,
}: UseBrowseHistoryNavigationParams): UseBrowseHistoryNavigationResult {
  const [navigationState, setNavigationState] = useState<NavigationState>({ entries: [], index: -1 });
  const historySpaceIdRef = useRef<number | undefined>(undefined);
  const isHistoryTraversalRef = useRef(false);

  useEffect(() => {
    let frame: number | null = null;
    const scheduleNavigationUpdate = (updater: SetStateAction<NavigationState>) => {
      frame = window.requestAnimationFrame(() => {
        setNavigationState(updater);
      });
    };

    if (isSearchMode) {
      scheduleNavigationUpdate({ entries: [], index: -1 });
      historySpaceIdRef.current = undefined;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    if (selectedSpaceId === undefined) {
      scheduleNavigationUpdate({ entries: [], index: -1 });
      historySpaceIdRef.current = undefined;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    if (historySpaceIdRef.current !== selectedSpaceId) {
      historySpaceIdRef.current = selectedSpaceId;
      scheduleNavigationUpdate({ entries: [selectedPath], index: 0 });
      isHistoryTraversalRef.current = false;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    if (isHistoryTraversalRef.current) {
      isHistoryTraversalRef.current = false;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    scheduleNavigationUpdate((prev) => {
      if (prev.index >= 0 && prev.entries[prev.index] === selectedPath) {
        return prev;
      }
      const entries = prev.index >= 0
        ? [...prev.entries.slice(0, prev.index + 1), selectedPath]
        : [selectedPath];
      return { entries, index: entries.length - 1 };
    });

    return () => {
      if (frame !== null) {
        window.cancelAnimationFrame(frame);
      }
    };
  }, [isSearchMode, selectedPath, selectedSpaceId]);

  const goBack = useCallback(() => {
    if (navigationState.index > 0) {
      const nextIndex = navigationState.index - 1;
      const targetPath = navigationState.entries[nextIndex];
      isHistoryTraversalRef.current = true;
      setNavigationState((prev) => ({ ...prev, index: nextIndex }));
      onNavigate(targetPath);
      return;
    }

    if (!incomingSearchQuery) {
      return;
    }

    onNavigateSearch(incomingSearchQuery);
  }, [incomingSearchQuery, navigationState, onNavigate, onNavigateSearch]);

  const goForward = useCallback(() => {
    if (navigationState.index < 0 || navigationState.index >= navigationState.entries.length - 1) {
      return;
    }
    const nextIndex = navigationState.index + 1;
    const targetPath = navigationState.entries[nextIndex];
    isHistoryTraversalRef.current = true;
    setNavigationState((prev) => ({ ...prev, index: nextIndex }));
    onNavigate(targetPath);
  }, [navigationState, onNavigate]);

  const canGoBack = navigationState.index > 0 || (!isSearchMode && Boolean(incomingSearchQuery));
  const canGoForward = navigationState.index >= 0 && navigationState.index < navigationState.entries.length - 1;

  return {
    canGoBack,
    canGoForward,
    goBack,
    goForward,
  };
}
