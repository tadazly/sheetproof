"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import type { MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent } from "react";

type ScreenshotViewerProps = {
  src: string;
  alt: string;
  caption: string;
  width?: number;
  height?: number;
  className?: string;
};

type ViewTransform = {
  scale: number;
  x: number;
  y: number;
};

type PointerPosition = {
  x: number;
  y: number;
};

type GestureStart = {
  mode: "drag" | "pinch";
  point?: PointerPosition;
  midpoint?: PointerPosition;
  distance?: number;
  transform: ViewTransform;
};

const MIN_SCALE = 1;
const MAX_SCALE = 5;

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function midpoint(first: PointerPosition, second: PointerPosition) {
  return { x: (first.x + second.x) / 2, y: (first.y + second.y) / 2 };
}

function distance(first: PointerPosition, second: PointerPosition) {
  return Math.hypot(second.x - first.x, second.y - first.y);
}

export function ScreenshotViewer({ src, alt, caption, width, height, className = "" }: ScreenshotViewerProps) {
  const reactId = useId();
  const viewerId = `screenshot-viewer-${reactId.replace(/:/g, "")}`;
  const triggerId = `${viewerId}-trigger`;
  const [open, setOpen] = useState(false);
  const [transform, setTransform] = useState<ViewTransform>({ scale: 1, x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const viewerRef = useRef<HTMLDivElement>(null);
  const stageRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);
  const pointersRef = useRef(new Map<number, PointerPosition>());
  const gestureRef = useRef<GestureStart | null>(null);

  const constrainTransform = useCallback((next: ViewTransform): ViewTransform => {
    if (next.scale <= MIN_SCALE) return { scale: MIN_SCALE, x: 0, y: 0 };

    const stage = stageRef.current;
    const image = imageRef.current;
    if (!stage || !image) return next;

    const stageRect = stage.getBoundingClientRect();
    const maxX = Math.max(0, (image.offsetWidth * next.scale - stageRect.width) / 2) + Math.min(48, stageRect.width * .08);
    const maxY = Math.max(0, (image.offsetHeight * next.scale - stageRect.height) / 2) + Math.min(48, stageRect.height * .08);
    return {
      scale: next.scale,
      x: clamp(next.x, -maxX, maxX),
      y: clamp(next.y, -maxY, maxY),
    };
  }, []);

  const resetView = useCallback(() => {
    setTransform({ scale: 1, x: 0, y: 0 });
  }, []);

  const zoomAt = useCallback((targetScale: number, clientX?: number, clientY?: number) => {
    setTransform((current) => {
      const nextScale = clamp(targetScale, MIN_SCALE, MAX_SCALE);
      const stageRect = stageRef.current?.getBoundingClientRect();
      if (!stageRect || nextScale === MIN_SCALE) return { scale: MIN_SCALE, x: 0, y: 0 };

      const pointX = (clientX ?? stageRect.left + stageRect.width / 2) - stageRect.left - stageRect.width / 2;
      const pointY = (clientY ?? stageRect.top + stageRect.height / 2) - stageRect.top - stageRect.height / 2;
      const ratio = nextScale / current.scale;
      return constrainTransform({
        scale: nextScale,
        x: pointX - ratio * (pointX - current.x),
        y: pointY - ratio * (pointY - current.y),
      });
    });
  }, [constrainTransform]);

  const zoomBy = useCallback((delta: number) => {
    setTransform((current) => {
      const nextScale = clamp(current.scale + delta, MIN_SCALE, MAX_SCALE);
      if (nextScale === MIN_SCALE) return { scale: MIN_SCALE, x: 0, y: 0 };
      const ratio = nextScale / current.scale;
      return constrainTransform({
        scale: nextScale,
        x: current.x * ratio,
        y: current.y * ratio,
      });
    });
  }, [constrainTransform]);

  const closeViewer = useCallback(() => {
    setOpen(false);
    resetView();
    pointersRef.current.clear();
    gestureRef.current = null;
  }, [resetView]);

  const openViewer = useCallback(() => {
    setOpen(true);
  }, []);

  useEffect(() => {
    const syncWithHash = () => {
      const hashTarget = document.getElementById(window.location.hash.slice(1));
      const isCurrentViewer = hashTarget === viewerRef.current;
      setOpen(isCurrentViewer);
      if (!isCurrentViewer) resetView();
    };
    syncWithHash();
    window.addEventListener("hashchange", syncWithHash);
    return () => window.removeEventListener("hashchange", syncWithHash);
  }, [resetView]);

  useEffect(() => {
    document.documentElement.classList.add("screenshot-viewer-enhanced");
  }, []);

  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    const previousScrollX = window.scrollX;
    const previousScrollY = window.scrollY;
    document.body.style.overflow = "hidden";
    document.documentElement.classList.add("lightbox-open");

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") closeViewer();
      if (event.key === "+" || event.key === "=") zoomBy(.35);
      if (event.key === "-") zoomBy(-.35);
      if (event.key === "0") resetView();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.documentElement.classList.remove("lightbox-open");
      window.removeEventListener("keydown", handleKeyDown);
      const previousScrollBehavior = document.documentElement.style.scrollBehavior;
      document.documentElement.style.scrollBehavior = "auto";
      window.scrollTo(previousScrollX, previousScrollY);
      document.documentElement.style.scrollBehavior = previousScrollBehavior;
    };
  }, [closeViewer, open, resetView, zoomBy]);

  useEffect(() => {
    if (!open) return;
    const stage = stageRef.current;
    if (!stage) return;

    const handleWheel = (event: WheelEvent) => {
      event.preventDefault();
      const factor = Math.exp(-event.deltaY * .0015);
      setTransform((current) => {
        const nextScale = clamp(current.scale * factor, MIN_SCALE, MAX_SCALE);
        const rect = stage.getBoundingClientRect();
        const pointX = event.clientX - rect.left - rect.width / 2;
        const pointY = event.clientY - rect.top - rect.height / 2;
        const ratio = nextScale / current.scale;
        return constrainTransform({
          scale: nextScale,
          x: pointX - ratio * (pointX - current.x),
          y: pointY - ratio * (pointY - current.y),
        });
      });
    };
    const stopPageGesture = (event: Event) => event.preventDefault();
    stage.addEventListener("wheel", handleWheel, { passive: false });
    stage.addEventListener("touchmove", stopPageGesture, { passive: false });
    stage.addEventListener("gesturestart", stopPageGesture, { passive: false });
    stage.addEventListener("gesturechange", stopPageGesture, { passive: false });
    return () => {
      stage.removeEventListener("wheel", handleWheel);
      stage.removeEventListener("touchmove", stopPageGesture);
      stage.removeEventListener("gesturestart", stopPageGesture);
      stage.removeEventListener("gesturechange", stopPageGesture);
    };
  }, [constrainTransform, open]);

  const beginGesture = useCallback(() => {
    const points = [...pointersRef.current.values()];
    if (points.length >= 2) {
      gestureRef.current = {
        mode: "pinch",
        midpoint: midpoint(points[0], points[1]),
        distance: Math.max(1, distance(points[0], points[1])),
        transform,
      };
      return;
    }
    if (points.length === 1) {
      gestureRef.current = { mode: "drag", point: points[0], transform };
    } else {
      gestureRef.current = null;
    }
  }, [transform]);

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.pointerType === "mouse" && event.button !== 0) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
    setDragging(true);
    beginGesture();
  };

  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!pointersRef.current.has(event.pointerId)) return;
    event.preventDefault();
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
    const gesture = gestureRef.current;
    const points = [...pointersRef.current.values()];
    if (!gesture) return;

    if (gesture.mode === "pinch" && points.length >= 2 && gesture.midpoint && gesture.distance) {
      const currentMidpoint = midpoint(points[0], points[1]);
      const nextScale = clamp(gesture.transform.scale * distance(points[0], points[1]) / gesture.distance, MIN_SCALE, MAX_SCALE);
      const rect = stageRef.current?.getBoundingClientRect();
      if (!rect) return;
      const startX = gesture.midpoint.x - rect.left - rect.width / 2;
      const startY = gesture.midpoint.y - rect.top - rect.height / 2;
      const currentX = currentMidpoint.x - rect.left - rect.width / 2;
      const currentY = currentMidpoint.y - rect.top - rect.height / 2;
      const ratio = nextScale / gesture.transform.scale;
      setTransform(constrainTransform({
        scale: nextScale,
        x: currentX - ratio * (startX - gesture.transform.x),
        y: currentY - ratio * (startY - gesture.transform.y),
      }));
      return;
    }

    if (gesture.mode === "drag" && points.length === 1 && gesture.point && gesture.transform.scale > MIN_SCALE) {
      setTransform(constrainTransform({
        scale: gesture.transform.scale,
        x: gesture.transform.x + points[0].x - gesture.point.x,
        y: gesture.transform.y + points[0].y - gesture.point.y,
      }));
    }
  };

  const handlePointerEnd = (event: ReactPointerEvent<HTMLDivElement>) => {
    pointersRef.current.delete(event.pointerId);
    if (pointersRef.current.size === 0) setDragging(false);
    beginGesture();
  };

  const handleBackdropClick = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget) closeViewer();
  };

  return <>
    <figure id={triggerId} className={`app-shot ${className}`.trim()}>
      <a className="screenshot-open" href={`#${viewerId}`} onClick={(event) => { event.preventDefault(); openViewer(); }} aria-label={`单独查看并缩放：${alt}`}>
        <img src={src} alt={alt} width={width} height={height} />
        <span>查看大图</span>
      </a>
      <figcaption>{caption}</figcaption>
    </figure>
    <div ref={viewerRef} id={viewerId} className={`screenshot-lightbox ${open ? "is-open" : "is-closed"}`} role="dialog" aria-modal="true" aria-label={alt} onClick={handleBackdropClick}>
      <div className="lightbox-toolbar">
        <p><span className="desktop-zoom-hint">滚轮缩放 · 拖动查看</span><span className="mobile-zoom-hint">双指缩放 · 拖动查看</span></p>
        <div className="lightbox-controls" aria-label="图片缩放控制">
          <button type="button" onClick={() => zoomBy(-.35)} aria-label="缩小图片">−</button>
          <button className="lightbox-scale" type="button" onClick={resetView} aria-label="恢复图片原始比例">{Math.round(transform.scale * 100)}%</button>
          <button type="button" onClick={() => zoomBy(.35)} aria-label="放大图片">＋</button>
        </div>
        <a className="lightbox-close" href={`#${triggerId}`} onClick={(event) => { event.preventDefault(); closeViewer(); }} aria-label="关闭大图">关闭</a>
      </div>
      <div
        ref={stageRef}
        className={`lightbox-stage${dragging ? " is-dragging" : ""}`}
        data-scale={transform.scale.toFixed(2)}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerEnd}
        onPointerCancel={handlePointerEnd}
        onDoubleClick={() => transform.scale > 1 ? resetView() : zoomAt(2)}
      >
        <img
          ref={imageRef}
          className="lightbox-image"
          src={src}
          alt={alt}
          width={width}
          height={height}
          draggable="false"
          style={{ transform: `translate(-50%, -50%) translate(${transform.x}px, ${transform.y}px) scale(${transform.scale})` }}
        />
      </div>
      <p className="lightbox-caption">{caption}</p>
    </div>
  </>;
}
