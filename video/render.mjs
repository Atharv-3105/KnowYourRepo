// Renders index.html deterministically (seek -> screenshot -> WebCodecs encode)
// and muxes H.264 + AAC into an MP4 - no ffmpeg.
//   node render.mjs probe                 what can this Chrome encode?
//   node render.mjs stills 3.2 12 40      PNG stills at those times (seconds)
//   node render.mjs full [fps]            the full video -> out/knowyourrepo-architecture.mp4
//        --seg 300      frames per browser session (default 300)
//        --mbps 7       video bitrate in Mbit/s (default 7)
//        --from/--to    render only a frame range (debugging)
//
// Long single-session renders stall on memory-starved machines, so video is
// encoded in short independent browser sessions. Each returns its raw encoded
// chunks; the MP4 is muxed once, here in Node, from all of them.
import http from "node:http";
import { createReadStream, existsSync, mkdirSync, writeFileSync, statSync } from "node:fs";
import { extname, join, normalize } from "node:path";
import puppeteer from "puppeteer-core";
import { Muxer, ArrayBufferTarget } from "mp4-muxer";

const ROOT = process.cwd();
const MIME = { ".html": "text/html", ".js": "text/javascript", ".mjs": "text/javascript", ".json": "application/json", ".png": "image/png", ".woff2": "font/woff2", ".wav": "audio/wav", ".css": "text/css" };
const server = http.createServer((req, res) => {
  const path = normalize(decodeURIComponent(req.url.split("?")[0])).replace(/^([/\\])+/, "");
  const file = join(ROOT, path || "index.html");
  if (!file.startsWith(ROOT) || !existsSync(file) || statSync(file).isDirectory()) { res.writeHead(404); res.end(); return; }
  res.writeHead(200, { "Content-Type": MIME[extname(file)] ?? "application/octet-stream", "Content-Length": statSync(file).size });
  createReadStream(file).pipe(res);
});
await new Promise((r) => server.listen(0, "127.0.0.1", r));
const BASE = `http://127.0.0.1:${server.address().port}`;

const [mode = "stills", ...rest] = process.argv.slice(2);
const opt = (k, d) => { const i = rest.indexOf("--" + k); return i >= 0 ? Number(rest[i + 1]) : d; };
mkdirSync("out", { recursive: true });

async function openPage() {
  const browser = await puppeteer.launch({
    executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
    headless: "new",
    userDataDir: ".chrome-profile",
    args: ["--hide-scrollbars", "--force-color-profile=srgb", "--no-first-run", "--disable-background-timer-throttling", "--disable-renderer-backgrounding", "--autoplay-policy=no-user-gesture-required"],
    defaultViewport: { width: 1920, height: 1080, deviceScaleFactor: 1 },
    protocolTimeout: 180000,
  });
  const page = await browser.newPage();
  page.on("pageerror", (e) => console.log("[pageerror]", e.message));
  await page.goto(`${BASE}/index.html`);
  await page.evaluate(() => window.__ready);
  return { browser, page };
}
const watchdog = (label, p, ms = 90000) => Promise.race([p, new Promise((_, j) => setTimeout(() => j(new Error(`watchdog: "${label}" exceeded ${ms / 1000}s`)), ms))]);
const b64 = (u8) => Buffer.from(u8).toString("base64");

/** Pull a big in-page array of {data(b64),...} out in slices. */
async function drain(page, name) {
  const n = await page.evaluate((name) => window[name].length, name);
  const out = [];
  for (let i = 0; i < n; i += 40) out.push(...(await page.evaluate((name, i) => window[name].slice(i, i + 40), name, i)));
  return out;
}

