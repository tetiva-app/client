// Turns a capture-tour.mjs frame dump into the clip the tour ships.
//
//   node scripts/assemble-tour.mjs --scene=protocols --theme=dark
//
// Holds are hardlinks, not copies — a 6-second clip is mostly still frames and
// h264 encodes those for almost nothing. Writes src/assets/onboarding/<scene>-<theme>.{mp4,webp}.

import { execFileSync } from 'node:child_process'
import { link, mkdir, readFile, readdir, rm, unlink } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const args = Object.fromEntries(
  process.argv.slice(2).map((a) => a.replace(/^--/, '').split('=')),
)
const SCENE = args.scene ?? 'protocols'
const THEME = args.theme ?? 'dark'

const HERE = dirname(fileURLToPath(import.meta.url))
const SRC_DIR = resolve(HERE, '../../.tour-capture', `${SCENE}-${THEME}`)
const ASSET_DIR = resolve(HERE, '../src/assets/onboarding')

// ~2x the tour's media slot; editor crops arrive 1688 px wide, workspaces 1120.
const WIDTH = 1320
const FPS = 20
const CRF = 21

const frame = (n) => `${SRC_DIR}/f-${String(n).padStart(4, '0')}.png`
const seqFrame = (n) => `${SRC_DIR}/seq-${String(n).padStart(4, '0')}.png`

async function main() {
  const manifest = JSON.parse(await readFile(`${SRC_DIR}/manifest.json`, 'utf8'))

  for (const name of await readdir(SRC_DIR)) {
    if (name.startsWith('seq-')) await unlink(`${SRC_DIR}/${name}`)
  }

  // Tab switches are a hard cut between two unrelated screens, which reads as a
  // glitch at 20 fps. Blended in-between frames turn it into a dissolve.
  let seq = 1
  let previous = null
  for (const [index, copies, fade = 0] of manifest.holds) {
    if (fade > 0 && previous !== null && previous !== index) {
      for (let step = 1; step <= fade; step++) {
        execFileSync('magick', [
          'composite', '-blend', String(Math.round((step * 100) / (fade + 1))),
          frame(index), frame(previous), seqFrame(seq++),
        ])
      }
    }
    for (let i = 0; i < copies; i++) await link(frame(index), seqFrame(seq++))
    previous = index
  }

  await mkdir(ASSET_DIR, { recursive: true })
  const mp4 = `${ASSET_DIR}/${SCENE}-${THEME}.mp4`
  const poster = `${ASSET_DIR}/${SCENE}-${THEME}.webp`
  await rm(mp4, { force: true })
  await rm(poster, { force: true })

  execFileSync('ffmpeg', [
    '-loglevel', 'error',
    '-framerate', String(FPS),
    '-start_number', '1',
    '-i', `${SRC_DIR}/seq-%04d.png`,
    '-vf', `scale=${WIDTH}:-2:flags=lanczos`,
    '-c:v', 'libx264', '-preset', 'veryslow', '-crf', String(CRF),
    '-tune', 'stillimage',
    '-profile:v', 'high', '-level', '4.0',
    '-pix_fmt', 'yuv420p',
    // A UI screencast is mostly still frames, so keyframes are the bulk of the
    // file: one per clip instead of one every two seconds halves it.
    '-g', '600',
    '-movflags', '+faststart',
    '-an',
    mp4,
  ])

  execFileSync('ffmpeg', [
    '-loglevel', 'error',
    '-i', frame(manifest.holds[0][0]),
    '-vf', `scale=${WIDTH}:-2:flags=lanczos`,
    '-frames:v', '1',
    '-c:v', 'libwebp', '-quality', '82', '-compression_level', '6',
    poster,
  ])

  const seconds = (seq - 1) / FPS
  console.log(`${mp4} — ${seconds.toFixed(1)}s`)
}

await main()
