import uiConfig from "./ui/eslint.config.js";

const bridgedConfig = Array.isArray(uiConfig) ? [...uiConfig] : [uiConfig];

if (!bridgedConfig.every((entry) => entry && typeof entry === "object")) {
  throw new TypeError(
    "Invalid UI ESLint config bridge: expected an object or an array of objects from ./ui/eslint.config.js",
  );
}

export default bridgedConfig;
