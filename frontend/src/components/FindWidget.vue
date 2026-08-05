<script setup lang="ts">
import { nextTick, ref } from "vue";
import AppIcon from "./AppIcon.vue";
import { useI18n } from "../i18n";

const props = defineProps<{
  side: "left" | "right";
  query: string;
  caseSensitive: boolean;
  wholeWord: boolean;
  regex: boolean;
  count: number;
  currentIndex: number;
  searching: boolean;
  error: string;
  invalidRegex: boolean;
}>();

const emit = defineEmits<{
  (event: "update:query", value: string): void;
  (event: "toggle", option: "caseSensitive" | "wholeWord" | "regex"): void;
  (event: "next"): void;
  (event: "previous"): void;
  (event: "close"): void;
  (event: "focus"): void;
}>();

const input = ref<HTMLInputElement | null>(null);
const { t } = useI18n();

function focus() {
  nextTick(() => {
    input.value?.focus();
    input.value?.select();
  });
}

function updateQuery(event: Event) {
  emit("update:query", (event.target as HTMLInputElement).value);
}

function handleEnter(event: KeyboardEvent) {
  if (event.shiftKey) emit("previous");
  else emit("next");
}

defineExpose({ focus });
</script>

<template>
  <div class="find-widget" :data-side="props.side" role="search" @pointerdown.stop @click.stop>
    <div class="find-input-wrap">
      <AppIcon name="search" :size="13" />
      <input
        ref="input"
        class="find-input"
        :value="props.query"
        :aria-label="t(`find.${props.side}Input`)"
        :title="t(`find.${props.side}Input`)"
        autocomplete="off"
        spellcheck="false"
        @input="updateQuery"
        @focus="emit('focus')"
        @keydown.enter.prevent.stop="handleEnter"
        @keydown.esc.prevent.stop="emit('close')"
      />
    </div>
    <button
      class="find-option"
      :class="{ active: props.caseSensitive }"
      :aria-label="t('find.caseSensitive')"
      :title="t('find.caseSensitive')"
      :aria-pressed="props.caseSensitive"
      @click="emit('toggle', 'caseSensitive')"
    >Aa</button>
    <button
      class="find-option"
      :class="{ active: props.wholeWord }"
      :aria-label="t('find.wholeWord')"
      :title="t('find.wholeWord')"
      :aria-pressed="props.wholeWord"
      @click="emit('toggle', 'wholeWord')"
    >ab</button>
    <button
      class="find-option"
      :class="{ active: props.regex }"
      :aria-label="t('find.regex')"
      :title="t('find.regex')"
      :aria-pressed="props.regex"
      @click="emit('toggle', 'regex')"
    >.*</button>
    <span class="find-status" :class="{ error: props.error }" aria-live="polite">
      <template v-if="props.searching">{{ t("find.searching") }}</template>
      <template v-else-if="props.error">{{ t(props.invalidRegex ? "find.invalidRegex" : "find.error") }}</template>
      <template v-else-if="props.query && props.count === 0">{{ t("find.noResults") }}</template>
      <template v-else>{{ props.currentIndex }} / {{ props.count }}</template>
    </span>
    <button class="find-icon-button" :aria-label="t('find.previous')" :title="t('find.previous')" @click="emit('previous')">
      <AppIcon name="chevron-left" :size="14" />
    </button>
    <button class="find-icon-button" :aria-label="t('find.next')" :title="t('find.next')" @click="emit('next')">
      <AppIcon name="chevron-right" :size="14" />
    </button>
    <button class="find-icon-button" :aria-label="t('find.close')" :title="t('find.close')" @click="emit('close')">
      <AppIcon name="x" :size="14" />
    </button>
    <div v-if="props.error" class="find-error-detail" :title="props.error">{{ props.error }}</div>
  </div>
</template>
