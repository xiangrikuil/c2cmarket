import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const frontendRoot = resolve(repoRoot, 'frontend')
const expectedDir = resolve(frontendRoot, 'src/api/generated/openapi')
const generatorPath = resolve(
  frontendRoot,
  'node_modules/@hey-api/openapi-ts/bin/run.js',
)
const temporaryDir = await mkdtemp(join(tmpdir(), 'c2cmarket-openapi-types.'))

async function listFiles(root, current = root) {
  const entries = await readdir(current, { withFileTypes: true })
  const files = []

  for (const entry of entries) {
    const entryPath = join(current, entry.name)
    if (entry.isDirectory()) {
      files.push(...(await listFiles(root, entryPath)))
    } else if (entry.isFile()) {
      files.push(relative(root, entryPath))
    }
  }

  return files.sort()
}

try {
  const result = spawnSync(
    process.execPath,
    [
      generatorPath,
      '--file',
      'openapi-ts.config.mjs',
      '--output',
      temporaryDir,
      '--no-log-file',
    ],
    {
      cwd: frontendRoot,
      encoding: 'utf8',
    },
  )

  if (result.error) {
    throw result.error
  }
  if (result.status !== 0) {
    throw new Error(
      `OpenAPI generation failed:\n${result.stdout}${result.stderr}`.trim(),
    )
  }
  if (result.stderr.trim()) {
    throw new Error(`OpenAPI generation wrote to stderr:\n${result.stderr}`)
  }
  if (/\bwarn(?:ing)?\b/i.test(result.stdout)) {
    throw new Error(`OpenAPI generation reported a warning:\n${result.stdout}`)
  }

  const [expectedFiles, actualFiles] = await Promise.all([
    listFiles(expectedDir),
    listFiles(temporaryDir),
  ])

  if (expectedFiles.join('\n') !== actualFiles.join('\n')) {
    throw new Error(
      [
        'Generated OpenAPI file set differs from the committed snapshot.',
        `Expected:\n${expectedFiles.join('\n')}`,
        `Actual:\n${actualFiles.join('\n')}`,
      ].join('\n'),
    )
  }

  for (const file of expectedFiles) {
    const [expected, actual] = await Promise.all([
      readFile(join(expectedDir, file)),
      readFile(join(temporaryDir, file)),
    ])
    if (!expected.equals(actual)) {
      throw new Error(`Generated OpenAPI file differs: ${file}`)
    }
  }

  console.log(
    `OpenAPI generated types match the committed snapshot (${expectedFiles.length} files).`,
  )
} finally {
  await rm(temporaryDir, { force: true, recursive: true })
}
