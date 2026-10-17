/* Beat-locked timeline for the architecture deep-dive video.
 * Every cut lands on the same bar grid the soundtrack is built on (audio.py):
 * 128 BPM, 36 bars. window.__seek(t) renders the frame at time t (seconds). */
const BPM = 128, B = 60 / BPM, BAR = B * 4;
const T = (bar, beat = 0) => bar * BAR + beat * B;
const DUR = 36 * BAR + 3.0; // 70.5s, matches out/music.wav

const $ = (s, r = document) => r.querySelector(s);
function h(tag, cls, html, parent, style) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (html != null) e.innerHTML = html;
  if (style) e.style.cssText = style;
  if (parent) parent.appendChild(e);
  return e;
}
const C = { blue: "#6f9bff", orange: "#ff8a4c", green: "#7fd1a0", violet: "#b28dff", dim: "#8d96a9", line: "#252d3d" };

const tl = gsap.timeline({ paused: true });

/* ---------- helpers ---------- */
function scene(id, a, b, whip = 0.28) {
  const el = document.getElementById(id);
  tl.set(el, { opacity: 1, scale: 1, filter: "none" }, a);
  tl.to(el, { opacity: 0, scale: 1.035, filter: "blur(14px)", duration: whip, ease: "power2.in" }, b - whip);
  tl.set(el, { opacity: 0 }, b);
  return el;
}
const reveal = (el, at, dur = 0.7) => tl.fromTo(el, { yPercent: 118 }, { yPercent: 0, duration: dur, ease: "expo.out" }, at);
const rise = (el, at, o = {}) =>
  tl.fromTo(el, { y: o.y ?? 50, opacity: 0, scale: o.scale ?? 0.94 }, { y: 0, opacity: 1, scale: 1, duration: o.dur ?? 0.5, ease: o.ease ?? "expo.out", stagger: o.stagger ?? 0 }, at);
const pop = (el, at, o = {}) =>
  tl.fromTo(el, { scale: 0.55, opacity: 0, y: 24 }, { scale: 1, opacity: 1, y: 0, duration: o.dur ?? 0.45, ease: "back.out(2.1)" }, at);
const flash = (at, a = 0.7, d = 0.35) => {
  tl.fromTo("#flash", { opacity: a }, { opacity: 0, duration: d, ease: "power2.out", immediateRender: false }, at);
};
function typeText(el, text, at, dur, caret = true) {
  const p = { n: 0 };
  tl.fromTo(p, { n: 0 }, { n: text.length, duration: dur, ease: "none", onUpdate() {
    const k = Math.floor(p.n);
    el.textContent = text.slice(0, k) + (caret && k < text.length ? "▌" : "");
  } }, at);
}
function count(el, to, at, dur, fmt = (v) => Math.round(v).toLocaleString("en-US"), from = 0) {
  const p = { v: from };
  el.textContent = fmt(from);
  tl.fromTo(p, { v: from }, { v: to, duration: dur, ease: "power3.out", onUpdate() { el.textContent = fmt(p.v); } }, at);
}
function wrapChars(el, text) {
  el.innerHTML = "";
  const inners = [];
  for (const ch of text) {
    const m = h("span", "mask", "", el, "padding:0 0 .16em;margin-bottom:-.16em");
    const s = h("span", "", ch === " " ? "&nbsp;" : ch, m);
    inners.push(s);
  }
  return inners;
}
function node(parent, o) {
  const d = h("div", "card", "", parent, `left:${o.x - o.w / 2}px;top:${o.y - o.h / 2}px;width:${o.w}px;height:${o.h}px;padding:22px 26px`);
  if (o.tag) h("div", "tag", o.tag, d, `color:${o.color || C.blue};margin-bottom:10px`);
  h("div", "t", o.title, d);
  if (o.sub) h("div", "s", o.sub, d);
  return d;
}
function edgePath(svg, a, b, color = C.blue, dashed = false) {
  const mx = (a.x + b.x) / 2;
  const vertical = Math.abs(a.x - b.x) < Math.abs(a.y - b.y);
  const d = vertical ? `M${a.x} ${a.y} C${a.x} ${(a.y + b.y) / 2}, ${b.x} ${(a.y + b.y) / 2}, ${b.x} ${b.y}` : `M${a.x} ${a.y} C${mx} ${a.y}, ${mx} ${b.y}, ${b.x} ${b.y}`;
  const p = document.createElementNS("http://www.w3.org/2000/svg", "path");
  p.setAttribute("d", d);
  p.setAttribute("fill", "none");
  p.setAttribute("stroke", color);
  p.setAttribute("stroke-width", "2.5");
  p.setAttribute("stroke-linecap", "round");
  svg.appendChild(p);
  const len = p.getTotalLength();
  if (dashed) { p.setAttribute("stroke-dasharray", "8 10"); p.style.opacity = 0; return { p, len, dashed }; }
  p.style.strokeDasharray = len;
  p.style.strokeDashoffset = len;
  return { p, len, dashed };
}
function drawEdge(e, at, dur = 0.45) {
  if (e.dashed) tl.fromTo(e.p, { opacity: 0 }, { opacity: 0.8, duration: dur }, at);
  else tl.to(e.p, { strokeDashoffset: 0, duration: dur, ease: "power2.out" }, at);
}
function packet(svg, e, color, start, end, period) {
  const c = document.createElementNS("http://www.w3.org/2000/svg", "circle");
  c.setAttribute("r", "7"); c.setAttribute("fill", color);
  c.style.filter = `drop-shadow(0 0 8px ${color})`; c.style.opacity = 0;
  svg.appendChild(c);
  const p = { k: 0 };
  tl.set(c, { opacity: 1 }, start);
  tl.fromTo(p, { k: 0 }, { k: (end - start) / period, duration: end - start, ease: "none", onUpdate() {
    const f = p.k % 1; const pt = e.p.getPointAtLength(f * e.len); c.setAttribute("cx", pt.x); c.setAttribute("cy", pt.y);
  } }, start);
  tl.set(c, { opacity: 0 }, end);
}
function sceneTitle(id, at) {
  const w = $(`#${id} .title-wrap`);
  tl.fromTo(w.children, { y: 40, opacity: 0 }, { y: 0, opacity: 1, duration: 0.55, ease: "expo.out", stagger: 0.07 }, at);
}
function chapter(label, at, endAt) {
  tl.call(() => { $("#hudchap").textContent = label; }, null, at);
}

