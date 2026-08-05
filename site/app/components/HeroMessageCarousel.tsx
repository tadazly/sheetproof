"use client";

import { useEffect, useState } from "react";
import type { Locale } from "../i18n";

const copy = {
  en: { label: "In practice", region: "Usage examples", controls: "Choose an example", show: (n: number, message: string) => `Show example ${n}: ${message}`, messages: ["When one record is inserted or removed, see that record's change without shifting every row below it.", "Review game balance, product configuration, and operational lists managed by stable IDs.", "Compare two versions and merge only approved items; files stay local and every operation is undoable."] },
  "zh-CN": { label: "实际使用", region: "适用说明", controls: "切换说明", show: (n: number, message: string) => `显示第 ${n} 条说明：${message}`, messages: ["插入或删除一条记录时，只显示这条记录的变化，不让后面的内容整段错位。", "适合游戏数值、产品配置、运营清单等按 ID 管理的 XLSX 表格。", "对照两个版本，逐项确认后再合并；文件留在本机，操作可以撤销。"] },
  ja: { label: "利用例", region: "利用例の説明", controls: "説明を切り替え", show: (n: number, message: string) => `${n} 件目の説明を表示：${message}`, messages: ["レコードを 1 件追加・削除しても、その変更だけを表示し、後続の行全体をずらしません。", "安定した ID で管理するゲームバランス、製品設定、運用リストのレビューに適しています。", "2 つのバージョンを比較し、確認した項目だけを反映。ファイルはローカルのままで、操作は取り消せます。"] },
} as const;

export function HeroMessageCarousel({ locale }: { locale: Locale }) {
  const [active, setActive] = useState(0);
  const text = copy[locale];

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const timer = window.setInterval(() => setActive((current) => (current + 1) % text.messages.length), 4800);
    return () => window.clearInterval(timer);
  }, [text.messages.length]);

  return <div className="hero-message-carousel" aria-label={text.region}>
    <div className="hero-message-copy" key={active}><span>{text.label}</span><p>{text.messages[active]}</p></div>
    <div className="hero-message-controls" aria-label={text.controls}>
      {text.messages.map((message, index) => <button
        className={index === active ? "is-active" : undefined}
        type="button"
        aria-label={text.show(index + 1, message)}
        aria-pressed={index === active}
        onClick={() => setActive(index)}
        key={message}
      />)}
    </div>
  </div>;
}
