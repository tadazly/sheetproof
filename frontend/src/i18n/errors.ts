import { t, type MessageKey } from "./index";

type StructuredError = {
  code?: string;
  details?: { path?: string; [key: string]: unknown };
  debugMessage?: string;
};

const codeKeys: Record<string, MessageKey> = {
  file_not_found: "errors.fileNotFound",
  file_unreadable: "errors.corruptWorkbook",
  unsupported_format: "errors.unsupportedFormat",
  corrupt_workbook: "errors.corruptWorkbook",
  no_worksheets: "errors.noWorksheets",
  external_modification: "errors.externalModification",
  file_unwritable: "errors.saveFailed",
  save_failed: "errors.saveFailed",
  readonly_source: "errors.readOnly",
  git_reference_not_found: "errors.gitReference",
  repository_import_failed: "errors.repositoryImport",
  ugit_configuration_failed: "errors.ugitConfiguration"
};

function parseError(reason: unknown): StructuredError {
  const message = reason instanceof Error ? reason.message : String(reason ?? "");
  try {
    const parsed = JSON.parse(message) as StructuredError;
    if (parsed && typeof parsed === "object") return parsed;
  } catch {
    // Existing Go errors use `code (path): debug text`. This compatibility
    // parser keeps old backends localizable while APIs move to structured errors.
  }
  const match = message.match(/^([a-z_]+)(?: \((.*?)\))?:\s*(.*)$/s);
  if (match) return { code: match[1], details: { path: match[2] }, debugMessage: match[3] };
  return { debugMessage: message };
}

export function localizeBackendError(reason: unknown): string {
  const parsed = parseError(reason);
  const key = parsed.code ? codeKeys[parsed.code] : undefined;
  return key
    ? t(key, { path: parsed.details?.path ?? "" })
    : t("errors.unknown", { details: parsed.debugMessage ?? "" });
}