/* ================= BACKDROP + HUD ================= */
tl.to("#blob1", { x: 420, y: 260, duration: DUR, ease: "sine.inOut" }, 0);
tl.to("#blob2", { x: -380, y: -300, duration: DUR, ease: "sine.inOut" }, 0);
tl.to("#grid", { x: -80, y: -80, duration: DUR, ease: "none" }, 0);
tl.to("#hud", { opacity: 1, duration: 0.5 }, T(2, 3));
tl.to("#hud", { opacity: 0, duration: 0.3 }, T(34) - 0.35);
tl.fromTo("#progress", { width: 0 }, { width: 1920, duration: T(34) - T(2, 3), ease: "none" }, T(2, 3));

/* ================= S0 COLD OPEN (bars 0-2) ================= */
scene("s0", 0, T(2));
{
  const code = `type Router struct {\n\tNotFoundHandler http.Handler\n\troutes []*Route\n\tnamedRoutes map[string]*Route\n}\n\nfunc (r *Router) Match(req *http.Request, match *RouteMatch) bool {\n\tfor _, route := range r.routes {\n\t\tif route.Match(req, match) {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n\nfunc NewRouter() *Router {\n\treturn &Router{namedRoutes: make(map[string]*Route)}\n}\n`;
  $("#rain").textContent = (code + "\n").repeat(5);
  tl.fromTo("#rain", { y: 40 }, { y: -520, duration: T(2), ease: "none" }, 0);
  const inner = (id) => $(`#${id} .mask > span`);
  reveal(inner("s0a"), T(0, 0.2), 0.8);
  reveal(inner("s0b"), T(0, 2.2), 0.8);
  tl.fromTo(inner("s0c"), { yPercent: 118, scale: 1.5 }, { yPercent: 0, scale: 1, duration: 0.55, ease: "expo.out" }, T(1, 0));
  flash(T(1, 0), 0.25, 0.3);
}

/* ================= S1 TITLE (bars 2-6) ================= */
scene("s1", T(2), T(6));
{
  const a = T(2);
  flash(a, 0.9, 0.5);
  tl.fromTo("#logo", { scale: 0.2, rotation: -25, opacity: 0 }, { scale: 1, rotation: 0, opacity: 1, duration: 0.8, ease: "back.out(2)" }, a);
  const chars = wrapChars($("#s1name"), "KnowYourRepo");
  tl.fromTo(chars, { yPercent: 125 }, { yPercent: 0, duration: 0.6, ease: "expo.out", stagger: 0.04 }, a + 0.18);
  tl.fromTo("#s1glow", { opacity: 0, scale: 0.7 }, { opacity: 1, scale: 1, duration: 1.2, ease: "power2.out" }, a + 0.2);
  typeText($("#s1tag"), "Know your repo before you touch it.", T(3), T(0, 3));
  const chips = [["Go", "Gin", C.blue], ["Python", "FastAPI", C.green], ["React", "TypeScript", C.violet], ["Postgres", "pgvector", C.orange]];
  const wrap = $("#s1chips");
  chips.forEach(([a1, b1, col], i) => {
    const c = h("span", "pill", `<span style="color:${col}">●</span>&nbsp; ${a1} <span class="dim">/ ${b1}</span>`, wrap);
    pop(c, T(4, 0) + i * B * 0.5);
  });
  tl.fromTo("#s1sub", { opacity: 0, letterSpacing: "0.9em" }, { opacity: 1, letterSpacing: "0.22em", duration: 0.9, ease: "expo.out" }, T(5, 0));
  tl.to("#logo", { y: -10, duration: 1.6, ease: "sine.inOut", yoyo: true, repeat: 3 }, T(3));
}

