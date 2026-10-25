// eslint.config.mjs
import js from "@eslint/js";
import prettier from "eslint-config-prettier";
import tseslint from "typescript-eslint";

export default [
  /* =========================
     Global ignores
     ========================= */
  {
    ignores: ["node_modules/**", "dist/**", ".astro/**"],
  },

  /* =========================
     Base JS rules
     ========================= */
  js.configs.recommended,

  /* =========================
     TypeScript (NON type-aware, fast)
     ========================= */
  ...tseslint.configs.recommended,

  {
    rules: {
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
