import uiConfig from "./ui/eslint.config.js";

export default Array.isArray(uiConfig) ? [...uiConfig] : [uiConfig];
