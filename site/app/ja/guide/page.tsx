import GuidePage from "../../guide/page";
import { pageMetadata } from "../../seo";
export const metadata = pageMetadata("ja", "guide", "/guide");
export default function Page() { return <GuidePage locale="ja" />; }
