// Captures real screenshots of the running app (docker compose up) for use
// inside the video. Needs a repo already ingested; pass its id as argv[2].
import puppeteer from "puppeteer-core";
import { mkdirSync } from "node:fs";

const REPO = process.argv[2] ?? "repo_1792235102465022795";
const BASE = "http://localhost:5173";
const OUT = "assets/ui";
mkdirSync(OUT, { recursive: true });

const browser = await puppeteer.launch({
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  headless: "new",
  userDataDir: ".chrome-profile",
  args: ["--hide-scrollbars", "--force-color-profile=srgb", "--no-first-run", "--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist"],
  defaultViewport: { width: 1440, height: 900, deviceScaleFactor: 2 },
});
const page = await browser.newPage();
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const ONLY = process.env.ONLY?.split(",");
const shot = (name) => (!ONLY || ONLY.includes(name)) ? page.screenshot({ path: `${OUT}/${name}.png` }) : null;
const go = async (path) => { await page.goto(`${BASE}/repos/${REPO}/${path}`, { waitUntil: "networkidle0" }); await sleep(900); };

await go("overview");           await shot("overview");
await page.evaluate(() => document.querySelector("main, #root")?.scrollTo?.(0, 0));
// Reading path with the inline code previews.
await page.evaluate(() => { for (const el of document.querySelectorAll("*")) if (el.scrollHeight > el.clientHeight + 50 && getComputedStyle(el).overflowY !== "visible") el.scrollTop = 520; });
await sleep(700);                await shot("overview-reading");

await go("architecture");       await shot("architecture");
await page.evaluate(() => {
  const chip = [...document.querySelectorAll("button")].find((b) => b.textContent.includes("mux.go:54-79"));
  chip?.parentElement?.querySelector("button:last-of-type")?.click?.();
  const open = chip && [...chip.parentElement.querySelectorAll("button")].find((b) => b.textContent.trim() === "open");
  open?.click();
});
await sleep(1800);               await shot("code-panel");
await page.keyboard.press("Escape");

await go("symbols");
await page.type('input[placeholder^="Search symbols"]', "Route");
await sleep(1200);               await shot("symbols");

await go("graph");
await page.type('input[placeholder="symbol name"]', "ServeHTTP");
await page.keyboard.press("Enter");
await sleep(3500);               await shot("graph2d");
// The 2D/3D toggle isn't a <button>; click it by position (the "3D" half).
await page.mouse.click(591, 117);
await sleep(6000);               await shot("graph3d");

if (ONLY && !ONLY.some((n) => n.startsWith("chat"))) { await browser.close(); console.log("captured:", ONLY); process.exit(0); }
await go("chat");
await page.type('input[placeholder^="Ask a question"], textarea', "How does the router match an incoming request to a route?");
await page.keyboard.press("Enter");
await page.waitForFunction(() => document.body.innerText.includes("Sources"), { timeout: 90000 });
await sleep(1500);               await shot("chat-answer");
await page.evaluate(() => { for (const el of document.querySelectorAll("*")) if (el.scrollHeight > el.clientHeight + 50 && getComputedStyle(el).overflowY !== "visible") el.scrollTop = el.scrollHeight; });
await sleep(900);                await shot("chat-sources");

await browser.close();
console.log("captured:", OUT);
