import { createApp } from "vue";
import App from "./App.vue";
import "./style.css";

function showFatal(reason: unknown) {
  const root = document.querySelector("#app");
  if (!root) return;
  root.replaceChildren();
  const panel = document.createElement("pre");
  panel.className = "fatal-error";
  panel.textContent = `界面发生错误：\n${reason instanceof Error ? reason.stack ?? reason.message : String(reason)}`;
  root.append(panel);
}

window.addEventListener("unhandledrejection", (event) => showFatal(event.reason));
window.addEventListener("error", (event) => showFatal(event.error ?? event.message));

const application = createApp(App);
application.config.errorHandler = (error) => showFatal(error);
application.mount("#app");
