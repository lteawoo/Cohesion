import React, { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { theme } from 'antd';

interface BottomSheetProps {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
  snapPoints?: number[];
  initialSnapIndex?: number;
  closeThresholdRatio?: number;
  zIndex?: number;
}

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);
const SHEET_ANIMATION_MS = 220;
const HANDLE_ZONE_HEIGHT_PX = 20;
const DEFAULT_MAX_HEIGHT_RATIO = 0.92;

const BottomSheet: React.FC<BottomSheetProps> = ({
  open,
  onClose,
  children,
  snapPoints,
  initialSnapIndex,
  closeThresholdRatio = 0.85,
  zIndex = 1200,
}) => {
  const { token } = theme.useToken();
  const [shouldRender, setShouldRender] = useState(open);
  const [isOpenVisual, setIsOpenVisual] = useState(open);
  const sortedSnapPoints = useMemo(
    () => [...new Set(snapPoints ?? [])].filter((p) => p > 0 && p <= 1).sort((a, b) => a - b),
    [snapPoints]
  );

  const [dragOffsetPx, setDragOffsetPx] = useState(0);
  const [isDragging, setIsDragging] = useState(false);
  const [baseHeightPx, setBaseHeightPx] = useState(0);
  const [isContentScrollable, setIsContentScrollable] = useState(false);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const dragOffsetRef = useRef(0);
  const baseHeightRef = useRef(0);
  const closeThresholdRatioRef = useRef(closeThresholdRatio);
  const onCloseRef = useRef(onClose);
  const lastTouchYRef = useRef<number | null>(null);
  const gestureModeRef = useRef<'scroll' | 'drag'>('drag');
  const dragStateRef = useRef<{
    startY: number;
    startOffset: number;
    startedInContent: boolean;
  } | null>(null);
  const activeSnapRef = useRef(0);

  useEffect(() => {
    if (open) {
      dragOffsetRef.current = 0;
      setDragOffsetPx(0);
      setShouldRender(true);
      requestAnimationFrame(() => setIsOpenVisual(true));
      return;
    }
    setIsOpenVisual(false);
    const timer = window.setTimeout(() => setShouldRender(false), SHEET_ANIMATION_MS);
    return () => window.clearTimeout(timer);
  }, [open]);

  useEffect(() => {
    if (typeof initialSnapIndex === 'number' && sortedSnapPoints.length > 0) {
      activeSnapRef.current = clamp(initialSnapIndex, 0, Math.max(sortedSnapPoints.length - 1, 0));
    }
  }, [initialSnapIndex, sortedSnapPoints.length]);

  useEffect(() => {
    if (!shouldRender) {
      return;
    }
    const maxRatio = sortedSnapPoints.length > 0
      ? sortedSnapPoints[sortedSnapPoints.length - 1]
      : DEFAULT_MAX_HEIGHT_RATIO;
    const nextMaxHeight = window.innerHeight * maxRatio;
    const contentHeight = contentRef.current?.scrollHeight ?? 0;
    const naturalHeight = HANDLE_ZONE_HEIGHT_PX + contentHeight;
    const nextBaseHeight = Math.min(naturalHeight, nextMaxHeight);
    const overflow = naturalHeight > nextMaxHeight;

    setBaseHeightPx(nextBaseHeight);
    setIsContentScrollable(overflow);
  }, [shouldRender, sortedSnapPoints, children, isDragging]);

  useEffect(() => {
    baseHeightRef.current = baseHeightPx;
  }, [baseHeightPx]);

  useEffect(() => {
    closeThresholdRatioRef.current = closeThresholdRatio;
  }, [closeThresholdRatio]);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!shouldRender) {
      return;
    }
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [shouldRender]);

  useEffect(() => {
    if (!shouldRender) {
      return;
    }
    const onEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onEscape);
    return () => window.removeEventListener('keydown', onEscape);
  }, [shouldRender, onClose]);

  useEffect(() => {
    if (!shouldRender) {
      return;
    }
    const onResize = () => {
      const maxRatio = sortedSnapPoints.length > 0
        ? sortedSnapPoints[sortedSnapPoints.length - 1]
        : DEFAULT_MAX_HEIGHT_RATIO;
      const nextMaxHeight = window.innerHeight * maxRatio;
      const contentHeight = contentRef.current?.scrollHeight ?? 0;
      const naturalHeight = HANDLE_ZONE_HEIGHT_PX + contentHeight;
      const nextBaseHeight = Math.min(naturalHeight, nextMaxHeight);
      const overflow = naturalHeight > nextMaxHeight;
      setBaseHeightPx(nextBaseHeight);
      setIsContentScrollable(overflow);
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [shouldRender, sortedSnapPoints, isDragging]);

  useEffect(() => {
    if (!shouldRender || !contentRef.current) {
      return;
    }
    const target = contentRef.current;
    const observer = new ResizeObserver(() => {
      const maxRatio = sortedSnapPoints.length > 0
        ? sortedSnapPoints[sortedSnapPoints.length - 1]
        : DEFAULT_MAX_HEIGHT_RATIO;
      const nextMaxHeight = window.innerHeight * maxRatio;
      const contentHeight = target.scrollHeight;
      const naturalHeight = HANDLE_ZONE_HEIGHT_PX + contentHeight;
      const nextBaseHeight = Math.min(naturalHeight, nextMaxHeight);
      const overflow = naturalHeight > nextMaxHeight;
      setBaseHeightPx(nextBaseHeight);
      setIsContentScrollable(overflow);
    });
    observer.observe(target);
    return () => observer.disconnect();
  }, [shouldRender, sortedSnapPoints, isDragging]);

  const beginDrag = (startY: number, startedInContent = false) => {
    dragStateRef.current = {
      startY,
      startOffset: dragOffsetRef.current,
      startedInContent,
    };
    setIsDragging(true);
  };

  const updateDrag = (currentY: number) => {
    if (!dragStateRef.current) {
      return;
    }
    const delta = currentY - dragStateRef.current.startY;
    const maxOffset = Math.max(0, baseHeightRef.current);
    const nextOffset = clamp(dragStateRef.current.startOffset + delta, 0, maxOffset);
    dragOffsetRef.current = nextOffset;
    setDragOffsetPx(nextOffset);
  };

  const endDrag = () => {
    if (!dragStateRef.current) {
      return;
    }
    dragStateRef.current = null;
    setIsDragging(false);

    const currentOffset = dragOffsetRef.current;
    const currentBaseHeight = baseHeightRef.current;
    const closeThreshold = currentBaseHeight * (1 - closeThresholdRatioRef.current);
    if (currentOffset > closeThreshold) {
      onCloseRef.current();
      return;
    }
    activeSnapRef.current = 0;
    dragOffsetRef.current = 0;
    setDragOffsetPx(0);
  };

  const handlePointerDown = (event: React.PointerEvent) => {
    if (event.pointerType === 'touch') {
      return;
    }
    if (!panelRef.current) {
      return;
    }
    panelRef.current.setPointerCapture(event.pointerId);
    const startedInContent = !!(contentRef.current && contentRef.current.contains(event.target as Node));
    beginDrag(event.clientY, startedInContent);
  };

  const handlePointerMove = (event: React.PointerEvent) => {
    if (event.pointerType === 'touch') {
      return;
    }
    updateDrag(event.clientY);
  };

  const handlePointerUp = (event: React.PointerEvent) => {
    if (event.pointerType === 'touch') {
      return;
    }
    endDrag();
  };

  useEffect(() => {
    if (!shouldRender || !panelRef.current) {
      return;
    }
    const panel = panelRef.current;

    const onTouchStart = (event: TouchEvent) => {
      if (event.touches.length === 0) {
        return;
      }
      const startedInContent = !!(contentRef.current && contentRef.current.contains(event.target as Node));
      gestureModeRef.current = startedInContent ? 'scroll' : 'drag';
      lastTouchYRef.current = event.touches[0].clientY;
      beginDrag(event.touches[0].clientY, startedInContent);
    };

    const onTouchMove = (event: TouchEvent) => {
      if (!dragStateRef.current || event.touches.length === 0) {
        return;
      }
      const currentY = event.touches[0].clientY;
      const previousY = lastTouchYRef.current ?? currentY;
      const deltaY = currentY - previousY;
      lastTouchYRef.current = currentY;
      const dragState = dragStateRef.current;
      const contentEl = contentRef.current;

      if (dragState.startedInContent && contentEl) {
        if (gestureModeRef.current === 'scroll') {
          if (deltaY <= 0 || contentEl.scrollTop > 0) {
            dragState.startY = currentY;
            dragState.startOffset = dragOffsetRef.current;
            return;
          }
          dragState.startY = currentY - Math.max(0, deltaY);
          dragState.startOffset = dragOffsetRef.current;
          gestureModeRef.current = 'drag';
        } else if (deltaY <= 0) {
          if (dragOffsetRef.current <= 0) {
            gestureModeRef.current = 'scroll';
            dragState.startY = currentY;
            dragState.startOffset = dragOffsetRef.current;
            return;
          }
        }
      }

      updateDrag(currentY);
      if (dragOffsetRef.current > 0 && event.cancelable) {
        event.preventDefault();
      }
    };

    const onTouchEnd = () => {
      gestureModeRef.current = 'drag';
      lastTouchYRef.current = null;
      endDrag();
    };

    panel.addEventListener('touchstart', onTouchStart, { passive: true });
    panel.addEventListener('touchmove', onTouchMove, { passive: false });
    panel.addEventListener('touchend', onTouchEnd, { passive: true });
    panel.addEventListener('touchcancel', onTouchEnd, { passive: true });

    return () => {
      panel.removeEventListener('touchstart', onTouchStart);
      panel.removeEventListener('touchmove', onTouchMove);
      panel.removeEventListener('touchend', onTouchEnd);
      panel.removeEventListener('touchcancel', onTouchEnd);
    };
  }, [shouldRender]);

  if (!shouldRender) {
    return null;
  }

  return createPortal(
    <div className="bottom-sheet-root" style={{ zIndex }}>
      <div
        className={`bottom-sheet-mask${isOpenVisual ? ' open' : ''}`}
        onClick={onClose}
        style={{ background: 'rgba(0, 0, 0, 0.55)' }}
      />
      <div
        className={`bottom-sheet-panel${isDragging ? ' dragging' : ''}${isOpenVisual ? ' open' : ''}`}
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
        onClick={(event) => event.stopPropagation()}
        style={{
          height: `${Math.max(0, baseHeightPx)}px`,
          transform: isOpenVisual
            ? `translateY(${dragOffsetPx}px)`
            : `translateY(${Math.max(0, baseHeightPx) + 24}px)`,
          background: token.colorBgContainer,
          borderTop: `1px solid ${token.colorBorder}`,
          boxShadow: token.boxShadowSecondary,
        }}
      >
        <div
          className="bottom-sheet-handle-zone"
        >
          <div
            className="bottom-sheet-handle"
            style={{ background: token.colorBorderSecondary }}
          />
        </div>
        <div
          className="bottom-sheet-content"
          ref={contentRef}
          style={{ overflowY: isContentScrollable ? 'auto' : 'visible' }}
        >
          {children}
        </div>
      </div>
    </div>,
    document.body
  );
};

export default BottomSheet;