/* ================= S2 SYSTEM (bars 6-10) ================= */
scene("s2", T(6), T(10));
chapter("THE SYSTEM", T(6));
{
  const svg = $("#s2svg"), host = $("#s2nodes");
  sceneTitle("s2", T(6, 0.1));
  const spec = {
    spa: { x: 290, y: 600, w: 340, h: 184, tag: "Browser", title: "React SPA", sub: "TanStack Query\nCytoscape + three.js", color: C.violet, at: T(6, 0.5) },
    api: { x: 790, y: 600, w: 340, h: 184, tag: "api/", title: "Go orchestrator", sub: "Gin · go-git\nTree-sitter", color: C.blue, at: T(6, 2) },
    car: { x: 1290, y: 600, w: 340, h: 184, tag: "extraction-service/", title: "Python sidecar", sub: "FastAPI · embeddings\nLLM router", color: C.green, at: T(7, 0) },
    db: { x: 1040, y: 905, w: 460, h: 150, tag: "Postgres 16", title: "pgvector", sub: "structure + 1024-d vectors", color: C.orange, at: T(7, 2) },
    gh: { x: 790, y: 330, w: 330, h: 150, tag: "External", title: "GitHub", sub: "clone · size check", color: C.dim, at: T(8, 0) },
    or: { x: 1700, y: 430, w: 310, h: 150, tag: "External · free", title: "OpenRouter", sub: "embeddings", color: C.dim, at: T(8, 1) },
    llm: { x: 1700, y: 665, w: 310, h: 184, tag: "External · free", title: "LLM providers", sub: "Groq · Gemini\nCerebras · OpenRouter", color: C.dim, at: T(8, 2) },
    obs: { x: 290, y: 905, w: 340, h: 150, tag: "Observability", title: "Prometheus", sub: "+ Grafana dashboards", color: C.dim, at: T(8, 3) },
  };
  const els = {};
  for (const [k, o] of Object.entries(spec)) {
    els[k] = node(host, { ...o, sub: o.sub.replace(/\n/g, "<br/>") });
    pop(els[k], o.at);
  }
  const E = (a, b, col, label, lx, ly, at, dashed) => {
    const e = edgePath(svg, a, b, col, dashed);
    drawEdge(e, at);
    const l = h("div", "elabel", label, host, `left:${lx}px;top:${ly}px`);
    tl.fromTo(l, { opacity: 0, scale: 0.8 }, { opacity: 1, scale: 1, duration: 0.3 }, at + 0.25);
    return e;
  };
  const e1 = E({ x: 460, y: 600 }, { x: 620, y: 600 }, C.violet, "REST / JSON", 540, 560, T(6, 3));
  const e2 = E({ x: 960, y: 600 }, { x: 1120, y: 600 }, C.blue, "shared secret", 1040, 560, T(7, 1));
  const e3 = E({ x: 790, y: 508 }, { x: 790, y: 405 }, C.dim, "clone", 790, 456, T(8, 0.8));
  const e4 = E({ x: 800, y: 692 }, { x: 900, y: 830 }, C.blue, "structure", 790, 775, T(7, 3));
  const e5 = E({ x: 1280, y: 692 }, { x: 1180, y: 830 }, C.green, "vectors", 1290, 775, T(7, 3.4));
  const e6 = E({ x: 1460, y: 560 }, { x: 1545, y: 440 }, C.green, "embed", 1500, 500, T(8, 1.8));
  const e7 = E({ x: 1460, y: 640 }, { x: 1545, y: 665 }, C.green, "chat", 1500, 622, T(8, 2.8));
  const e8 = E({ x: 460, y: 890 }, { x: 740, y: 692 }, C.dim, "/metrics", 560, 790, T(9, 0.2), true);
  const pk = T(9, 0.4), pend = T(10) - 0.5;
  [[e1, C.violet], [e2, C.blue], [e3, C.dim], [e4, C.blue], [e5, C.green], [e6, C.green], [e7, C.green]].forEach(([e, col], i) => packet(svg, e, col, pk + i * 0.12, pend, 0.9));
  // emphasize the two-language split
  tl.to([els.api, els.car], { boxShadow: "0 0 0 1px rgba(111,155,255,.6), 0 0 60px rgba(111,155,255,.3)", duration: 0.4 }, T(9, 2));
}

