const rows = [
  { id: "20044", name: "风蚀峡谷", group: "reward_a" },
  { id: "20045", name: "旧赛季入口", group: "reward_old", removed: true },
  { id: "50001", name: "远古遗迹", group: "reward_b" },
  { id: "50002", name: "冰原前线", group: "reward_c" },
];

export function GitReviewDemo() {
  return <figure className="git-review-demo" aria-label="SheetProof 按主键对齐 Git 工作区与版本记录的示例">
    <div className="demo-topbar">
      <div><span className="demo-file-dot" aria-hidden="true" />config/mapItem.xlsx</div>
      <span className="demo-status">主键：地图ID</span>
    </div>
    <div className="demo-panes">
      <section className="demo-pane">
        <header><span>当前工作区</span><em>可编辑</em></header>
        <div className="demo-table" role="table" aria-label="当前工作区记录">
          <div className="demo-row demo-row-head" role="row"><span>地图ID</span><span>名称</span><span>掉落组</span></div>
          {rows.map((row) => <div className={`demo-row${row.removed ? " is-removed" : ""}`} role="row" key={row.id}>
            <span>{row.id}</span><span>{row.name}</span><span>{row.group}</span>
          </div>)}
        </div>
      </section>
      <section className="demo-pane">
        <header><span>origin/main</span><em>只读</em></header>
        <div className="demo-table" role="table" aria-label="Git 版本记录">
          <div className="demo-row demo-row-head" role="row"><span>地图ID</span><span>名称</span><span>掉落组</span></div>
          {rows.map((row) => row.removed
            ? <div className="demo-row is-missing" role="row" key={row.id}><span /><span>该记录不存在</span><span /></div>
            : <div className="demo-row" role="row" key={row.id}><span>{row.id}</span><span>{row.name}</span><span>{row.group}</span></div>)}
        </div>
      </section>
    </div>
    <figcaption>
      <div><strong>1</strong><span>条真实删除</span></div>
      <div><strong>0</strong><span>条错位修改</span></div>
      <p>中间增删记录，后续共同 ID 仍保持对齐。</p>
    </figcaption>
  </figure>;
}
