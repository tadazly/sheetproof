import type { ReactNode } from "react";

export function PageIntro({ eyebrow, title, description, aside }: { eyebrow: string; title: string; description: string; aside?: ReactNode }) {
  return <section className="page-intro">
    <div className="page-width page-intro-grid">
      <div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p>{description}</p></div>
      {aside ? <div className="intro-aside">{aside}</div> : null}
    </div>
  </section>;
}
