import js from "@eslint/js";
import reactDom from "eslint-plugin-react-dom";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import reactX from "eslint-plugin-react-x";
import { defineConfig, globalIgnores } from "eslint/config";
import globals from "globals";
import tseslint from "typescript-eslint";

export default defineConfig([
  globalIgnores(["dist"]),
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat["recommended-latest"],
      reactRefresh.configs.vite,
      tseslint.configs.recommendedTypeChecked,
      tseslint.configs.strictTypeChecked,
      tseslint.configs.stylisticTypeChecked,
      reactX.configs["recommended-typescript"],
      reactDom.configs.recommended,
    ],
    languageOptions: {
      ecmaVersion: "latest",
      globals: globals.browser,
      parserOptions: {
        // Uses the TypeScript project service to pick up tsconfigs
        // automatically instead of hardcoding them.
        projectService: true,
      },
    },
    rules: {
      // shadcn-style ui files intentionally export helpers alongside their
      // components; allow these named exports for fast refresh.
      "react-refresh/only-export-components": [
        "error",
        { allowExportNames: ["buttonVariants", "useSidebar"] },
      ],
    },
  },
  {
    linterOptions: {
      reportUnusedDisableDirectives: "error",
    },
  },
]);
