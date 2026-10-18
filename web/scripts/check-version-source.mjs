import { readdir, readFile } from 'node:fs/promises'
import { dirname, extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const sourceRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src')
const forbidden = /\bv?0\.1\.(?:1|2|3)\b/g
const files = []

async function collect(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) {
      await collect(path)
    } else if (!entry.name.includes('.test.') && ['.ts', '.vue', '.css'].includes(extname(entry.name))) {
      files.push(path)
    }
  }
}

await collect(sourceRoot)
const matches = []
for (const path of files) {
  const content = await readFile(path, 'utf8')
  for (const match of content.matchAll(forbidden)) {
    matches.push(`${path}:${match.index}`)
  }
}

if (matches.length > 0) {
  console.error('Production web/src must obtain business versions from /version, not hardcode them:')
  for (const match of matches) console.error(match)
  process.exitCode = 1
}
