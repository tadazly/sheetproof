import { describe, expect, it } from "vitest";
import vectors from "../../../product/locale-test-vectors.json";
import { detectSystemLocale, resolveLocale } from "./detectLocale";
import { normalizeLocale } from "./locale";

describe("locale normalization", () => {
  it.each(vectors)("normalizes $input to $expected", ({ input, expected }) => {
    expect(normalizeLocale(input)).toBe(expected);
  });

  it("uses a manual preference before the environment", () => {
    expect(resolveLocale("ja", { languages: ["zh-CN"], language: "zh-CN" })).toBe("ja");
  });

  it("re-reads the environment for system preference", () => {
    expect(resolveLocale("system", { languages: ["ja-JP"], language: "en-US" })).toBe("ja");
    expect(detectSystemLocale({ languages: ["zh-TW", "zh-CN"], language: "en-US" })).toBe("zh-CN");
  });
});
