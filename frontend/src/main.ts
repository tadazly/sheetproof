import { createApp } from "vue";
import App from "./App.vue";
import "./style.css";
import { OnFileDrop } from "../wailsjs/runtime/runtime";
import { initializeLocale, t } from "./i18n";

initializeLocale("system");

const wailsWindow = window as Window & { runtime?: { OnFileDrop?: unknown } };
if (typeof wailsWindow.runtime?.OnFileDrop === "function") {
  // Install Wails' DOM drag/drop handlers before Vue mounts. Besides resolving
  // native paths, the handlers prevent WebView2 from navigating to a dropped
  // folder while the Go-side listener performs the actual repository opening.
  OnFileDrop(() => undefined, false);
}

function showFatal(reason: unknown) {
  const root = document.querySelector("#app");
  if (!root) return;
  root.replaceChildren();
  const panel = document.createElement("pre");
  panel.className = "fatal-error";
  panel.textContent = t("errors.interfaceFailure", { details: reason instanceof Error ? reason.stack ?? reason.message : String(reason) });
  root.append(panel);
}

window.addEventListener("unhandledrejection", (event) => showFatal(event.reason));
window.addEventListener("error", (event) => showFatal(event.error ?? event.message));

const application = createApp(App);
application.config.errorHandler = (error) => showFatal(error);
application.mount("#app");