/* ================= S3 INGESTION (bars 10-16) ================= */
scene("s3", T(10), T(16));
chapter("01 / INGESTION", T(10));
{
  sceneTitle("s3", T(10, 0.1));
  const stages = [
    ["01", "Clone", "go-git, no git binary"], ["02", "Walk", "skip minified, huge, vendored"], ["03", "Hash", "SHA-256 diff, skip unchanged"],
    ["04", "Parse", "Tree-sitter ASTs"], ["05", "Extract", "symbols + call edges"], ["06", "Chunk", "function-level, overlapping"], ["07", "Embed", "128 chunks per request"],
  ];
  const host = $("#s3stages"), W = 215, G = 25, X0 = 132;
  const cards = stages.map(([n, t, s], i) => {
    const d = h("div", "card", "", host, `left:${X0 + i * (W + G)}px;top:430px;width:${W}px;height:200px;padding:20px 20px`);
    h("div", "tag", n, d, `color:${C.orange}`);
    h("div", "t", t, d, "margin-top:14px;font-size:38px");
    h("div", "s", s, d, "font-size:16px");
    const at = T(10, 1) + i * B * 0.6;
    pop(d, at);
    // lights up as the beam passes
    tl.to(d, { borderColor: "rgba(111,155,255,.9)", boxShadow: "0 0 0 1px rgba(111,155,255,.6), 0 0 50px rgba(111,155,255,.35)", duration: 0.25 }, T(10, 1) + (i / 6) * (T(13, 0) - T(10, 1)));
    return d;
  });
  tl.fromTo("#s3beam", { width: 0 }, { width: 7 * W + 6 * G, duration: T(13, 0) - T(10, 1), ease: "none" }, T(10, 1));
  const P = $("#s3panels");
  const mk = (x, w, tag, at) => { const d = h("div", "card", "", P, `left:${x}px;top:720px;width:${w}px;height:260px;padding:24px 28px`); h("div", "tag", tag, d, `color:${C.blue}`); rise(d, at); return d; };
  const pa = mk(132, 520, "real run · gorilla/mux", T(12, 0));
  const nums = [["files", 17, C.blue], ["symbols", 261, C.green], ["call edges", 2239, C.orange]];
  nums.forEach(([lab, val, col], i) => {
    const row = h("div", "", "", pa, `position:absolute;left:28px;top:${62 + i * 62}px;display:flex;align-items:baseline;gap:18px`);
    const n = h("span", "h-md", "0", row, `color:${col};min-width:150px;font-size:54px`);
    h("span", "mono dim", lab, row, "font-size:22px");
    count(n, val, T(12, 0.6) + i * 0.15, 1.3);
  });
  const pb = mk(690, 520, "tree-sitter → symbol", T(12, 2));
  h("div", "mono", `<span class="accent">type</span> Router <span class="accent">struct</span> {<br/>&nbsp;&nbsp;NotFoundHandler http.Handler<br/>&nbsp;&nbsp;routes []*Route<br/>}`, pb, "position:absolute;left:28px;top:62px;font-size:20px;line-height:1.55;color:#c9d3ea");
  const chip = h("div", "pill", `Router · <span style="color:${C.green}">struct</span> · mux.go:54-79`, pb, "position:absolute;left:28px;bottom:26px;border-color:rgba(127,209,160,.6)");
  tl.fromTo(chip, { opacity: 0, x: -20 }, { opacity: 1, x: 0, duration: 0.4, ease: "expo.out" }, T(13, 0.3));
  const pc = mk(1248, 540, "only what changed is re-processed", T(13, 0));
  const grid = h("div", "", "", pc, "position:absolute;left:28px;top:66px;display:grid;grid-template-columns:repeat(8,1fr);gap:10px;width:484px");
  const sq = [];
  for (let i = 0; i < 16; i++) sq.push(h("div", "", "", grid, `height:44px;border-radius:8px;background:#1b2232;border:1px solid ${C.line}`));
  const changed = [3, 10];
  sq.forEach((s, i) => {
    tl.fromTo(s, { scale: 0, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.25, ease: "back.out(2)" }, T(13, 0.2) + i * 0.025);
    if (changed.includes(i)) tl.to(s, { background: "rgba(111,155,255,.25)", borderColor: C.blue, boxShadow: "0 0 24px rgba(111,155,255,.6)", duration: 0.25 }, T(14, 0.5));
    else tl.to(s, { opacity: 0.28, duration: 0.25 }, T(14, 0.3));
  });
  h("div", "s", `<span style="color:${C.blue}">2 changed</span> re-parsed + re-embedded<br/>14 unchanged skipped by hash`, pc, "position:absolute;left:28px;bottom:22px;font-size:18px");
}

