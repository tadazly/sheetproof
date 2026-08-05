import HomePage from "../page";
import { pageMetadata } from "../seo";
export const metadata = pageMetadata("ja", "home", "/");
export default function Page() { return <HomePage locale="ja" />; }
