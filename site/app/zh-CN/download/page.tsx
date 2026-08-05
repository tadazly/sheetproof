import DownloadPage from "../../download/page";
import { pageMetadata } from "../../seo";
export const metadata = pageMetadata("zh-CN", "download", "/download");
export default function Page() { return <DownloadPage locale="zh-CN" />; }
