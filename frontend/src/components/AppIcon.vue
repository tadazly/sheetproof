<script setup lang="ts">
withDefaults(defineProps<{
  name:
    | "alert"
    | "branch"
    | "check"
    | "chevron-down"
    | "chevron-left"
    | "chevron-right"
    | "edit"
    | "file"
    | "files"
    | "folder"
    | "folder-open"
    | "info"
    | "merge"
    | "refresh"
    | "repository"
    | "save"
    | "save-as"
    | "search"
    | "selection"
    | "settings"
    | "undo"
    | "x"
    | "zoom";
  size?: number;
}>(), {
  size: 16
});
</script>

<template>
  <svg
    class="app-icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <template v-if="name === 'repository'">
      <path d="M3 7.5h18v11.25A2.25 2.25 0 0 1 18.75 21H5.25A2.25 2.25 0 0 1 3 18.75Z" />
      <path d="M3 7.5V5.25A2.25 2.25 0 0 1 5.25 3h4l2 2.25H21V7.5" />
    </template>
    <template v-else-if="name === 'folder' || name === 'folder-open'">
      <path v-if="name === 'folder'" d="M3 6.75A1.75 1.75 0 0 1 4.75 5h5l2 2h7.5A1.75 1.75 0 0 1 21 8.75v8.5A1.75 1.75 0 0 1 19.25 19H4.75A1.75 1.75 0 0 1 3 17.25Z" />
      <path v-else d="M3 8V6.75A1.75 1.75 0 0 1 4.75 5h5l2 2h7.5A1.75 1.75 0 0 1 21 8.75V10M4.5 19h13.8a1.8 1.8 0 0 0 1.7-1.2l1.45-4.15A1.25 1.25 0 0 0 20.27 12H6.1a1.8 1.8 0 0 0-1.7 1.2L3 17.25A1.3 1.3 0 0 0 4.5 19Z" />
    </template>
    <template v-else-if="name === 'file'">
      <path d="M6 3h8l4 4v14H6Z" />
      <path d="M14 3v5h5M9 12h6M9 16h6" />
    </template>
    <template v-else-if="name === 'files'">
      <path d="M7 3h8l4 4v13H7Z" />
      <path d="M15 3v5h5M4 7v13a1 1 0 0 0 1 1h10" />
    </template>
    <template v-else-if="name === 'branch'">
      <circle cx="6" cy="5" r="2" />
      <circle cx="18" cy="6" r="2" />
      <circle cx="6" cy="19" r="2" />
      <path d="M6 7v10M8 8.5c1.5 0 2.75.05 4-.75L16 6" />
    </template>
    <template v-else-if="name === 'chevron-left'"><path d="m15 18-6-6 6-6" /></template>
    <template v-else-if="name === 'chevron-right'"><path d="m9 18 6-6-6-6" /></template>
    <template v-else-if="name === 'chevron-down'"><path d="m6 9 6 6 6-6" /></template>
    <template v-else-if="name === 'merge'">
      <circle cx="6" cy="5" r="2" />
      <circle cx="18" cy="19" r="2" />
      <path d="M6 7v3c0 5 3 9 10 9M18 5v7M15 9l3 3 3-3" />
    </template>
    <template v-else-if="name === 'undo'">
      <path d="m9 7-5 5 5 5M5 12h8.5a5.5 5.5 0 0 1 5.5 5.5V19" />
    </template>
    <template v-else-if="name === 'zoom'">
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="m15.5 15.5 5 5M8 10.5h5M10.5 8v5" />
    </template>
    <template v-else-if="name === 'save'">
      <path d="M4 3h13l3 3v15H4Z" />
      <path d="M8 3v6h8V3M8 21v-7h8v7" />
    </template>
    <template v-else-if="name === 'save-as'">
      <path d="M4 3h11l3 3v6M8 3v6h7V3M8 20H4V3" />
      <path d="m14 19 6-6M16 21l-3 .75.75-3L19 13l2 2Z" />
    </template>
    <template v-else-if="name === 'refresh'">
      <path d="M20 7v5h-5M4 17v-5h5" />
      <path d="M6.1 8.5A7 7 0 0 1 18.5 6L20 8M4 16l1.5 2A7 7 0 0 0 18 15.5" />
    </template>
    <template v-else-if="name === 'search'">
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="m15.5 15.5 5 5" />
    </template>
    <template v-else-if="name === 'selection'">
      <path d="M5 8V5h3M16 5h3v3M19 16v3h-3M8 19H5v-3" />
      <path d="M9 9h6v6H9Z" />
    </template>
    <template v-else-if="name === 'settings'">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21h-4v-.09A1.7 1.7 0 0 0 8.6 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H3v-4h.09A1.7 1.7 0 0 0 4.6 8.6a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V3h4v.09A1.7 1.7 0 0 0 15.4 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.2.38.54.7 1 .9.34.15.72.2 1.1.1H21v4h-.09A1.7 1.7 0 0 0 19.4 15Z" />
    </template>
    <template v-else-if="name === 'edit'">
      <path d="M4 20h4l11-11-4-4L4 16Z" />
      <path d="m13.5 6.5 4 4" />
    </template>
    <template v-else-if="name === 'check'"><path d="m5 12 4 4L19 6" /></template>
    <template v-else-if="name === 'x'"><path d="m6 6 12 12M18 6 6 18" /></template>
    <template v-else-if="name === 'alert'">
      <path d="M12 3 2.8 20h18.4Z" />
      <path d="M12 9v5M12 17.5h.01" />
    </template>
    <template v-else-if="name === 'info'">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v6M12 7h.01" />
    </template>
  </svg>
</template>
