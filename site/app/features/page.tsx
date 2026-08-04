import { PageIntro } from "../components/PageIntro";
import { ScreenshotViewer } from "../components/ScreenshotViewer";
import { SiteShell } from "../components/SiteShell";
import { features, screenshots } from "../content";

const workflowStories = [
  {
    number: "01",
    overline: "Git 版本",
    title: "比较真实工作区与本地 Git 引用",
    body: "仓库模式读取尚未提交的工作区文件，并从经过校验的 Git 引用读取右侧版本。整个过程不会 checkout、fetch 或切换分支。",
    points: ["工作区保持可编辑", "Git 快照始终只读", "保存不自动 add 或 commit"],
  },
  {
    number: "02",
    overline: "记录对齐",
    title: "按主键对齐同一条记录",
    body: "双方存在可靠主键时，SheetProof 会把共同记录重新对齐，并把单侧记录留在原来的业务邻近位置。没有可靠主键时才回退到物理行号。",
    points: ["自动识别 id 与唯一的 *ID 表头", "可从列标题右键指定主键", "歧义记录只保守影响自身"],
  },
  {
    number: "03",
    overline: "合并保存",
    title: "将确认结果写回左侧",
    body: "选择确认过的单元格或整行写入左侧；编辑、覆盖和追加都进入撤销历史。保存前还会检测外部修改并校验临时文件。",
    points: ["左侧是唯一可写结果", "逐格、整行合并与撤销", "重开校验与原子替换"],
  },
];

const supportingFeatures = [features[1], features[2], features[4], features[6]];
const screenshotById = (id: string) => {
  const screenshot = screenshots.find((item) => item.id === id);
  if (!screenshot) throw new Error(`Missing screenshot metadata: ${id}`);
  return screenshot;
};

const keyedScreenshot = screenshotById("direct-compare");
const positionedScreenshot = screenshotById("row-number-comparison");
const beforeMergeScreenshot = screenshotById("review-difference");
const afterMergeScreenshot = screenshotById("merge-applied");

export default function FeaturesPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="功能" title="Git 配置表的比较、合并与保存" description="SheetProof 比较工作区与 Git 引用中的 .xlsx，按主键对齐记录，并将确认结果写回左侧。" aside={<><strong>比较范围</strong><code>值 · 公式 · 类型 · 工作表</code><span>样式不计入内容差异</span></>} />
    <section className="section page-width capability-stories">
      {workflowStories.map((story) => <article key={story.number}>
        <div className="capability-story-number"><span>{story.number}</span><em>{story.overline}</em></div>
        <div className="capability-story-copy"><h2>{story.title}</h2><p>{story.body}</p></div>
        <ul>{story.points.map((point) => <li key={point}>{point}</li>)}</ul>
      </article>)}
    </section>
    <section className="section supporting-section">
      <div className="page-width">
        <div className="section-heading compact"><p className="eyebrow">辅助功能</p><h2>大表审阅与差异定位</h2><p>提供同步滚动、差异筛选、修改前回看和安全保存。</p></div>
        <div className="supporting-grid">
          {supportingFeatures.map((feature) => <article key={feature.title}><p className="overline">{feature.title}</p><h3>{feature.titleZh}</h3><p>{feature.summaryZh}</p></article>)}
        </div>
      </div>
    </section>
    <section className="section page-width screenshot-section">
      <div className="section-heading compact"><p className="eyebrow">功能对比</p><h2>使用同一组数据直接比较</h2><p>所有图片均来自当前版本的 Windows 应用和同一组角色配置表。</p></div>
      <div className="screenshot-comparisons">
        <section className="screenshot-comparison-group" aria-labelledby="alignment-comparison-title">
          <div className="screenshot-comparison-heading"><span>01</span><div><h3 id="alignment-comparison-title">记录插入后的比较结果</h3><p>按行号会使后续记录错位；按主键只保留真实新增和字段修改。</p></div></div>
          <div className="screenshot-comparison-grid">
            <article><strong>按行号 · 90 处差异</strong><ScreenshotViewer src={positionedScreenshot.src} alt={positionedScreenshot.alt} caption={positionedScreenshot.caption} width={positionedScreenshot.width} height={positionedScreenshot.height} /></article>
            <article className="is-recommended"><strong>按主键 · 16 处差异</strong><ScreenshotViewer src={keyedScreenshot.src} alt={keyedScreenshot.alt} caption={keyedScreenshot.caption} width={keyedScreenshot.width} height={keyedScreenshot.height} /></article>
          </div>
        </section>
        <section className="screenshot-comparison-group" aria-labelledby="merge-comparison-title">
          <div className="screenshot-comparison-heading"><span>02</span><div><h3 id="merge-comparison-title">选择性合并前后</h3><p>只合并确认的生命值，其他差异保持不变。</p></div></div>
          <div className="screenshot-comparison-grid">
            <article><strong>合并前 · 16 处差异</strong><ScreenshotViewer src={beforeMergeScreenshot.src} alt={beforeMergeScreenshot.alt} caption={beforeMergeScreenshot.caption} width={beforeMergeScreenshot.width} height={beforeMergeScreenshot.height} /></article>
            <article className="is-recommended"><strong>合并后 · 15 处差异</strong><ScreenshotViewer src={afterMergeScreenshot.src} alt={afterMergeScreenshot.alt} caption={afterMergeScreenshot.caption} width={afterMergeScreenshot.width} height={afterMergeScreenshot.height} /></article>
          </div>
        </section>
      </div>
    </section>
    <section id="limits" className="section page-width limits-section">
      <div className="section-heading compact"><p className="eyebrow">支持范围</p><h2>当前支持什么</h2><p>SheetProof 处理常见 .xlsx 内容和本地版本工作流，以下范围暂不支持。</p></div>
      <div className="limits-grid">
        <article><h3>文件格式</h3><p>只支持 .xlsx；不支持 .xls、.xlsm、.ods、CSV 和加密工作簿。</p></article>
        <article><h3>高级对象</h3><p>不编辑图表、图片、透视表、宏、外部连接或复杂条件格式，也不承诺完整保真。</p></article>
        <article><h3>Git 操作</h3><p>不 fetch、pull、push、add、commit，不创建、删除或切换分支。</p></article>
        <article><h3>编辑范围</h3><p>不是 Excel 级编辑器；没有重做、格式工具栏、完整公式计算或整表复制。</p></article>
      </div>
    </section>
  </main></SiteShell>;
}
