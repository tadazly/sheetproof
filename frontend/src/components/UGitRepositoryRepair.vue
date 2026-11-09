<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";
import { backend } from "../backend";
import { useI18n, type MessageKey } from "../i18n";
import { localizeBackendError } from "../i18n/errors";
import type { UGitRepositoryRepair, UGitRepositoryScanResult } from "../types";
import AppIcon from "./AppIcon.vue";

defineProps<{ disabled: boolean }>();
const emit = defineEmits<{ busy: [active: boolean] }>();
const { t } = useI18n();
const working = ref<"scan" | "repair" | null>(null);
const scan = ref<UGitRepositoryScanResult | null>(null);
const selected = ref<string[]>([]);
const results = ref<UGitRepositoryRepair[]>([]);
const completed = ref(false);
const cancelling = ref(false);
const notice = ref("");
const error = ref("");
let generation = 0;
const outcomes = computed(() => new Map(results.value.map(item => [item.path, item])));
const allSelected = computed(() => !!scan.value?.scan.repositories.length && selected.value.length === scan.value.scan.repositories.length);
const statusKeys: Record<UGitRepositoryRepair["status"], MessageKey> = {
  repaired: "ugitScan.repaired", unchanged: "ugitScan.unchanged", changed: "ugitScan.changed", failed: "ugitScan.failed"
};

function setWorking(value: typeof working.value) {
  if (working.value === value) return;
  working.value = value;
  emit("busy", value !== null);
}

async function startScan() {
  if (working.value) return;
  const request = ++generation;
  scan.value = null;
  selected.value = [];
  results.value = [];
  completed.value = false;
  notice.value = error.value = "";
  cancelling.value = false;
  setWorking("scan");
  try {
    const result = await backend.scanUGitRepositories();
    if (request !== generation) return;
    if (result.cancelled) notice.value = t("ugitScan.cancelled");
    else {
      scan.value = result;
      selected.value = result.scan.repositories.map(item => item.path);
    }
  } catch (reason) {
    if (request === generation) error.value = localizeBackendError(reason);
  } finally {
    if (request === generation) setWorking(null);
  }
}

async function cancelScan() {
  cancelling.value = true;
  try {
    await backend.cancelUGitRepositoryScan();
  } catch (reason) {
    error.value = localizeBackendError(reason);
    cancelling.value = false;
  }
}

function toggleAll() {
  selected.value = allSelected.value ? [] : scan.value?.scan.repositories.map(item => item.path) ?? [];
}

async function repair() {
  if (!scan.value || !selected.value.length || working.value || completed.value) return;
  const request = ++generation;
  notice.value = error.value = "";
  setWorking("repair");
  try {
    const result = await backend.repairUGitRepositories(scan.value.scanID, [...selected.value]);
    if (request !== generation) return;
    if (result.cancelled) notice.value = t("ugitScan.repairCancelled");
    else {
      completed.value = true;
      results.value = result.results;
      notice.value = t("ugitScan.finished", {
        repaired: result.results.filter(item => item.status === "repaired").length,
        unchanged: result.results.filter(item => item.status === "unchanged").length,
        failed: result.results.filter(item => item.status === "failed" || item.status === "changed").length
      });
    }
  } catch (reason) {
    if (request === generation) error.value = localizeBackendError(reason);
  } finally {
    if (request === generation) setWorking(null);
  }
}

onBeforeUnmount(() => {
  generation++;
  if (working.value === "scan") void backend.cancelUGitRepositoryScan().catch(() => undefined);
  setWorking(null);
});
</script>