try {
  if (mode === "probe") {
    const { browser, page } = await openPage();
    console.log(await page.evaluate(async () => {
      const r = {};
      r.video = (await VideoEncoder.isConfigSupported({ codec: "avc1.640028", width: 1920, height: 1080, bitrate: 14e6, framerate: 30, avc: { format: "avc" } })).supported;
      for (const codec of ["mp4a.40.2", "opus"]) r[codec] = (await AudioEncoder.isConfigSupported({ codec, sampleRate: codec === "opus" ? 48000 : 44100, numberOfChannels: 2, bitrate: 192000 })).supported;
      return JSON.stringify(r);
    }));
    await browser.close();
  }

  if (mode === "stills") {
    const { browser, page } = await openPage();
    for (const t of rest.map(Number)) {
      await page.evaluate((t) => window.__seek(t), t);
      await new Promise((r) => setTimeout(r, 120));
      const f = `out/still-${String(t).replace(".", "_")}.png`;
      await page.screenshot({ path: f });
      console.log("wrote", f);
    }
    await browser.close();
  }

  if (mode === "full") {
    const FPS = Number(rest[0] && !rest[0].startsWith("--") ? rest[0] : 30);
    const SEG = opt("seg", 300), MBPS = opt("mbps", 7);
    let dur;
    {
      const { browser, page } = await openPage();
      dur = await page.evaluate(() => window.__duration);
      await browser.close();
    }
    const total = Math.round(dur * FPS), FROM = opt("from", 0), TO = Math.min(opt("to", total), total);
    const OUT = FROM === 0 && TO === total ? "out/knowyourrepo-architecture.mp4" : `out/partial-${FROM}-${TO}.mp4`;
    console.log(`duration ${dur.toFixed(2)}s, ${total} frames @ ${FPS}fps, segments of ${SEG}, ${MBPS} Mbit/s`);
    const t0 = Date.now();

    // ---- audio: decode the WAV once and encode to AAC ----
    let audio, audioMeta;
    {
      const { browser, page } = await openPage();
      const sr = await page.evaluate(async () => {
        const wav = await (await fetch("/out/music.wav")).arrayBuffer();
        const buf = await new OfflineAudioContext(2, 1, 44100).decodeAudioData(wav);
        const L = buf.getChannelData(0), R = buf.getChannelData(1), sr = buf.sampleRate;
        window.__chunks = []; window.__meta = null;
        const enc = new AudioEncoder({
          output: (c, m) => {
            const d = new Uint8Array(c.byteLength); c.copyTo(d);
            if (!window.__meta && m?.decoderConfig) window.__meta = { codec: m.decoderConfig.codec, sampleRate: m.decoderConfig.sampleRate, numberOfChannels: m.decoderConfig.numberOfChannels, description: m.decoderConfig.description ? btoa(String.fromCharCode(...new Uint8Array(m.decoderConfig.description))) : null };
            let s = ""; for (let i = 0; i < d.length; i += 0x8000) s += String.fromCharCode.apply(null, d.subarray(i, i + 0x8000));
            window.__chunks.push({ type: c.type, timestamp: c.timestamp, duration: c.duration, data: btoa(s) });
          },
          error: (e) => { window.__err = String(e); },
        });
        enc.configure({ codec: "mp4a.40.2", sampleRate: sr, numberOfChannels: 2, bitrate: 192000 });
        const CH = 4096;
        for (let i = 0; i < L.length; i += CH) {
          const n = Math.min(CH, L.length - i), data = new Float32Array(n * 2);
          data.set(L.subarray(i, i + n), 0); data.set(R.subarray(i, i + n), n);
          const ad = new AudioData({ format: "f32-planar", sampleRate: sr, numberOfFrames: n, numberOfChannels: 2, timestamp: Math.round((i / sr) * 1e6), data });
          enc.encode(ad); ad.close();
        }
        await enc.flush();
        return sr;
      });
      audio = await drain(page, "__chunks");
      audioMeta = await page.evaluate(() => window.__meta);
      await browser.close();
      console.log(`audio: ${audio.length} AAC chunks @ ${sr} Hz`);
    }

    // ---- video: short independent sessions ----
    const video = [];
    let videoMeta = null;
    for (let a = FROM; a < TO; a += SEG) {
      const b = Math.min(a + SEG, TO);
      const { browser, page } = await openPage();
      await page.evaluate(async (FPS, MBPS) => {
        window.__chunks = []; window.__meta = null;
        window.__venc = new VideoEncoder({
          output: (c, m) => {
            const d = new Uint8Array(c.byteLength); c.copyTo(d);
            if (!window.__meta && m?.decoderConfig?.description) window.__meta = { codec: m.decoderConfig.codec, coded: [m.decoderConfig.codedWidth, m.decoderConfig.codedHeight], description: btoa(String.fromCharCode(...new Uint8Array(m.decoderConfig.description))) };
            let s = ""; for (let i = 0; i < d.length; i += 0x8000) s += String.fromCharCode.apply(null, d.subarray(i, i + 0x8000));
            window.__chunks.push({ type: c.type, timestamp: c.timestamp, duration: c.duration, data: btoa(s) });
          },
          error: (e) => { window.__err = String(e); },
        });
        window.__venc.configure({ codec: "avc1.640028", width: 1920, height: 1080, bitrate: MBPS * 1e6, framerate: FPS, avc: { format: "avc" }, latencyMode: "quality" });
      }, FPS, MBPS);

      for (let i = a; i < b; i++) {
        await watchdog(`seek ${i}`, page.evaluate((t) => window.__seek(t), i / FPS));
        const jpg = await watchdog(`screenshot ${i}`, page.screenshot({ type: "jpeg", quality: 93 }));
        await watchdog(`encode ${i}`, page.evaluate(async (data, i, key, FPS) => {
          const bytes = Uint8Array.from(atob(data), (c) => c.charCodeAt(0));
          const bmp = await createImageBitmap(new Blob([bytes], { type: "image/jpeg" }));
          const vf = new VideoFrame(bmp, { timestamp: Math.round((i / FPS) * 1e6), duration: Math.round(1e6 / FPS) });
          while (window.__venc.encodeQueueSize > 4) await new Promise((r) => setTimeout(r, 4));
          window.__venc.encode(vf, { keyFrame: key });
          vf.close(); bmp.close();
        }, jpg.toString("base64"), i, i === a || i % (FPS * 2) === 0, FPS));
      }
      await watchdog("flush", page.evaluate(() => window.__venc.flush()), 120000);
      const err = await page.evaluate(() => window.__err ?? null);
      if (err) throw new Error("encoder error: " + err);
      video.push(...(await drain(page, "__chunks")));
      const meta = await page.evaluate(() => window.__meta);
      if (!videoMeta) videoMeta = meta; else if (meta && meta.description !== videoMeta.description) console.log("warning: H.264 parameter sets differ between segments");
      await browser.close();
      const el = (Date.now() - t0) / 1000, done = b - FROM, eta = (el / done) * (TO - b);
      console.log(`frames ${a}-${b}/${TO}  ${(b / FPS).toFixed(1)}s  chunks ${video.length}  elapsed ${el.toFixed(0)}s  eta ${eta.toFixed(0)}s`);
    }

    // ---- mux once ----
    const u8 = (s) => new Uint8Array(Buffer.from(s, "base64"));
    const target = new ArrayBufferTarget();
    const muxer = new Muxer({ target, fastStart: "in-memory", firstTimestampBehavior: "offset", video: { codec: "avc", width: 1920, height: 1080, frameRate: FPS }, audio: { codec: "aac", numberOfChannels: 2, sampleRate: audioMeta.sampleRate } });
    const vmeta = { decoderConfig: { codec: videoMeta.codec, codedWidth: 1920, codedHeight: 1080, description: u8(videoMeta.description) } };
    const ameta = { decoderConfig: { codec: audioMeta.codec, sampleRate: audioMeta.sampleRate, numberOfChannels: audioMeta.numberOfChannels, description: audioMeta.description ? u8(audioMeta.description) : undefined } };
    const rebase = FROM === 0 ? 0 : Math.round((FROM / FPS) * 1e6);
    video.forEach((c, k) => muxer.addVideoChunkRaw(u8(c.data), c.type, c.timestamp - rebase, c.duration, k === 0 ? vmeta : undefined));
    audio.forEach((c, k) => { if (c.timestamp >= rebase) muxer.addAudioChunkRaw(u8(c.data), c.type, c.timestamp - rebase, c.duration, k === 0 || c.timestamp === rebase ? ameta : undefined); });
    muxer.finalize();
    writeFileSync(OUT, Buffer.from(target.buffer));
    console.log(`wrote ${OUT}  ${(target.buffer.byteLength / 1e6).toFixed(1)} MB  in ${((Date.now() - t0) / 1000).toFixed(0)}s`);
  }
} finally {
  server.close();
}
