#!/usr/bin/env node
// Builds one short clip per related model in public/assets/model-cards/.
//
// Related-model cards previously shared four hero images per modality, so a row
// of four cards read as four copies of the same picture. Each model now gets its
// own clip with its id rendered into the frame, which is what makes the row
// scannable. The public site cannot call the upstream providers to produce real
// samples (Rule 9 -- these pages are preview surfaces), so the clips are
// rendered brand motion, not model output.
//
// Usage:
//   node scripts/build-related-model-videos.mjs                    # default set
//   node scripts/build-related-model-videos.mjs modelA modelB ...  # explicit
//
// Outputs <slug>.mp4 + <slug>.png per model. Re-run when the related set changes.

import { execFile } from "node:child_process";
import { mkdtemp, rm, writeFile, mkdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const run = promisify(execFile);

const HERE = dirname(fileURLToPath(import.meta.url));
const OUT_DIR = resolve(HERE, "..", "public", "assets", "model-cards");

const WIDTH = 640;
const HEIGHT = 360;
const FPS = 24;
const DURATION_S = 4;
const TOTAL_FRAMES = FPS * DURATION_S;

const CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

// Models currently surfaced as "related" for the video model pages. Keep in sync
// with the pricing catalog; an unknown model falls back to the neutral palette.
const DEFAULT_MODELS = [
  "seedance-2.5",
  "MiniMax-H3",
  "grok-imagine-video",
  "grok-imagine-video-1.5",
  "veo-3.1-generate-preview",
  "veo-3.1-fast-generate-preview",
  "sonilo-video-to-music",
];

// Vendor-ish accent so sibling models stay visually distinct in one row.
const PALETTES = [
  { match: /seedance|doubao|bytedance/i, from: "#2563eb", to: "#7c3aed" },
  { match: /minimax/i, from: "#f23f5d", to: "#7c3aed" },
  { match: /grok|xai/i, from: "#111827", to: "#4b5563" },
  { match: /veo|gemini|imagen/i, from: "#4285f4", to: "#34a853" },
  { match: /sonilo|music|audio/i, from: "#0ea5e9", to: "#14b8a6" },
  { match: /sora|gpt|openai/i, from: "#10a37f", to: "#0f766e" },
  { match: /claude|anthropic/i, from: "#d97757", to: "#b45309" },
];

function paletteFor(model) {
  return PALETTES.find((entry) => entry.match.test(model)) ?? { from: "#4c1d95", to: "#2563eb" };
}

export function modelCardSlug(model) {
  return model.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

const ease = (x) => 1 - Math.pow(1 - x, 3);

function framePage(model, frameIndex) {
  const t = frameIndex / (TOTAL_FRAMES - 1);
  const e = ease(Math.min(1, t / 0.35));
  const palette = paletteFor(model);
  // A slow drift plus a sweep keeps the loop alive without implying the clip is
  // real model output.
  const drift = (1 - e) * 18;
  const sweep = -30 + t * 160;
  const bars = Array.from({ length: 9 }, (_, i) => {
    const phase = (t * 2 + i * 0.16) % 1;
    return 12 + Math.round(Math.abs(Math.sin(phase * Math.PI)) * 30);
  });

  return `<!doctype html><meta charset="utf-8"><style>
  *{margin:0;padding:0;box-sizing:border-box}
  html,body{width:${WIDTH}px;height:${HEIGHT}px;overflow:hidden}
  body{
    display:grid;place-items:center;position:relative;
    font-family:-apple-system,"SF Pro Display","Helvetica Neue",Arial,sans-serif;
    background:linear-gradient(135deg,${palette.from} 0%,${palette.to} 100%);
  }
  .grid{
    position:absolute;inset:0;opacity:.22;
    background:
      linear-gradient(to right,rgba(255,255,255,.5) 1px,transparent 1px) 0 0/40px 40px,
      linear-gradient(to bottom,rgba(255,255,255,.4) 1px,transparent 1px) 0 0/40px 40px;
  }
  .sweep{
    position:absolute;top:-40%;left:${sweep.toFixed(1)}%;width:26%;height:180%;
    background:linear-gradient(90deg,transparent,rgba(255,255,255,.22),transparent);
    transform:rotate(12deg);
  }
  .card{
    position:relative;display:grid;justify-items:center;gap:14px;
    padding:26px 40px;border-radius:20px;
    background:rgba(255,255,255,.12);border:1px solid rgba(255,255,255,.26);
    backdrop-filter:blur(6px);
    transform:translateY(${drift.toFixed(2)}px);
  }
  .name{
    font-family:ui-monospace,"SF Mono",Menlo,monospace;
    font-size:28px;font-weight:700;color:#fff;letter-spacing:-.01em;
    text-shadow:0 2px 14px rgba(0,0,0,.35);
  }
  .brand{
    font-size:11px;font-weight:800;letter-spacing:.22em;text-transform:uppercase;
    color:rgba(255,255,255,.8);
  }
  .bars{display:flex;align-items:flex-end;gap:6px;height:44px}
  .bars u{width:6px;border-radius:3px;background:rgba(255,255,255,.85)}
  </style>
  <div class="grid"></div><div class="sweep"></div>
  <div class="card">
    <div class="brand">flatkey</div>
    <div class="name">${model}</div>
    <div class="bars">${bars.map((h) => `<u style="height:${h}px"></u>`).join("")}</div>
  </div>`;
}

async function buildOne(model, work) {
  const slug = modelCardSlug(model);
  const frameDir = join(work, slug);
  await mkdir(frameDir, { recursive: true });

  for (let i = 0; i < TOTAL_FRAMES; i++) {
    const html = join(frameDir, `f${i}.html`);
    await writeFile(html, framePage(model, i), "utf-8");
    await run(CHROME, [
      "--headless",
      "--disable-gpu",
      "--hide-scrollbars",
      `--screenshot=${join(frameDir, `f${String(i).padStart(4, "0")}.png`)}`,
      `--window-size=${WIDTH},${HEIGHT}`,
      `file://${html}`,
    ]);
  }

  const mp4 = join(OUT_DIR, `${slug}.mp4`);
  // Video only: these play on hover, where an audio track would be hostile.
  await run("ffmpeg", [
    "-y", "-v", "error",
    "-framerate", String(FPS),
    "-i", join(frameDir, "f%04d.png"),
    "-an",
    "-c:v", "libx264", "-preset", "slow", "-crf", "30",
    "-pix_fmt", "yuv420p", "-movflags", "+faststart",
    mp4,
  ]);
  await run("ffmpeg", [
    "-y", "-v", "error",
    "-i", mp4, "-ss", "1.5", "-vframes", "1",
    join(OUT_DIR, `${slug}.png`),
  ]);
  return slug;
}

async function main() {
  const models = process.argv.slice(2).length > 0 ? process.argv.slice(2) : DEFAULT_MODELS;
  const work = await mkdtemp(join(tmpdir(), "fk-cards-"));
  try {
    await mkdir(OUT_DIR, { recursive: true });
    for (const model of models) {
      const slug = await buildOne(model, work);
      console.log(`wrote ${slug}.mp4`);
    }
  } finally {
    await rm(work, { recursive: true, force: true });
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
