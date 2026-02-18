import { useState, useCallback, useEffect, useRef } from 'react';
import type { RefObject } from 'react';
import type { SelectionBox } from '../types';

interface UseBoxSelectionParams {
  enabled: boolean;
  startAreaRef?: { current: HTMLElement | null };
  startAreaOutsetPx?: number;
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
  startAreaRef,
  startAreaOutsetPx = 0,
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
  const scrollAnimationFrameId = useRef<number | null>(null);
  const pointerPositionRef = useRef<{ x: number; y: number } | null>(null);

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

  const toContentPoint = useCallback((clientX: number, clientY: number) => {
    const container = containerRef.current;
    if (!container) return null;

    const containerRect = container.getBoundingClientRect();
    return {
      x: clientX - containerRect.left + container.scrollLeft,
      y: clientY - containerRect.top + container.scrollTop,
    };
  }, [containerRef]);

  // 교차 판정 알고리즘 (AABB)
  const getIntersectedItems = useCallback(
    (box: SelectionBox): string[] => {
      const items = itemsRef.current;
      const container = containerRef.current;
      if (!items || !container) return [];

      const intersected: string[] = [];
      const containerRect = container.getBoundingClientRect();

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
        const contentRect = {
          left: itemRect.left - containerRect.left + container.scrollLeft,
          top: itemRect.top - containerRect.top + container.scrollTop,
          right: itemRect.right - containerRect.left + container.scrollLeft,
          bottom: itemRect.bottom - containerRect.top + container.scrollTop,
        };

        // AABB 충돌 검사
        if (
          boxRect.left < contentRect.right &&
          boxRect.right > contentRect.left &&
          boxRect.top < contentRect.bottom &&
          boxRect.bottom > contentRect.top
        ) {
          intersected.push(path);
        }
      });

