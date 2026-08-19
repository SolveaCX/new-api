#!/usr/bin/env node
// Builds the Seedance model-page example clip in public/assets/cli/.
//
// The public model page must not run a real generation (see Rule 9 — it is a
// preview surface, generation happens in the console after sign-in), so the
// "example output" is a rendered brand film rather than upstream footage.
// Frames are drawn in headless Chrome and muxed with ffmpeg; both are the only
// external dependencies.
//
// Usage:
//   node scripts/build-model-example-video.mjs
//
// Outputs public/assets/cli/flatkey-seedance-brand-film.{mp4,png}.

import { execFile } from "node:child_process";
import { mkdtemp, rm, writeFile, mkdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const run = promisify(execFile);

const HERE = dirname(fileURLToPath(import.meta.url));
const OUT_DIR = resolve(HERE, "..", "public", "assets", "cli");
const NAME = "flatkey-seedance-brand-film";

const WIDTH = 1280;
const HEIGHT = 720;
const FPS = 30;
const DURATION_S = 6;
const TOTAL_FRAMES = FPS * DURATION_S;

const CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

// Four beats matching the model page's example prompt: logo, catalog, pricing,
// generated preview. `t` is 0..1 within the whole clip.
const SCENES = [
  { at: 0.0, id: "logo" },
  { at: 0.22, id: "catalog" },
  { at: 0.5, id: "pricing" },
  { at: 0.76, id: "preview" },
];

function sceneAt(t) {
  let current = SCENES[0];
  for (const scene of SCENES) if (t >= scene.at) current = scene;
  const index = SCENES.indexOf(current);
  const next = SCENES[index + 1];
  const end = next ? next.at : 1;
  const local = (t - current.at) / Math.max(0.0001, end - current.at);
  return { id: current.id, local: Math.min(1, Math.max(0, local)) };
}

// easeOutCubic — motion settles rather than stopping dead, which is what reads
// as "camera move" at this frame count.
const ease = (x) => 1 - Math.pow(1 - x, 3);

function framePage(frameIndex) {
  const t = frameIndex / (TOTAL_FRAMES - 1);
  const { id, local } = sceneAt(t);
  const e = ease(local);
  // Each scene fades in over its first 18% and out over its last 12%, so cuts
  // read as dissolves instead of hard jumps.
  const opacity = Math.min(1, local / 0.18) * Math.min(1, (1 - local) / 0.12 + 0.4);
  const lift = (1 - e) * 26;
  const scale = 0.985 + e * 0.015;

  const body = {
    logo: `
      <div class="mark">fk</div>
      <div class="wordmark">flatkey</div>
      <div class="tagline">One API for every video model</div>`,
    catalog: `
      <div class="eyebrow">Model catalog</div>
      <div class="rows">
        <div class="row is-lead"><span class="dot"></span><b>seedance-2.5</b><i>ByteDance</i></div>
        <div class="row"><span class="dot dot-muted"></span><b>gpt-5</b><i>OpenAI</i></div>
        <div class="row"><span class="dot dot-muted"></span><b>claude-opus-4</b><i>Anthropic</i></div>
      </div>`,
    pricing: `
      <div class="eyebrow">Live pricing</div>
      <div class="price-card">
        <div class="price-label">Seedance 2.5 video</div>
        <div class="price-main">from $0.0756<span> / second</span></div>
        <div class="price-compare"><s>$0.084</s><em>10% below list</em></div>
      </div>`,
    preview: `
      <div class="eyebrow">Generated with Seedance 2.5</div>
      <div class="screen">
        <div class="screen-bar"><span></span><span></span><span></span></div>
        <div class="screen-body"><div class="play"></div></div>
        <div class="waves">${Array.from({ length: 11 }, (_, i) => `<u style="height:${8 + ((i * 7) % 22)}px"></u>`).join("")}</div>
      </div>`,
  }[id];

  return `<!doctype html><meta charset="utf-8"><style>
  *{margin:0;padding:0;box-sizing:border-box}
  html,body{width:${WIDTH}px;height:${HEIGHT}px;overflow:hidden}
  body{
    display:grid;place-items:center;
    font-family:-apple-system,"SF Pro Display","Helvetica Neue",Arial,sans-serif;
    color:#0b0b0f;
    background:
      linear-gradient(to right,rgba(37,99,235,.05) 1px,transparent 1px) 0 0/64px 64px,
      linear-gradient(to bottom,rgba(37,99,235,.04) 1px,transparent 1px) 0 0/64px 64px,
      linear-gradient(140deg,#ffffff 0%,#f7f9fc 55%,#eef2ff 100%);
  }
  .stage{
    display:grid;justify-items:center;gap:18px;text-align:center;
    opacity:${opacity.toFixed(3)};
    transform:translateY(${lift.toFixed(2)}px) scale(${scale.toFixed(4)});
  }
  .mark{
    width:104px;height:104px;border-radius:26px;display:grid;place-items:center;
    background:linear-gradient(140deg,#7c3aed,#2563eb);
    color:#fff;font-size:42px;font-weight:800;letter-spacing:-.02em;
    box-shadow:0 24px 60px -28px rgba(76,29,149,.75);
  }
  .wordmark{font-size:76px;font-weight:800;letter-spacing:-.035em}
  .tagline{font-size:22px;font-weight:600;color:#5f6673}
  .eyebrow{
    font-size:13px;font-weight:800;letter-spacing:.16em;text-transform:uppercase;
    color:#2563eb;
  }
  .rows{display:grid;gap:12px;min-width:560px}
  .row{
    display:flex;align-items:center;gap:14px;padding:18px 24px;border-radius:16px;
    background:#fff;border:1px solid #e7e4ec;font-size:22px;
    box-shadow:0 18px 46px -40px rgba(24,14,38,.4);
  }
  .row.is-lead{border-color:rgba(37,99,235,.35);box-shadow:0 22px 50px -32px rgba(37,99,235,.5)}
  .row b{font-weight:700;font-family:ui-monospace,"SF Mono",Menlo,monospace}
  .row i{margin-left:auto;font-style:normal;font-size:17px;font-weight:600;color:#6a7280}
  .dot{width:10px;height:10px;border-radius:50%;background:#2563eb}
  .dot-muted{background:#cbd5e1}
  .price-card{
    padding:34px 46px;border-radius:22px;background:#fff;border:1px solid #e7e4ec;
    display:grid;gap:10px;box-shadow:0 26px 64px -44px rgba(24,14,38,.5);
  }
  .price-label{font-size:16px;font-weight:700;color:#6a7280}
  .price-main{font-size:52px;font-weight:800;letter-spacing:-.02em;color:#047857}
  .price-main span{font-size:24px;font-weight:600;color:#6a7280}
  .price-compare{display:flex;align-items:center;justify-content:center;gap:14px;font-size:18px}
  .price-compare s{color:#98a2b3}
  .price-compare em{
    font-style:normal;font-weight:700;color:#047857;
    background:rgba(16,185,129,.12);padding:5px 12px;border-radius:999px;
  }
  .screen{
    width:620px;border-radius:20px;background:#10131a;padding:16px;
    display:grid;gap:14px;box-shadow:0 30px 70px -46px rgba(8,10,18,.9);
  }
  .screen-bar{display:flex;gap:7px}
  .screen-bar span{width:11px;height:11px;border-radius:50%;background:rgba(255,255,255,.22)}
  .screen-body{height:250px;border-radius:14px;background:#171b24;display:grid;place-items:center}
  .play{
    width:0;height:0;border-left:34px solid rgba(255,255,255,.92);
    border-top:21px solid transparent;border-bottom:21px solid transparent;
    margin-left:8px;
  }
  .waves{display:flex;align-items:flex-end;justify-content:center;gap:7px;height:26px}
  .waves u{width:7px;border-radius:4px;background:#7c3aed}
  </style><div class="stage">${body}</div>`;
}

async function main() {
  const work = await mkdtemp(join(tmpdir(), "fk-video-"));
  try {
    await mkdir(OUT_DIR, { recursive: true });
    process.stdout.write(`rendering ${TOTAL_FRAMES} frames`);

    for (let i = 0; i < TOTAL_FRAMES; i++) {
      const html = join(work, `f${i}.html`);
      await writeFile(html, framePage(i), "utf-8");
      await run(CHROME, [
        "--headless",
        "--disable-gpu",
        "--hide-scrollbars",
        `--screenshot=${join(work, `f${String(i).padStart(4, "0")}.png`)}`,
        `--window-size=${WIDTH},${HEIGHT}`,
        `file://${html}`,
      ]);
      if (i % 15 === 0) process.stdout.write(".");
    }
    process.stdout.write("\n");

    const mp4 = join(OUT_DIR, `${NAME}.mp4`);
    // Silent AAC track: the <video> element is muted-autoplay, but keeping the
    // stream layout identical to the other example clips avoids per-file
    // special-casing in players.
    await run("ffmpeg", [
      "-y", "-v", "error",
      "-framerate", String(FPS),
      "-i", join(work, "f%04d.png"),
      "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
      "-shortest",
      "-c:v", "libx264", "-preset", "slow", "-crf", "30",
      "-pix_fmt", "yuv420p", "-movflags", "+faststart",
      "-c:a", "aac", "-b:a", "48k",
      mp4,
    ]);

    // Poster comes from the pricing beat, which is the most legible still.
    await run("ffmpeg", [
      "-y", "-v", "error",
      "-i", mp4,
      "-ss", String(DURATION_S * 0.62),
      "-vframes", "1",
      join(OUT_DIR, `${NAME}.png`),
    ]);

    console.log(`wrote ${mp4}`);
  } finally {
    await rm(work, { recursive: true, force: true });
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
