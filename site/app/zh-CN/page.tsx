import HomePage from "../page";
import { pageMetadata } from "../seo";
export const metadata = pageMetadata("zh-CN", "home", "/");
export default function Page() { return <HomePage locale="zh-CN" />; }