/* ================= S4 RETRIEVAL (bars 16-22) ================= */
scene("s4", T(16), T(22));
chapter("02 / RETRIEVAL", T(16));
{
  sceneTitle("s4", T(16, 0.1));
  const svg = $("#s4svg"), host = $("#s4nodes");
  const q = node(host, { x: 330, y: 575, w: 400, h: 170, tag: "question", title: "&nbsp;", color: C.orange });
  const qt = $(".t", q); qt.style.cssText = "font-family:var(--sans);font-weight:500;font-size:26px;line-height:1.3";
  pop(q, T(16, 1));
  typeText(qt, "How does the router match an incoming request?", T(16, 1.6), T(0, 3.5));
  const lanes = [
    { y: 390, tag: "semantic", title: "Vector search", sub: "embed query → cosine HNSW top-k", col: C.blue },
    { y: 575, tag: "lexical", title: "Identifier search", sub: "exact names ranked first", col: C.orange },
    { y: 760, tag: "call graph", title: "Neighbour expansion", sub: "callers + callees of every hit", col: C.green },
  ];
  const mg = node(host, { x: 1200, y: 575, w: 280, h: 200, tag: "merge", title: "De-dupe", sub: "by symbol + file<br/>then rank", color: C.violet });
  const rt = node(host, { x: 1600, y: 575, w: 380, h: 390, tag: "sidecar", title: "LLM router", sub: "", color: C.green });
  const provs = ["Groq", "Gemini", "Cerebras", "OpenRouter"];
  const prow = provs.map((p, i) => h("div", "pill", p, rt, `position:absolute;left:26px;top:${104 + i * 54}px;width:328px;font-size:19px`));
  h("div", "s", "cooldowns · rate-limit windows", rt, "position:absolute;left:26px;bottom:20px");
  pop(mg, T(17, 3)); pop(rt, T(18, 1));
  lanes.forEach((l, i) => {
    const n = node(host, { x: 760, y: l.y, w: 430, h: 150, tag: l.tag, title: l.title, sub: l.sub, color: l.col });
    pop(n, T(16, 3) + i * B * 0.7);
    const e1 = edgePath(svg, { x: 530, y: 575 }, { x: 545, y: l.y }, l.col); drawEdge(e1, T(16, 3) + i * B * 0.7 + 0.1);
    const e2 = edgePath(svg, { x: 975, y: l.y }, { x: 1060, y: 575 }, l.col); drawEdge(e2, T(17, 2) + i * 0.1);
    if (i === 0) { packet(svg, e1, l.col, T(17, 1), T(19, 2), 0.8); }
    if (i === 1) { packet(svg, e1, l.col, T(17, 1.3), T(19, 2), 0.8); }
    if (i === 2) { packet(svg, e1, l.col, T(17, 1.6), T(19, 2), 0.8); }
    packet(svg, e2, l.col, T(18, 0.2) + i * 0.1, T(19, 2), 0.8);
  });
  const e3 = edgePath(svg, { x: 1340, y: 575 }, { x: 1410, y: 575 }, C.violet); drawEdge(e3, T(18, 0.6));
  provs.forEach((_, i) => {
    for (let k = 0; k < 2; k++) {
      const at = T(18, 2) + (i + k * 4) * B * 0.5;
      if (at < T(19, 2)) {
        tl.to(prow[i], { background: "rgba(127,209,160,.22)", borderColor: C.green, duration: 0.12 }, at);
        tl.to(prow[i], { background: "rgba(21,26,37,.8)", borderColor: C.line, duration: 0.2 }, at + B * 0.5);
      }
    }
  });
  // phase B: compress the pipeline, bring in the real answer
  const pB = T(19, 2);
  tl.to("#s4flow", { scale: 0.46, x: 0, y: -10, transformOrigin: "100% 0%", duration: 0.7, ease: "expo.inOut" }, pB);
  tl.fromTo("#s4frame", { y: 700, opacity: 0, rotationX: 12 }, { y: 0, opacity: 1, rotationX: 0, duration: 0.8, ease: "expo.out" }, pB + 0.15);
  tl.to("#s4img", { y: -90, duration: T(1, 0), ease: "sine.inOut" }, pB + 0.8);
  tl.to("#s4img", { opacity: 0, duration: 0.3 }, T(21, 0));
  tl.to("#s4img2", { opacity: 1, duration: 0.3 }, T(21, 0));
  const co = $("#s4callouts");
  const c1 = h("div", "pill", `<span style="color:${C.blue}">● semantic</span> &nbsp;<span style="color:${C.orange}">● lexical</span>`, co, "position:absolute;left:1580px;top:490px;font-size:20px");
  const c2 = h("div", "pill", `<span style="color:${C.green}">●</span> cited sources → open exact lines`, co, "position:absolute;left:1580px;top:780px;font-size:20px;width:300px;white-space:normal;border-radius:18px");
  tl.fromTo(c1, { x: 40, opacity: 0 }, { x: 0, opacity: 1, duration: 0.5, ease: "expo.out" }, T(20, 2));
  tl.fromTo(c2, { x: 40, opacity: 0 }, { x: 0, opacity: 1, duration: 0.5, ease: "expo.out" }, T(21, 1));
}

