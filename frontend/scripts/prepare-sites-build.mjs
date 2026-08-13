import { cp, mkdir, readdir, rename, rm } from 'node:fs/promises'
import { join } from 'node:path'

const root = process.cwd()
const dist = join(root, 'dist')
const client = join(dist, 'client')

await rm(client, { recursive: true, force: true })
await mkdir(client, { recursive: true })

for (const entry of await readdir(dist, { withFileTypes: true })) {
  if (entry.name === 'client' || entry.name === 'server' || entry.name === '.openai') continue
  await rename(join(dist, entry.name), join(client, entry.name))
}

await mkdir(join(dist, 'server'), { recursive: true })
await cp(join(root, 'sites-worker.mjs'), join(dist, 'server', 'index.js'))
