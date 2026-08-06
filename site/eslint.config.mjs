import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
  {
    rules: {
      // ScreenshotViewer needs a directly transformable image element for its keyboard,
      // wheel, and touch gesture behavior; the brand icon is a static local SVG.
      "@next/next/no-img-element": "off",
    },
  },
]);

export default eslintConfig;