/* ================= S5 FREE TIERS (bars 22-26) ================= */
scene("s5", T(22), T(26));
chapter("03 / FREE TIERS", T(22));
{
  sceneTitle("s5", T(22, 0.1));
  const host = $("#s5cards");
  const defs = [
    { n: 128, fmt: (v) => Math.round(v), unit: "chunks per embedding request", sub: "free models are limited per request, so batches are as big as allowed", col: C.blue },
    { n: 6, fmt: (v) => Math.round(v) + " / 50", unit: "daily requests for a 750-chunk repo", sub: "OpenRouter free quota: 50 requests/day, 20 per minute", col: C.orange },
    { n: 20, fmt: (v) => Math.round(v), unit: "requests / minute limiter", sub: "client-side limiter plus a daily budget that fails fast", col: C.green },
    { n: 0, fmt: (v) => "$" + Math.round(v), unit: "to run it", sub: "OpenRouter · Groq · Gemini · Neon · Render free tiers", col: C.violet },
  ];
  const W = 400, G = 36, X0 = 106;
  defs.forEach((d, i) => {
    const card = h("div", "card", "", host, `left:${X0 + i * (W + G)}px;top:330px;width:${W}px;height:480px;padding:28px 30px`);
    rise(card, T(22, 1) + i * B);
    const big = h("div", "h-lg", "", card, `color:${d.col};margin-top:6px;font-size:104px;white-space:nowrap`);
    count(big, d.n, T(22, 1.3) + i * B, 1.1, d.fmt);
    h("div", "tag", d.unit, card, "margin-top:12px;font-size:16px;color:var(--ink);line-height:1.4");
    h("div", "s", d.sub, card, "font-size:16px;margin-top:14px");
    const vis = h("div", "", "", card, "position:absolute;left:30px;right:30px;bottom:30px;height:120px");
    if (i === 0) { // 128 squares
      vis.style.cssText += ";display:grid;grid-template-columns:repeat(32,1fr);gap:3px;align-content:end";
      for (let k = 0; k < 128; k++) { const s = h("div", "", "", vis, `height:18px;border-radius:3px;background:${d.col}`); tl.fromTo(s, { opacity: 0 }, { opacity: 0.85, duration: 0.05 }, T(22, 1.5) + i * B + k * 0.008); }
    }
    if (i === 1) { // 50 segments, 6 used
      vis.style.cssText += ";display:grid;grid-template-columns:repeat(25,1fr);gap:4px;align-content:end";
      for (let k = 0; k < 50; k++) { const s = h("div", "", "", vis, `height:30px;border-radius:3px;background:#1b2232;border:1px solid ${C.line}`); if (k < 6) tl.to(s, { background: d.col, borderColor: d.col, boxShadow: `0 0 14px ${d.col}`, duration: 0.2 }, T(23, 0) + k * 0.1); }
    }
    if (i === 2) { // pulse bars
      vis.style.cssText += ";display:flex;gap:6px;align-items:flex-end";
      for (let k = 0; k < 20; k++) { const s = h("div", "", "", vis, `flex:1;border-radius:4px;background:${d.col};opacity:.85;height:20px`); tl.to(s, { height: 30 + ((k * 37) % 80), duration: 0.35, ease: "back.out(2)" }, T(23, 0.5) + k * 0.04); tl.to(s, { height: 24 + ((k * 53) % 40), duration: 0.5, yoyo: true, repeat: 2, ease: "sine.inOut" }, T(23, 1.5) + k * 0.03); }
    }
    if (i === 3) { // free-tier stack chips
      vis.style.cssText += ";display:flex;flex-wrap:wrap;gap:8px;align-content:flex-end";
      ["no card", "no trial", "no catch"].forEach((t, k) => { const s = h("span", "pill", t, vis, "font-size:16px;padding:6px 14px"); pop(s, T(23, 1) + k * 0.15); });
    }
  });
  const rs = $("#s5resume");
  const row = h("div", "", "", rs, "display:flex;align-items:center;gap:8px");
  h("span", "tag", "quota ran out mid-repo?", row, `color:${C.orange};margin-right:20px;white-space:nowrap`);
  const segs = [];
  for (let k = 0; k < 12; k++) segs.push(h("div", "", "", row, `flex:1;height:34px;border-radius:6px;background:#1b2232;border:1px solid ${C.line}`));
  const lbl = h("span", "mono", "", row, `margin-left:20px;font-size:19px;color:${C.green};white-space:nowrap`);
  segs.forEach((s, k) => {
    tl.fromTo(s, { opacity: 0, scaleY: 0 }, { opacity: 1, scaleY: 1, duration: 0.25, ease: "back.out(2)" }, T(24, 0) + k * 0.04);
    if (k < 7) tl.to(s, { background: "rgba(127,209,160,.7)", borderColor: C.green, duration: 0.15 }, T(24, 1) + k * 0.08);
  });
  tl.call(() => { lbl.textContent = "7 embedded · 5 resume on next sync"; }, null, T(24, 3));
  tl.fromTo(lbl, { opacity: 0 }, { opacity: 1, duration: 0.3 }, T(24, 3));
  segs.slice(7).forEach((s, k) => tl.to(s, { borderColor: C.orange, background: "rgba(255,138,76,.18)", duration: 0.2 }, T(25, 0) + k * 0.07));
}

/* ================= S6 HARDENING (bars 26-30) ================= */
scene("s6", T(26), T(30));
chapter("04 / HARDENING", T(26));
{
  sceneTitle("s6", T(26, 0.1));
  const host = $("#s6cards");
  const items = [
    ["X-Internal-Key", "Shared-secret sidecar", "The Python service rejects any call without the API's secret.", C.blue],
    ["rate limits", "Per-IP token buckets", "Client IP is hop-counted, never the spoofable leftmost header.", C.orange],
    ["SIGTERM", "Graceful shutdown", "In-flight jobs get a deadline, then resume after restart.", C.green],
    ["GET /files", "Traversal-proof file API", "Absolute and escaping paths rejected, size capped.", C.violet],
    ["allow-list", "Strict repo URLs", "Only github.com/owner/repo reaches the clone path.", C.blue],
    ["X-Request-ID", "Traceable end to end", "One ID from browser to sidecar logs to Prometheus.", C.orange],
  ];
  const W = 560, H = 250, G = 30, X0 = 90;
  const cards = items.map(([tag, title, desc, col], i) => {
    const x = X0 + (i % 3) * (W + G), y = 330 + Math.floor(i / 3) * (H + G);
    const d = h("div", "card", "", host, `left:${x}px;top:${y}px;width:${W}px;height:${H}px;padding:28px 32px;border-left:4px solid ${col}`);
    h("div", "tag", tag, d, `color:${col}`);
    h("div", "t", title, d, "margin-top:12px");
    h("div", "s", desc, d, "font-size:19px;margin-top:12px");
    pop(d, T(26, 0.2) + i * B * 0.85);
    return d;
  });
  // strobe through the cards, accelerating under the snare roll
  const steps = [];
  for (let i = 0; i < 8; i++) steps.push(T(28, i * 0.5));
  for (let i = 0; i < 12; i++) steps.push(T(29, i * 0.25 + (i > 7 ? (i - 8) * -0.125 : 0)));
  steps.forEach((at, i) => {
    const c = cards[i % 6];
    tl.to(c, { borderColor: "rgba(255,255,255,.9)", boxShadow: "0 0 0 1px rgba(255,255,255,.7), 0 0 60px rgba(111,155,255,.5)", duration: 0.06 }, at);
    tl.to(c, { borderColor: "#252d3d", boxShadow: "0 30px 80px rgba(0,0,0,.45)", duration: 0.2 }, at + 0.1);
  });
  tl.to(host, { scale: 1.06, duration: T(2, 0), ease: "power2.in" }, T(28, 0));
}

