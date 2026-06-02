import uiConfig from "./ui/eslint.config.js";

const bridgedConfig = Array.isArray(uiConfig) ? [...uiConfig] : [uiConfig];
const nodeScriptConfig = {
  files: ["scripts/**/*.mjs", "ui/scripts/**/*.mjs"],
  languageOptions: {
    ecmaVersion: 2022,
    sourceType: "module",
    globals: {
      console: "readonly",
      process: "readonly",
    },
  },
  rules: {
    "no-undef": "error",
    "no-unreachable": "error",
    "no-unsafe-finally": "error",
    "no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
  },
};

if (!bridgedConfig.every((entry) => entry && typeof entry === "object")) {
  throw new TypeError(
    "Invalid UI ESLint config bridge: expected an object or an array of objects from ./ui/eslint.config.js",
  );
}

export default [...bridgedConfig, nodeScriptConfig];