<template>
  <div class="ugit-scan">
    <p>{{ t("ugitScan.description") }}</p>
    <div class="ugit-scan-actions">
      <button class="secondary" :disabled="disabled || !!working" :aria-label="t('ugitScan.start')" @click="startScan">
        <AppIcon name="folder-open" />{{ t("ugitScan.start") }}
      </button>
      <button v-if="working === 'scan'" :disabled="cancelling" @click="cancelScan">{{ t("ugitScan.cancel") }}</button>
    </div>
    <p v-if="working" role="status">{{ t(working === "scan" ? "ugitScan.scanning" : "ugitScan.repairing") }}</p>
    <template v-if="scan">
      <p class="ugit-scan-directory">{{ scan.scan.directory }}</p>
      <p role="status">{{ t("ugitScan.summary", { total: scan.scan.inspected, count: scan.scan.repositories.length }) }}</p>
      <template v-if="scan.scan.repositories.length">
        <label class="ugit-scan-select-all">
          <input type="checkbox" :checked="allSelected" :disabled="disabled || completed" @change="toggleAll" />
          {{ t("ugitScan.selectAll") }}
        </label>
        <ul class="ugit-scan-list" :aria-label="t('ugitScan.list')">
          <li v-for="item in scan.scan.repositories" :key="item.path">
            <label>
              <input v-model="selected" type="checkbox" :value="item.path" :disabled="disabled || completed" :aria-label="item.path" />
              <span><strong>{{ item.path }}</strong><small>{{ item.tools.length ? item.tools.join(" · ") : t("ugitScan.incomplete") }}</small></span>
            </label>
            <span class="ugit-scan-outcome" :class="outcomes.get(item.path)?.status">
              {{ outcomes.has(item.path) ? t(statusKeys[outcomes.get(item.path)!.status]) : t(completed ? "ugitScan.notSelected" : "ugitScan.pending") }}
            </span>
            <p v-if="outcomes.get(item.path)?.message" class="ugit-scan-error">{{ outcomes.get(item.path)?.message }}</p>
          </li>
        </ul>
        <p>{{ t("ugitScan.scope") }}</p>
        <button v-if="!completed" class="primary" :disabled="disabled || !selected.length" @click="repair">
          {{ t("ugitScan.repair", { count: selected.length }) }}
        </button>
      </template>
      <details v-if="scan.scan.issues.length" class="ugit-scan-issues">
        <summary>{{ t("ugitScan.issues", { count: scan.scan.issues.length }) }}</summary>
        <ul><li v-for="(issue, index) in scan.scan.issues" :key="index"><strong>{{ issue.path }}</strong><span>{{ issue.message }}</span></li></ul>
      </details>
    </template>
    <p v-if="notice" class="ugit-scan-notice" role="status">{{ notice }}</p>
    <p v-if="error" class="ugit-scan-error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.ugit-scan { min-width: 0; display: grid; gap: 10px; padding-top: 12px; border-top: 1px solid var(--color-border); font-size: calc(12px * var(--ui-font-scale)); line-height: 1.5; }
.ugit-scan p { margin: 0; overflow-wrap: anywhere; }
.ugit-scan > p { color: var(--color-text-secondary); }
.ugit-scan-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.ugit-scan-directory { font-weight: 600; }
.ugit-scan-select-all { display: flex; align-items: center; gap: 8px; }
.ugit-scan input { flex-shrink: 0; }
.ugit-scan-list { max-height: 270px; overflow-y: auto; margin: 0; padding: 0; list-style: none; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface); }
.ugit-scan-list li { display: grid; gap: 5px; padding: 10px; border-bottom: 1px solid var(--color-border); }
.ugit-scan-list li:last-child { border-bottom: 0; }
.ugit-scan-list label { display: flex; align-items: flex-start; gap: 8px; }
.ugit-scan-list label > span { min-width: 0; display: grid; gap: 3px; overflow-wrap: anywhere; }
.ugit-scan-list small { color: var(--color-text-secondary); font-size: inherit; }
.ugit-scan-outcome { padding-left: 24px; color: var(--color-text-muted); }
.ugit-scan-outcome.repaired, .ugit-scan-outcome.unchanged { color: #26724d; }
.ugit-scan-outcome.changed, .ugit-scan-outcome.failed, .ugit-scan .ugit-scan-error { color: #a52f36; }
.ugit-scan .ugit-scan-notice { padding: 10px; border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-surface); }
.ugit-scan-issues { overflow-wrap: anywhere; }
.ugit-scan-issues ul { max-height: 180px; overflow: auto; padding-left: 20px; }
.ugit-scan-issues li { margin-block: 8px; }
.ugit-scan-issues strong, .ugit-scan-issues span { display: block; }
.ugit-scan > button { justify-self: end; }
</style>
