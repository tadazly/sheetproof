import type { Locale } from "../i18n";

const copy = {
  en: { aria: "Example of SheetProof aligning worktree and Git revision records by key", key: "Key: Map ID", worktree: "Current worktree", editable: "Editable", readonly: "Read-only", worktreeAria: "Current worktree map records", gitAria: "Git revision map records", headers: ["Map ID", "Map Name", "Drop Group"], names: ["Windscar Canyon", "Legacy Season Gate", "Sunken Ruins", "Frostfield Outpost"], missing: "Record not present", deleted: "actual deletion", shifted: "shifted changes", caption: "When a record is removed in the middle, later records with the same ID remain aligned." },
  "zh-CN": { aria: "SheetProof 按主键对齐工作区与 Git 版本记录的示例", key: "主键：地图 ID", worktree: "当前工作区", editable: "可编辑", readonly: "只读", worktreeAria: "当前工作区地图记录", gitAria: "Git 版本地图记录", headers: ["地图 ID", "地图名称", "掉落组"], names: ["风蚀峡谷", "旧赛季入口", "沉没遗迹", "霜原前哨"], missing: "该记录不存在", deleted: "条实际删除", shifted: "条错位修改", caption: "中间删除记录后，后续相同 ID 的记录仍保持对齐。" },
  ja: { aria: "SheetProof がワークツリーと Git リビジョンのレコードをキーで揃える例", key: "キー：マップ ID", worktree: "現在のワークツリー", editable: "編集可能", readonly: "読み取り専用", worktreeAria: "現在のワークツリーのマップレコード", gitAria: "Git リビジョンのマップレコード", headers: ["マップ ID", "マップ名", "ドロップグループ"], names: ["風裂きの峡谷", "旧シーズン入口", "水没遺跡", "霜原の前哨地"], missing: "レコードなし", deleted: "件の実際の削除", shifted: "件の行ずれによる変更", caption: "途中のレコードを削除しても、後続の同じ ID は正しく揃います。" },
} as const;
const ids = ["20044", "20045", "50001", "50002"];
const groups = ["reward_a", "reward_old", "reward_b", "reward_c"];

export function GitReviewDemo({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const rows = ids.map((id, index) => ({ id, name: text.names[index], group: groups[index], removed: index === 1 }));
  return <figure className="git-review-demo" aria-label={text.aria}>
    <div className="demo-topbar">
      <div><span className="demo-file-dot" aria-hidden="true" />configs/maps.xlsx</div>
      <span className="demo-status">{text.key}</span>
    </div>
    <div className="demo-panes">
      <section className="demo-pane">
        <header><span>{text.worktree}</span><em>{text.editable}</em></header>
        <div className="demo-table" role="table" aria-label={text.worktreeAria}>
          <div className="demo-row demo-row-head" role="row">{text.headers.map((header) => <span key={header}>{header}</span>)}</div>
          {rows.map((row) => <div className={`demo-row${row.removed ? " is-removed" : ""}`} role="row" key={row.id}>
            <span>{row.id}</span><span>{row.name}</span><span>{row.group}</span>
          </div>)}
        </div>
      </section>
      <section className="demo-pane">
        <header><span>origin/main</span><em>{text.readonly}</em></header>
        <div className="demo-table" role="table" aria-label={text.gitAria}>
          <div className="demo-row demo-row-head" role="row">{text.headers.map((header) => <span key={header}>{header}</span>)}</div>
          {rows.map((row) => row.removed
            ? <div className="demo-row is-missing" role="row" key={row.id}><span /><span>{text.missing}</span><span /></div>
            : <div className="demo-row" role="row" key={row.id}><span>{row.id}</span><span>{row.name}</span><span>{row.group}</span></div>)}
        </div>
      </section>
    </div>
    <figcaption>
      <div><strong>1</strong><span>{text.deleted}</span></div>
      <div><strong>0</strong><span>{text.shifted}</span></div>
      <p>{text.caption}</p>
    </figcaption>
  </figure>;
}
