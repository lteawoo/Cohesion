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
  const accumulatedSelection = useRef<Set<string>>(new Set());
  const animationFrameId = useRef<number | null>(null);
  const scrollAnimationFrameId = useRef<number | null>(null);

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


  // 마우스 다운: 박스 선택 시작 (컨테이너 내부의 빈 영역만)
  const handleMouseDown = useCallback(
    (e: MouseEvent) => {
      if (!enabled) return;

      // 컨테이너 범위 체크
      const container = containerRef.current;
      if (!container) {
        return;
      }

      const containerRect = container.getBoundingClientRect();
      const inBounds =
        e.clientX >= containerRect.left &&
        e.clientX <= containerRect.right &&
        e.clientY >= containerRect.top &&
        e.clientY <= containerRect.bottom;

      if (!inBounds) {
        return; // 컨테이너 밖에서 클릭
      }

      const target = e.target as HTMLElement;
      const isCard = target.closest('.ant-card');
      const isTableRow = target.closest('tr');

      // 카드나 테이블 행 클릭 시 박스 선택 무시
      if (isCard || isTableRow) {
        return;
      }

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
      // 드래그 중 누적 선택 초기화
      accumulatedSelection.current = new Set();
    },
    [enabled, selectedItems, containerRef]
  );

  // 마우스 무브: requestAnimationFrame으로 최적화
  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!isSelecting || !selectionBox) return;

      if (animationFrameId.current) {
        cancelAnimationFrame(animationFrameId.current);
      }

      animationFrameId.current = requestAnimationFrame(() => {
        // 새 박스 생성 (state 업데이트와 교차 판정에 동일한 값 사용)
        const newBox = {
          ...selectionBox,
          currentX: e.clientX,
          currentY: e.clientY,
        };

        setSelectionBox(newBox);

        // 교차 판정 + 누적
        const intersected = getIntersectedItems(newBox);
        // 한 번이라도 드래그 영역에 걸린 항목들을 누적
        intersected.forEach(path => accumulatedSelection.current.add(path));

        // 최종 선택 계산 (Ctrl/Shift 모드 고려)
        let finalSelection: Set<string>;
        if (modifierKeys.ctrl) {
          // Ctrl: 초기 선택 + 누적 항목 토글
          finalSelection = new Set(initialSelection.current);
          accumulatedSelection.current.forEach(path => {
            if (initialSelection.current.has(path)) {
              finalSelection.delete(path);
            } else {
              finalSelection.add(path);
            }
          });
        } else if (modifierKeys.shift) {
          // Shift: 초기 선택 + 누적 항목 추가
          finalSelection = new Set([
            ...initialSelection.current,
            ...accumulatedSelection.current,
          ]);
        } else {
          // 일반: 누적된 항목만 선택
          finalSelection = new Set(accumulatedSelection.current);
        }

        onSelectionChange(finalSelection);
      });
    },
    [isSelecting, selectionBox, getIntersectedItems, modifierKeys, onSelectionChange]
  );

  // 마우스 업: 선택 완료
  const handleMouseUp = useCallback(() => {
    if (animationFrameId.current) {
      cancelAnimationFrame(animationFrameId.current);
    }

    // 마지막 선택 상태 확정
    if (selectionBox) {
      const intersected = getIntersectedItems(selectionBox);
      // 마지막으로 교차한 항목도 누적에 포함
      intersected.forEach(path => accumulatedSelection.current.add(path));

      // 최종 선택 계산 (Ctrl/Shift 모드 고려)
      let finalSelection: Set<string>;
      if (modifierKeys.ctrl) {
        finalSelection = new Set(initialSelection.current);
        accumulatedSelection.current.forEach(path => {
          if (initialSelection.current.has(path)) {
            finalSelection.delete(path);
          } else {
            finalSelection.add(path);
          }
        });
      } else if (modifierKeys.shift) {
        finalSelection = new Set([
          ...initialSelection.current,
          ...accumulatedSelection.current,
        ]);
      } else {
        finalSelection = new Set(accumulatedSelection.current);
      }

      onSelectionChange(finalSelection);

      // 박스 선택이 실제로 발생했으면 플래그 설정
      setWasRecentlySelecting(true);
      // 다음 프레임에서 플래그 해제 (click 이벤트보다 먼저)
      setTimeout(() => setWasRecentlySelecting(false), 0);
    }

    setIsSelecting(false);
    setSelectionBox(null);
  }, [selectionBox, getIntersectedItems, modifierKeys, onSelectionChange]);

  // 스크롤: 박스 선택 중 스크롤 시 교차 판정 재계산
  const handleScroll = useCallback(() => {
    if (!isSelecting || !selectionBox) return;

    if (scrollAnimationFrameId.current) {
      cancelAnimationFrame(scrollAnimationFrameId.current);
    }

    scrollAnimationFrameId.current = requestAnimationFrame(() => {
      // 현재 selectionBox로 교차 판정 재계산 (박스는 고정, 아이템들이 움직임)
      const intersected = getIntersectedItems(selectionBox);
      // 한 번이라도 드래그 영역에 걸린 항목들을 누적
      intersected.forEach(path => accumulatedSelection.current.add(path));

      // 최종 선택 계산 (Ctrl/Shift 모드 고려)
      let finalSelection: Set<string>;
      if (modifierKeys.ctrl) {
        // Ctrl: 초기 선택 + 누적 항목 토글
        finalSelection = new Set(initialSelection.current);
        accumulatedSelection.current.forEach(path => {
          if (initialSelection.current.has(path)) {
            finalSelection.delete(path);
          } else {
            finalSelection.add(path);
          }
        });
      } else if (modifierKeys.shift) {
        // Shift: 초기 선택 + 누적 항목 추가
        finalSelection = new Set([
          ...initialSelection.current,
          ...accumulatedSelection.current,
        ]);
      } else {
        // 일반: 누적된 항목만 선택
        finalSelection = new Set(accumulatedSelection.current);
      }

      onSelectionChange(finalSelection);
    });
  }, [isSelecting, selectionBox, getIntersectedItems, modifierKeys, onSelectionChange]);

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

  // 스크롤 이벤트 리스너 등록 (컨테이너에 등록)
  useEffect(() => {
    if (!enabled) return;

    const container = containerRef.current;
    if (!container) return;

    container.addEventListener('scroll', handleScroll);

    return () => {
      container.removeEventListener('scroll', handleScroll);
      if (scrollAnimationFrameId.current) {
        cancelAnimationFrame(scrollAnimationFrameId.current);
      }
    };
  }, [enabled, handleScroll, containerRef]);

  return {
    isSelecting,
    selectionBox,
    wasRecentlySelecting,
  };
}