/* ================= S7 MONTAGE (bars 30-34) ================= */
scene("s7", T(30), T(34), 0.12);
chapter("", T(30));
let graph3d;
{
  const host = $("#s7shots");
  const shots = [
    ["overview", "Overview", "Summary, stats, reading path"],
    ["architecture", "Architecture", "Components detected from the code"],
    ["code-panel", "Source in context", "Exact lines, highlighted"],
    ["symbols", "Symbol index", "Functions, structs, interfaces"],
    ["graph2d", "Call graph", "Anchored on any symbol"],
    [null, "3D call graph", "22 nodes, 29 real edges"],
    ["chat-answer", "Chat", "Answers grounded in the code"],
    ["chat-sources", "Receipts", "Every source one click from the lines"],
  ];
  const FW = 1360, FH = 850;
  const cut = 2 * B;
  shots.forEach(([img, title, sub], i) => {
    const a = T(30) + i * cut;
    const fr = h("div", "frame", "", host, `left:${(1920 - FW) / 2}px;top:70px;width:${FW}px;opacity:0`);
    h("div", "bar", `<i></i><i></i><i></i><b>localhost:5173 / ${title.toLowerCase().replace(/ /g, "-")}</b>`, fr);
    const shot = h("div", "shot", "", fr, `height:${FH}px;background:#0b0e15`);
    if (img) { const im = h("img", "", "", shot); im.src = `assets/ui/${img}.png`; }
    else {
      const cv = h("canvas", "", "", shot, "display:block"); cv.width = FW; cv.height = FH; graph3d = cv;
    }
    const dir = i % 2 ? 1 : -1;
    tl.set(fr, { opacity: 1 }, a);
    tl.fromTo(fr, { x: dir * 260, rotationY: -dir * 16, scale: 0.84, filter: "blur(6px)" }, { x: 0, rotationY: 0, scale: 1, filter: "blur(0px)", duration: 0.34, ease: "expo.out" }, a);
    tl.to(fr, { scale: 1.045, duration: cut - 0.34, ease: "none" }, a + 0.34);
    tl.set(fr, { opacity: 0 }, a + cut);
    const chip = h("div", "", `<div class="tag" style="color:${C.orange}">${String(i + 1).padStart(2, "0")} / 08</div><div class="h-md" style="margin-top:6px">${title}</div><div class="mono dim" style="font-size:20px;margin-top:8px">${sub}</div>`, host, `position:absolute;left:150px;bottom:62px;padding:22px 30px;background:rgba(7,9,14,.86);border:1px solid ${C.line};border-radius:16px;opacity:0;backdrop-filter:blur(6px)`);
    tl.fromTo(chip, { opacity: 0, x: -40 }, { opacity: 1, x: 0, duration: 0.3, ease: "expo.out" }, a + 0.12);
    tl.set(chip, { opacity: 0 }, a + cut);
    flash(a, 0.16, 0.18);
  });
  tl.call(() => {}, null, T(34));
}
function buildGraph(data) {
  const names = data.nodes.map((n) => n.symbol), idx = new Map(names.map((n, i) => [n, i]));
  const edges = data.edges.map((e) => [idx.get(e.caller), idx.get(e.callee)]).filter((e) => e[0] != null && e[1] != null);
  const root = idx.get(data.root_symbol);
  let s = 90210; const rnd = () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);
  const P = names.map((_, i) => (i === root ? [0, 0, 0] : [(rnd() - 0.5) * 2, (rnd() - 0.5) * 2, (rnd() - 0.5) * 2]));
  for (let it = 0; it < 300; it++) {
    const F = P.map(() => [0, 0, 0]);
    for (let i = 0; i < P.length; i++) for (let j = i + 1; j < P.length; j++) {
      const d = [P[i][0] - P[j][0], P[i][1] - P[j][1], P[i][2] - P[j][2]], r2 = d[0] ** 2 + d[1] ** 2 + d[2] ** 2 + 0.01, f = 0.05 / r2;
      for (let k = 0; k < 3; k++) { F[i][k] += d[k] * f; F[j][k] -= d[k] * f; }
    }
    for (const [a, b] of edges) {
      const d = [P[b][0] - P[a][0], P[b][1] - P[a][1], P[b][2] - P[a][2]], L = Math.hypot(...d) + 1e-6, f = (L - 0.8) * 0.06;
      for (let k = 0; k < 3; k++) { F[a][k] += (d[k] / L) * f; F[b][k] -= (d[k] / L) * f; }
    }
    for (let i = 0; i < P.length; i++) for (let k = 0; k < 3; k++) { F[i][k] -= P[i][k] * 0.02; P[i][k] += F[i][k] * 4; }
    P[root] = [0, 0, 0];
  }
  const m = Math.max(...P.map((p) => Math.hypot(...p)));
  P.forEach((p) => p.forEach((_, k) => (p[k] /= m)));
  // BFS depth from the root
  const depth = names.map(() => 9); depth[root] = 0;
  for (let pass = 0; pass < 6; pass++) for (const [a, b] of edges) if (depth[a] + 1 < depth[b]) depth[b] = depth[a] + 1;
  return { names, P, edges, depth, root };
}
function drawGraph(cv, G, angle, t) {
  const ctx = cv.getContext("2d"), W = cv.width, H = cv.height;
  const bg = ctx.createRadialGradient(W / 2, H / 2, 40, W / 2, H / 2, W * 0.7);
  bg.addColorStop(0, "#141a28"); bg.addColorStop(1, "#080b12");
  ctx.fillStyle = bg; ctx.fillRect(0, 0, W, H);
  const ca = Math.cos(angle), sa = Math.sin(angle), tilt = 0.42, ct = Math.cos(tilt), st = Math.sin(tilt);
  const R = 410, f = 3.2;
  const pts = G.P.map((p, i) => {
    const x = p[0] * ca + p[2] * sa, z0 = -p[0] * sa + p[2] * ca;
    const y = p[1] * ct - z0 * st, z = p[1] * st + z0 * ct;
    const k = f / (f + z);
    return { x: W / 2 + x * R * k * 1.35, y: H / 2 + y * R * k * 1.05, z, k, i };
  });
  const col = (d) => (d === 0 ? C.orange : d === 1 ? C.blue : C.violet);
  ctx.lineWidth = 1.5;
  for (const [a, b] of G.edges) {
    const A = pts[a], B2 = pts[b], al = Math.max(0.12, 0.55 - (A.z + B2.z) * 0.18);
    ctx.strokeStyle = `rgba(150,170,220,${al})`;
    ctx.beginPath(); ctx.moveTo(A.x, A.y); ctx.lineTo(B2.x, B2.y); ctx.stroke();
    const u = ((t * 0.5 + (a * 7 + b * 3) * 0.13) % 1);
    ctx.fillStyle = "rgba(255,255,255,.85)";
    ctx.beginPath(); ctx.arc(A.x + (B2.x - A.x) * u, A.y + (B2.y - A.y) * u, 3, 0, 7); ctx.fill();
  }
  [...pts].sort((a, b) => b.z - a.z).forEach((p) => {
    const d = G.depth[p.i], r = (d === 0 ? 16 : 9) * p.k * 1.2, c = col(d);
    ctx.shadowColor = c; ctx.shadowBlur = 24 * p.k; ctx.fillStyle = c;
    ctx.globalAlpha = Math.max(0.45, Math.min(1, 1.15 - p.z * 0.35));
    ctx.beginPath(); ctx.arc(p.x, p.y, r, 0, 7); ctx.fill();
    ctx.shadowBlur = 0;
    if (d <= 1 || p.z < -0.2) {
      ctx.fillStyle = "#dfe6f7"; ctx.font = `${d === 0 ? 700 : 500} ${d === 0 ? 24 : 15}px "JetBrains Mono"`;
      ctx.fillText(G.names[p.i], p.x + r + 7, p.y + 5);
    }
    ctx.globalAlpha = 1;
  });
}

