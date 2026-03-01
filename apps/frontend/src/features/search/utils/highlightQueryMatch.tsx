import type { ReactNode } from 'react';

const DEFAULT_HIGHLIGHT_CLASS_NAME = 'search-match-highlight';

export function highlightQueryMatch(
  text: string,
  query: string,
  highlightClassName: string = DEFAULT_HIGHLIGHT_CLASS_NAME,
): ReactNode {
  const normalizedText = text;
  const normalizedQuery = query.trim();
  if (!normalizedText || !normalizedQuery) {
    return normalizedText;
  }

  const lowerText = normalizedText.toLowerCase();
  const lowerQuery = normalizedQuery.toLowerCase();
  const highlightedNodes: ReactNode[] = [];

  let cursor = 0;
  let matchIndex = lowerText.indexOf(lowerQuery, cursor);
  if (matchIndex < 0) {
    return normalizedText;
  }

  let matchSequence = 0;
  while (matchIndex >= 0) {
    if (matchIndex > cursor) {
      highlightedNodes.push(normalizedText.slice(cursor, matchIndex));
    }

    const matchEnd = matchIndex + normalizedQuery.length;
    highlightedNodes.push(
      <mark key={`search-highlight-${matchSequence}`} className={highlightClassName}>
        {normalizedText.slice(matchIndex, matchEnd)}
      </mark>,
    );

    cursor = matchEnd;
    matchIndex = lowerText.indexOf(lowerQuery, cursor);
    matchSequence += 1;
  }

  if (cursor < normalizedText.length) {
    highlightedNodes.push(normalizedText.slice(cursor));
  }

  return highlightedNodes;
}
