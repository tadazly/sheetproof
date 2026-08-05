import GuidePage from "../../guide/page";
import { pageMetadata } from "../../seo";
export const metadata = pageMetadata("zh-CN", "guide", "/guide");
export default function Page() { return <GuidePage locale="zh-CN" />; }
