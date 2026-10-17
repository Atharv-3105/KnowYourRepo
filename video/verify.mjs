// Loads the finished MP4 in Chrome and reports what a viewer would get:
// duration, resolution, whether audio decodes, and frames pulled from the file.
//   node verify.mjs [out/knowyourrepo-architecture.mp4] [t1 t2 ...]
import http from "node:http";
import { createReadStream, statSync, writeFileSync } from "node:fs";
import puppeteer from "puppeteer-core";

const file = process.argv[2] ?? "out/knowyourrepo-architecture.mp4";
const times = process.argv.slice(3).map(Number);
const size = statSync(file).size;

const server = http.createServer((req, res) => {
  const range = req.headers.range;
  if (range) {
    const [s, e] = range.replace("bytes=", "").split("-");
    const start = Number(s), end = e ? Number(e) : size - 1;
    res.writeHead(206, { "Content-Type": "video/mp4", "Content-Range": `bytes ${start}-${end}/${size}`, "Accept-Ranges": "bytes", "Content-Length": end - start + 1 });
    createReadStream(file, { start, end }).pipe(res);
  } else {
    res.writeHead(200, { "Content-Type": "video/mp4", "Content-Length": size, "Accept-Ranges": "bytes" });
    createReadStream(file).pipe(res);
  }
});
await new Promise((r) => server.listen(0, "127.0.0.1", r));
const port = server.address().port;

const browser = await puppeteer.launch({
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  headless: "new", userDataDir: ".chrome-profile",
  args: ["--autoplay-policy=no-user-gesture-required", "--no-first-run"],
  defaultViewport: { width: 1920, height: 1080 },
});
const page = await browser.newPage();
await page.goto(`http://127.0.0.1:${port}/`).catch(() => {});
const report = await page.evaluate(async (port) => {
  document.body.innerHTML = `<video id="v" src="http://127.0.0.1:${port}/v.mp4" muted style="width:1920px"></video>`;
  const v = document.getElementById("v");
  await new Promise((r, j) => { v.onloadeddata = r; v.onerror = () => j(new Error("video error " + v.error?.code + " " + v.error?.message)); });
  const out = { duration: v.duration, width: v.videoWidth, height: v.videoHeight };
  // audio: play unmuted briefly and see whether decoded audio bytes accumulate
  v.muted = false; v.volume = 0;
  try { await v.play(); await new Promise((r) => setTimeout(r, 2500)); } catch (e) { out.playError = String(e); }
  out.audioDecodedBytes = v.webkitAudioDecodedByteCount ?? null;
  out.videoDecodedBytes = v.webkitVideoDecodedByteCount ?? null;
  out.playedTo = v.currentTime;
  v.pause();
  return out;
}, port);
console.log(JSON.stringify(report));

for (const t of times) {
  await page.evaluate(async (t) => { const v = document.getElementById("v"); v.currentTime = t; await new Promise((r) => (v.onseeked = r)); }, t);
  const data = await page.evaluate(() => { const v = document.getElementById("v"); const c = document.createElement("canvas"); c.width = 960; c.height = 540; c.getContext("2d").drawImage(v, 0, 0, 960, 540); return c.toDataURL("image/png").split(",")[1]; });
  const f = `out/verify-${String(t).replace(".", "_")}.png`;
  writeFileSync(f, Buffer.from(data, "base64"));
  console.log("frame", t, "->", f);
}
await browser.close();
server.close();