      return intersected;
    },
    [itemsRef, containerRef]
  );


  const computeSelectionFromIntersected = useCallback((intersected: string[]) => {
    const intersectedSet = new Set(intersected);

    if (modifierKeys.ctrl) {
      // Ctrl/Cmd: 초기 선택 기준으로 현재 교차 항목만 토글
      const next = new Set(initialSelection.current);
      intersectedSet.forEach((path) => {
        if (initialSelection.current.has(path)) {
          next.delete(path);
        } else {
          next.add(path);
        }
      });
      return next;
    }

    if (modifierKeys.shift) {
      // Shift: 초기 선택에 현재 교차 항목 추가
      return new Set([...initialSelection.current, ...intersectedSet]);
    }

    // 기본: 현재 교차 항목만 선택
    return intersectedSet;
  }, [modifierKeys]);

  // 마우스 다운: 박스 선택 시작 (컨테이너 내부의 빈 영역만)
  const handleMouseDown = useCallback(
    (e: MouseEvent) => {
      if (!enabled) return;

      // 시작 가능 영역 범위 체크 (없으면 컨테이너 기준)
      const startArea = startAreaRef?.current ?? containerRef.current;
      if (!startArea) {
        return;
      }
      const startAreaRect = startArea.getBoundingClientRect();
      const inBounds =
        e.clientX >= (startAreaRect.left - startAreaOutsetPx) &&
        e.clientX <= (startAreaRect.right + startAreaOutsetPx) &&
        e.clientY >= (startAreaRect.top - startAreaOutsetPx) &&
        e.clientY <= (startAreaRect.bottom + startAreaOutsetPx);

      if (!inBounds) {
        return; // 시작 영역 밖 클릭
      }

      const target = e.target as HTMLElement;
      if (target.closest('[data-selection-exclude="true"]')) {
        return;
      }
      if (target.closest('button, input, select, textarea, a, [role="button"], .ant-btn, .ant-select, .ant-select-dropdown')) {
        return;
      }

      // 좌표/스크롤 계산용 컨테이너 체크
      const container = containerRef.current;
      if (!container) {
        return;
      }
      const isModalLayer = target.closest('.ant-modal-root, .ant-modal-mask, .ant-modal-wrap, .ant-modal');
      if (isModalLayer) {
        return;
      }

      const hasOpenModal = Boolean(document.querySelector('.ant-modal-root .ant-modal-mask'));
      if (hasOpenModal) {
        return;
      }

      const isCard = target.closest('.ant-card');
      const isTableRow = target.closest('tr');

      // 카드나 테이블 행 클릭 시 박스 선택 무시
      if (isCard || isTableRow) {
        return;
      }

      e.preventDefault(); // 텍스트 선택 방지
      const contentPoint = toContentPoint(e.clientX, e.clientY);
      if (!contentPoint) return;

      pointerPositionRef.current = { x: e.clientX, y: e.clientY };
      setIsSelecting(true);
      setSelectionBox({
        startX: contentPoint.x,
        startY: contentPoint.y,
        currentX: contentPoint.x,
        currentY: contentPoint.y,
      });

      // 초기 선택 상태 저장 (Ctrl/Shift 처리용)
      initialSelection.current = new Set(selectedItems);
    },
    [enabled, selectedItems, startAreaRef, startAreaOutsetPx, containerRef, toContentPoint]
  );

  // 마우스 무브: requestAnimationFrame으로 최적화
  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!isSelecting || !selectionBox) return;

      if (animationFrameId.current) {
        cancelAnimationFrame(animationFrameId.current);
      }

      animationFrameId.current = requestAnimationFrame(() => {
        pointerPositionRef.current = { x: e.clientX, y: e.clientY };
        const contentPoint = toContentPoint(e.clientX, e.clientY);
        if (!contentPoint) return;

        // 새 박스 생성 (state 업데이트와 교차 판정에 동일한 값 사용)
        const newBox = {
          ...selectionBox,
          currentX: contentPoint.x,
          currentY: contentPoint.y,
        };

        setSelectionBox(newBox);

        // 교차 판정 (실시간 교차 기준)
        const intersected = getIntersectedItems(newBox);
        onSelectionChange(computeSelectionFromIntersected(intersected));
      });
    },
    [isSelecting, selectionBox, getIntersectedItems, onSelectionChange, computeSelectionFromIntersected, toContentPoint]
  );

  // 마우스 업: 선택 완료
  const handleMouseUp = useCallback(() => {
    if (animationFrameId.current) {
      cancelAnimationFrame(animationFrameId.current);
    }

    // 마지막 선택 상태 확정
    if (selectionBox) {
      const intersected = getIntersectedItems(selectionBox);
      onSelectionChange(computeSelectionFromIntersected(intersected));

      // 박스 선택이 실제로 발생했으면 플래그 설정
      setWasRecentlySelecting(true);
      // 다음 프레임에서 플래그 해제 (click 이벤트보다 먼저)
      setTimeout(() => setWasRecentlySelecting(false), 0);
    }

    setIsSelecting(false);
    setSelectionBox(null);
    pointerPositionRef.current = null;
  }, [selectionBox, getIntersectedItems, onSelectionChange, computeSelectionFromIntersected]);

  // 스크롤: 박스 선택 중 스크롤 시 교차 판정 재계산
  const handleScroll = useCallback(() => {
    if (!isSelecting || !selectionBox || !pointerPositionRef.current) return;

    if (scrollAnimationFrameId.current) {
      cancelAnimationFrame(scrollAnimationFrameId.current);
    }

    scrollAnimationFrameId.current = requestAnimationFrame(() => {
      const pointer = pointerPositionRef.current;
      if (!pointer) return;

      const contentPoint = toContentPoint(pointer.x, pointer.y);
      if (!contentPoint) return;

      // 스크롤 변화에 맞춰 현재 박스 끝점도 콘텐츠 좌표로 갱신
      const newBox = {
        ...selectionBox,
        currentX: contentPoint.x,
        currentY: contentPoint.y,
      };
      setSelectionBox(newBox);

      // 현재 selectionBox로 교차 판정 재계산
      const intersected = getIntersectedItems(newBox);
      onSelectionChange(computeSelectionFromIntersected(intersected));
    });
  }, [isSelecting, selectionBox, getIntersectedItems, onSelectionChange, computeSelectionFromIntersected, toContentPoint]);

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
