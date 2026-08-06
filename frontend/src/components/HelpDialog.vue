<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import packageInfo from "../../package.json";
import { useI18n } from "../i18n";
import AppIcon from "./AppIcon.vue";

const emit = defineEmits<{ (event: "close"): void }>();
const { locale, t } = useI18n();
const dialog = ref<HTMLElement | null>(null);

const guideURL = computed(() => ({
  en: "https://sheetproof.luyilabs.com/guide/",
  "zh-CN": "https://sheetproof.luyilabs.com/zh-CN/guide/",
  ja: "https://sheetproof.luyilabs.com/ja/guide/"
}[locale.value]));

const shortcutGroups = [
  {
    title: "help.findAndBrowse",
    items: [
      ["help.findCurrent", ["Ctrl / ⌘", "F"]],
      ["help.nextFind", ["F3"]],
      ["help.previousFind", ["Shift", "F3"]],
      ["help.findNext", ["Enter"]],
      ["help.findPrevious", ["Shift", "Enter"]],
      ["help.diffFilters", ["1–4"]],
      ["help.allDiffFilters", ["5"]],
      ["help.selectWorksheet", ["Ctrl / ⌘", "A"]],
      ["help.previewOriginal", ["Tab"]],
      ["help.zoom", ["Ctrl / ⌘", "Mouse wheel"]]
    ]
  },
  {
    title: "help.editAndSave",
    items: [
      ["help.clearSelection", ["Backspace / Delete"]],
      ["help.editCell", ["Double-click"]],
      ["help.commitEdit", ["Enter"]],
      ["help.cancelEdit", ["Esc"]],
      ["help.undo", ["Ctrl / ⌘", "Z"]],
      ["help.save", ["Ctrl / ⌘", "S"]],
      ["help.saveAs", ["Ctrl / ⌘", "Shift", "S"]]
    ]
  }
] as const;

function close() {
  emit("close");
}

function openGuide() {
  BrowserOpenURL(guideURL.value);
}

function focusables(): HTMLElement[] {
  return Array.from(dialog.value?.querySelectorAll<HTMLElement>(
    "button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])"
  ) ?? []).filter((item) => !item.hasAttribute("hidden"));
}

function onKeydown(event: KeyboardEvent) {
  event.stopPropagation();
  if (event.key === "Escape") {
    event.preventDefault();
    close();
    return;
  }
  if (event.key !== "Tab") return;
  const items = focusables();
  if (!items.length) return;
  const first = items[0];
  const last = items.at(-1)!;
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

onMounted(() => nextTick(() => dialog.value?.querySelector<HTMLElement>("button")?.focus()));
</script>

<template>
  <div class="help-overlay" @pointerdown.self="close">
    <section
      ref="dialog"
      class="help-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="help-dialog-title"
      aria-describedby="help-dialog-description"
      @keydown.capture="onKeydown"
    >
      <header class="help-dialog-header">
        <div>
          <strong id="help-dialog-title">{{ t("help.title") }}</strong>
          <span id="help-dialog-description">{{ t("help.description") }}</span>
        </div>
        <button class="icon-button" :title="t('common.close')" :aria-label="t('common.close')" @click="close">
          <AppIcon name="x" />
        </button>
      </header>
      <div class="help-dialog-content">
        <section class="help-product-info" :aria-label="t('help.productInfo')">
          <span class="brand-mark" aria-hidden="true"><img src="/appicon.svg" alt="" /></span>
          <div>
            <strong>SheetProof</strong>
            <span>{{ t("help.version") }} <b class="version-badge">v{{ packageInfo.version }}</b></span>
          </div>
        </section>
        <section class="help-guide-section" :aria-label="t('help.guideTitle')">
          <div>
            <strong>{{ t("help.guideTitle") }}</strong>
            <span>{{ t("help.guideDescription") }}</span>
          </div>
          <button class="primary help-guide-button" :title="t('help.openGuide')" :aria-label="t('help.openGuide')" @click="openGuide">
            <AppIcon name="external-link" />{{ t("help.openGuide") }}
          </button>
        </section>
        <section class="help-shortcuts" :aria-label="t('help.shortcuts')">
          <div v-for="group in shortcutGroups" :key="group.title" class="help-shortcut-group">
            <h2>{{ t(group.title) }}</h2>
            <ul>
              <li v-for="item in group.items" :key="item[0]">
                <span>{{ t(item[0]) }}</span>
                <span class="shortcut-keys"><kbd v-for="key in item[1]" :key="key">{{ key }}</kbd></span>
              </li>
            </ul>
          </div>
        </section>
      </div>
      <footer class="help-dialog-actions">
        <button :title="t('common.close')" :aria-label="t('common.close')" @click="close">{{ t("common.close") }}</button>
      </footer>
    </section>
  </div>
</template>