/* ================= S8 OUTRO (bars 34-36 + tail) ================= */
scene("s8", T(34), DUR + 1, 0.01);
{
  const a = T(34);
  flash(a, 1, 0.6);
  tl.fromTo("#logo2", { scale: 0.2, rotation: 20, opacity: 0 }, { scale: 1, rotation: 0, opacity: 1, duration: 0.8, ease: "back.out(2)" }, a);
  const chars = wrapChars($("#s8name"), "KnowYourRepo");
  tl.fromTo(chars, { yPercent: 125 }, { yPercent: 0, duration: 0.6, ease: "expo.out", stagger: 0.035 }, a + 0.2);
  tl.fromTo("#s8line", { opacity: 0, y: 30 }, { opacity: 1, y: 0, duration: 0.8, ease: "expo.out" }, a + 1.0);
  tl.fromTo("#s8url", { opacity: 0 }, { opacity: 1, duration: 0.5 }, a + 1.7);
  tl.to("#s8ul", { width: "100%", duration: 0.7, ease: "power3.out" }, a + 2.0);
  ["Go", "Python", "React", "pgvector", "Tree-sitter", "OpenRouter", "Docker"].forEach((t, i) => {
    const c = h("span", "pill", t, $("#s8chips"), "font-size:20px"); pop(c, a + 2.2 + i * 0.09);
  });
  tl.to("#logo2", { y: -8, duration: 1.5, ease: "sine.inOut", yoyo: true, repeat: 3 }, a + 1);
}
tl.fromTo("#fade", { opacity: 0 }, { opacity: 1, duration: 1.4, ease: "power2.in" }, DUR - 1.4);
tl.to({}, { duration: DUR }, 0); // pin the timeline length

/* ---------- 3D graph driver + public API ---------- */
window.__duration = DUR;
window.__ready = (async () => {
  await document.fonts.ready;
  await Promise.all(["Fraunces", "JetBrains Mono", "Public Sans"].map((f) => document.fonts.load(`600 20px "${f}"`)));
  const data = await (await fetch("assets/data/graph.json")).json();
  const G = buildGraph(data);
  const drv = { a: 0 };
  const t0 = T(30) + 5 * 2 * B;
  tl.fromTo(drv, { a: 0 }, { a: 1, duration: 2 * B, ease: "none", onUpdate() { drawGraph(graph3d, G, 0.5 + drv.a * 1.5, drv.a * 2); } }, t0);
  drawGraph(graph3d, G, 0.5, 0);
  await Promise.all([...document.images].map((i) => (i.complete ? 1 : new Promise((r) => (i.onload = i.onerror = r)))));
  tl.time(0);
  return true;
})();
window.__seek = (t) => { tl.time(Math.min(t, DUR), false); };
