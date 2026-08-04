"use client";

import { useEffect, useState } from "react";

const messages = [
  "插入或删除一条记录时，只显示这条记录的变化，不让后面的内容整段错位。",
  "适合游戏数值、产品配置、运营清单等按 ID 管理的 XLSX 表格。",
  "对照两个版本，逐项确认后再合并；文件留在本机，操作可以撤销。",
];

export function HeroMessageCarousel() {
  const [active, setActive] = useState(0);

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const timer = window.setInterval(() => setActive((current) => (current + 1) % messages.length), 4800);
    return () => window.clearInterval(timer);
  }, []);

  return <div className="hero-message-carousel" aria-label="适用说明">
    <div className="hero-message-copy" key={active}><span>实际使用</span><p>{messages[active]}</p></div>
    <div className="hero-message-controls" aria-label="切换说明">
      {messages.map((message, index) => <button
        className={index === active ? "is-active" : undefined}
        type="button"
        aria-label={`显示第 ${index + 1} 条说明：${message}`}
        aria-pressed={index === active}
        onClick={() => setActive(index)}
        key={message}
      />)}
    </div>
  </div>;
}
