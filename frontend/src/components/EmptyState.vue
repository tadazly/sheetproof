<script setup lang="ts">
import AppIcon from "./AppIcon.vue";

withDefaults(defineProps<{
  icon?: "alert" | "branch" | "check" | "file" | "files" | "info" | "repository" | "search";
  title: string;
  description?: string;
  compact?: boolean;
}>(), {
  icon: "info",
  description: "",
  compact: false
});
</script>

<template>
  <div class="empty-state" :class="{ compact }">
    <span class="empty-state-icon"><AppIcon :name="icon" :size="compact ? 17 : 22" /></span>
    <strong>{{ title }}</strong>
    <span v-if="description">{{ description }}</span>
    <slot />
  </div>
</template>
