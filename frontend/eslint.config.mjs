// eslint.config.js
import { fixupPluginRules } from "@eslint/compat";
import js from "@eslint/js";
import prettier from "eslint-config-prettier";
import jsxA11y from "eslint-plugin-jsx-a11y";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default [
  /* =========================
     Global ignores
     ========================= */
  {
    ignores: ["node_modules/**", ".next/**", "dist/**", "coverage/**", "*.config.*"],
  },

  /* =========================
     Base JS rules
     ========================= */
  js.configs.recommended,

  /* =========================
     TypeScript (NON type-aware, fast)
     ========================= */
  ...tseslint.configs.recommended,

  /* =========================
     TypeScript + React (type-aware, scoped)
     ========================= */
  {
    files: ["src/**/*.{ts,tsx}"],
    languageOptions: {
      parserOptions: {
        project: "./tsconfig.eslint.json",
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: {
      // These plugins don't support ESLint 10 yet
      react: fixupPluginRules(react),
      "react-hooks": reactHooks,
      "jsx-a11y": fixupPluginRules(jsxA11y),
    },
    settings: {
      react: { version: "detect" },
    },
    rules: {
      /* React */
      "react/react-in-jsx-scope": "off",
      "react/jsx-uses-react": "off",
      "react/jsx-key": "error",
      "react/no-unknown-property": "error",
      "react/jsx-filename-extension": ["warn", { extensions: [".tsx"] }],

      /* Hooks */
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",

      /* Accessibility (lightweight) */
      "jsx-a11y/alt-text": "warn",
      "jsx-a11y/anchor-is-valid": "warn",

      /* TypeScript */
      "@typescript-eslint/no-unused-vars": ["warn", { argsIgnorePattern: "^_", varsIgnorePattern: "^_" }],
      "@typescript-eslint/no-explicit-any": "warn",
    },
  },

  /* =========================
     Prettier (ALWAYS LAST)
     ========================= */
  prettier,
];
