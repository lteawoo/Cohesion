import { useState, useCallback, useEffect, useRef } from 'react';
import type { RefObject } from 'react';
import type { SelectionBox } from '../types';

interface UseBoxSelectionParams {
  enabled: boolean;
  containerRef: { current: HTMLElement | null };
  itemsRef: RefObject<Map<string, HTMLElement>>;
  selectedItems: Set<string>;
  onSelectionChange: (items: Set<string>) => void;
}

interface UseBoxSelectionReturn {
  isSelecting: boolean;
  selectionBox: SelectionBox | null;
  wasRecentlySelecting: boolean;
}

export function useBoxSelection({
  enabled,
  containerRef,
  itemsRef,
  selectedItems,
  onSelectionChange,
}: UseBoxSelectionParams): UseBoxSelectionReturn {
  const [isSelecting, setIsSelecting] = useState(false);
  const [selectionBox, setSelectionBox] = useState<SelectionBox | null>(null);
  const [wasRecentlySelecting, setWasRecentlySelecting] = useState(false);
  const [modifierKeys, setModifierKeys] = useState({
    ctrl: false,
    shift: false,
  });

  const initialSelection = useRef<Set<string>>(new Set());
  const animationFrameId = useRef<number | null>(null);

  // 키보드 상태 추적
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      setModifierKeys({
        ctrl: e.ctrlKey || e.metaKey,
        shift: e.shiftKey,
      });
    };

    const handleKeyUp = (e: KeyboardEvent) => {
      setModifierKeys({
        ctrl: e.ctrlKey || e.metaKey,
        shift: e.shiftKey,
      });
    };

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('keyup', handleKeyUp);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('keyup', handleKeyUp);
    };
  }, []);

  // 교차 판정 알고리즘 (AABB)
  const getIntersectedItems = useCallback(
    (box: SelectionBox): string[] => {
      const items = itemsRef.current;
      if (!items) return [];

      const intersected: string[] = [];

      // 박스 정규화
      const boxRect = {
        left: Math.min(box.startX, box.currentX),
        top: Math.min(box.startY, box.currentY),
        right: Math.max(box.startX, box.currentX),
        bottom: Math.max(box.startY, box.currentY),
      };

      // 각 아이템 getBoundingClientRect() 비교
      items.forEach((element, path) => {
        const itemRect = element.getBoundingClientRect();

        // AABB 충돌 검사
        if (
          boxRect.left < itemRect.right &&
          boxRect.right > itemRect.left &&
          boxRect.top < itemRect.bottom &&
          boxRect.bottom > itemRect.top
        ) {
          intersected.push(path);
        }
      });

      return intersected;
    },
    [itemsRef]
  );

  // 선택 업데이트 로직
  const updateSelection = useCallback(
    (intersectedPaths: string[]) => {
      if (modifierKeys.ctrl) {
        // Ctrl: 기존 선택 + 교차 항목 토글
        const newSelection = new Set(initialSelection.current);
        intersectedPaths.forEach((path) => {
          if (initialSelection.current.has(path)) {
            newSelection.delete(path);
          } else {
            newSelection.add(path);
          }
        });
        onSelectionChange(newSelection);
      } else if (modifierKeys.shift) {
        // Shift: 기존 선택 + 교차 항목 추가
        const newSelection = new Set([
          ...initialSelection.current,
          ...intersectedPaths,
        ]);
        onSelectionChange(newSelection);
      } else {
        // 일반: 교차 항목만 선택
        onSelectionChange(new Set(intersectedPaths));
      }
    },
    [modifierKeys, onSelectionChange]
  );

  // 마우스 다운: 박스 선택 시작 (컨테이너 내부의 빈 영역만)
  const handleMouseDown = useCallback(
    (e: MouseEvent) => {
      console.log('[BoxSelection] mousedown', { enabled, hasContainer: !!containerRef.current });

      if (!enabled) return;

      // 컨테이너 범위 체크
      const container = containerRef.current;
      if (!container) {
        console.log('[BoxSelection] No container ref');
        return;
      }

      const containerRect = container.getBoundingClientRect();
      const inBounds =
        e.clientX >= containerRect.left &&
        e.clientX <= containerRect.right &&
        e.clientY >= containerRect.top &&
        e.clientY <= containerRect.bottom;

      console.log('[BoxSelection] Range check', {
        click: { x: e.clientX, y: e.clientY },
        container: {
          left: containerRect.left,
          right: containerRect.right,
          top: containerRect.top,
          bottom: containerRect.bottom,
        },
        inBounds,
      });

      if (!inBounds) {
        return; // 컨테이너 밖에서 클릭
      }

      const target = e.target as HTMLElement;
      const isCard = target.closest('.ant-card');
      const isTableRow = target.closest('tr');

      console.log('[BoxSelection] Target check', {
        tagName: target.tagName,
        className: target.className,
        isCard: !!isCard,
        isTableRow: !!isTableRow,
      });

      // 카드나 테이블 행 클릭 시 박스 선택 무시
      if (isCard || isTableRow) {
        console.log('[BoxSelection] Ignored - card or table row');
        return;
      }

      console.log('[BoxSelection] Starting selection');
      e.preventDefault(); // 텍스트 선택 방지
      setIsSelecting(true);
      setSelectionBox({
        startX: e.clientX,
        startY: e.clientY,
        currentX: e.clientX,
        currentY: e.clientY,
      });

      // 초기 선택 상태 저장 (Ctrl/Shift 처리용)
      initialSelection.current = new Set(selectedItems);
    },
    [enabled, selectedItems, containerRef]
  );

  // 마우스 무브: requestAnimationFrame으로 최적화
  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!isSelecting) return;

      if (animationFrameId.current) {
        cancelAnimationFrame(animationFrameId.current);
      }

      animationFrameId.current = requestAnimationFrame(() => {
        setSelectionBox((prev) =>
          prev
            ? {
                ...prev,
                currentX: e.clientX,
                currentY: e.clientY,
              }
            : null
        );

        // 교차 판정 + 선택 업데이트
        if (selectionBox) {
          const updatedBox = {
            ...selectionBox,
            currentX: e.clientX,
            currentY: e.clientY,
          };
          const intersected = getIntersectedItems(updatedBox);
          updateSelection(intersected);
        }
      });
    },
    [isSelecting, selectionBox, getIntersectedItems, updateSelection]
  );

  // 마우스 업: 선택 완료
  const handleMouseUp = useCallback(() => {
    if (animationFrameId.current) {
      cancelAnimationFrame(animationFrameId.current);
    }

    // 마지막 선택 상태 확정
    if (selectionBox) {
      const intersected = getIntersectedItems(selectionBox);
      updateSelection(intersected);

      // 박스 선택이 실제로 발생했으면 플래그 설정
      setWasRecentlySelecting(true);
      // 다음 프레임에서 플래그 해제 (click 이벤트보다 먼저)
      setTimeout(() => setWasRecentlySelecting(false), 0);
    }

    setIsSelecting(false);
    setSelectionBox(null);
  }, [selectionBox, getIntersectedItems, updateSelection]);

  // 이벤트 리스너 등록 (window에 등록하되 handleMouseDown에서 범위 체크)
  useEffect(() => {
    if (!enabled) return;

    window.addEventListener('mousedown', handleMouseDown);
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);

    return () => {
      window.removeEventListener('mousedown', handleMouseDown);
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
      if (animationFrameId.current) {
        cancelAnimationFrame(animationFrameId.current);
      }
    };
  }, [enabled, handleMouseDown, handleMouseMove, handleMouseUp]);

  return {
    isSelecting,
    selectionBox,
    wasRecentlySelecting,
  };
}
