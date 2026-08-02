"use client";

import { useEffect, useState } from "react";

type ScreenshotViewerProps = {
  src: string;
  alt: string;
  caption: string;
  width?: number;
  height?: number;
  className?: string;
};

export function ScreenshotViewer({ src, alt, caption, width, height, className = "" }: ScreenshotViewerProps) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  return <>
    <figure className={`app-shot ${className}`.trim()}>
      <button className="screenshot-open" type="button" onClick={() => setOpen(true)} aria-label={`放大查看：${alt}`}>
        <img src={src} alt={alt} width={width} height={height} />
        <span>查看原图</span>
      </button>
      <figcaption>{caption}</figcaption>
    </figure>
    {open && <div className="screenshot-lightbox" role="dialog" aria-modal="true" aria-label={alt} onClick={() => setOpen(false)}>
      <button className="lightbox-close" type="button" onClick={() => setOpen(false)} aria-label="关闭大图">关闭</button>
      <div className="lightbox-stage" onClick={(event) => event.stopPropagation()}>
        <img src={src} alt={alt} width={width} height={height} />
        <p>{caption}</p>
      </div>
    </div>}
  </>;
}
