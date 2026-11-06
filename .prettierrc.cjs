const { execFileSync } = require("node:child_process");
const { existsSync } = require("node:fs");
const { createRequire } = require("node:module");
const { join } = require("node:path");

// mise 分别安装 npm 包；按实际安装目录解析插件，不依赖全局模块或个人路径。
function plugin(name) {
  const prefix = execFileSync("mise", ["where", `npm:${name}`], {
    cwd: __dirname,
    encoding: "utf8",
    timeout: 3000,
    stdio: ["ignore", "pipe", "pipe"],
  }).trim();
  for (const base of [prefix, join(prefix, "lib")]) {
    if (existsSync(join(base, "node_modules", name, "package.json"))) {
      return createRequire(join(base, "package.json")).resolve(name);
    }
  }
  throw new Error(`找不到 mise 已安装的插件 ${name}，请运行 mise run setup`);
}

module.exports = {
  printWidth: 120,
  tabWidth: 2,
  useTabs: false,
  endOfLine: "lf",
  proseWrap: "preserve",
  embeddedLanguageFormatting: "off",
  xmlWhitespaceSensitivity: "strict",
  plugins: [plugin("@prettier/plugin-xml"), plugin("prettier-plugin-toml")],
};
