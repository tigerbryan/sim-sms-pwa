import { createHash } from 'node:crypto'
import { readdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const dist = join(root, 'internal/web/dist')
async function files(dir) {
  const entries = await readdir(dir, { withFileTypes: true })
  return (await Promise.all(entries.map(entry => entry.isDirectory() ? files(join(dir, entry.name)) : join(dir, entry.name)))).flat()
}
// Includes icons and the worker, so an icon-only release is an application update too.
const hash = createHash('sha256')
for (const file of (await files(dist)).sort()) {
  hash.update(relative(dist, file)).update('\0').update(await readFile(file)).update('\0')
}
const id = hash.digest('hex').slice(0, 20)
const version = (await readFile(join(root, 'VERSION'), 'utf8')).trim()
const indexPath = join(dist, 'index.html')
let html = await readFile(indexPath, 'utf8')
html = html.replace('</head>', `<meta name="app-release" content="${id}" /><meta name="app-version" content="${version}" /></head>`)
html = html.replace(/(href="\/icons\/[^"?]+)(")/g, `$1?v=${id}$2`)
await writeFile(indexPath, html)
const manifestPath = join(dist, 'manifest.webmanifest')
const manifest = JSON.parse(await readFile(manifestPath, 'utf8'))
for (const icon of manifest.icons || []) icon.src += `?v=${id}`
await writeFile(manifestPath, JSON.stringify(manifest, null, 2) + '\n')
const workerPath = join(dist, 'sw.js')
await writeFile(workerPath, (await readFile(workerPath, 'utf8')).replaceAll('__PWA_RELEASE__', id))
await writeFile(join(dist, 'pwa-release.json'), JSON.stringify({ id, version }) + '\n')
console.log(`PWA release ${version} (${id})`)
