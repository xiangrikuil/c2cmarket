#!/usr/bin/env node

import { createHash } from 'node:crypto'
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const testRoot = mkdtempSync(join(tmpdir(), 'c2cmarket-package-test-'))

const run = (command, args, options = {}) =>
  spawnSync(command, args, {
    cwd: testRoot,
    encoding: 'utf8',
    env: {
      ...process.env,
      GIT_AUTHOR_DATE: '2026-01-01T00:00:00Z',
      GIT_COMMITTER_DATE: '2026-01-01T00:00:00Z',
    },
    ...options,
  })

const mustRun = (command, args) => {
  const result = run(command, args)
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(' ')} failed:\n${result.stdout}\n${result.stderr}`,
    )
  }
  return result
}

const expectFailure = (command, args, expectedMessage) => {
  const result = run(command, args)
  if (result.status === 0 || !result.stderr.includes(expectedMessage)) {
    throw new Error(
      `expected failure containing ${JSON.stringify(expectedMessage)}:\n${result.stdout}\n${result.stderr}`,
    )
  }
}

const sha256 = path => createHash('sha256').update(readFileSync(path)).digest('hex')

try {
  mkdirSync(join(testRoot, 'scripts'), { recursive: true })
  mkdirSync(join(testRoot, 'output', 'tracked-assets'), { recursive: true })
  mkdirSync(join(testRoot, '.codex'), { recursive: true })
  mkdirSync(join(testRoot, '.trellis', 'tasks', 'fixture'), { recursive: true })
  mkdirSync(join(testRoot, '.trellis', 'workspace'), { recursive: true })
  mkdirSync(join(testRoot, 'frontend'), { recursive: true })
  copyFileSync(
    join(repoRoot, 'scripts', 'package-source.sh'),
    join(testRoot, 'scripts', 'package-source.sh'),
  )
  copyFileSync(join(repoRoot, '.gitattributes'), join(testRoot, '.gitattributes'))
  copyFileSync(join(repoRoot, '.gitignore'), join(testRoot, '.gitignore'))
  writeFileSync(join(testRoot, 'safe.txt'), 'tracked source\n')
  writeFileSync(join(testRoot, '.env.example'), 'SAFE_EXAMPLE=true\n')
  writeFileSync(join(testRoot, '.env.production.example'), 'SAFE_EXAMPLE=true\n')
  writeFileSync(join(testRoot, '.env.staging.example'), 'SAFE_EXAMPLE=true\n')
  writeFileSync(join(testRoot, 'frontend', '.env.development'), 'VITE_API_MODE=real\n')
  writeFileSync(join(testRoot, 'output', 'tracked-assets', 'asset.bin'), 'generated\n')
  writeFileSync(join(testRoot, '.codex', 'context.json'), '{}\n')
  writeFileSync(join(testRoot, '.trellis', 'tasks', 'fixture', 'prd.md'), 'task\n')
  writeFileSync(join(testRoot, '.trellis', 'workspace', 'journal.md'), 'local history\n')

  mustRun('git', ['init', '--quiet'])
  mustRun('git', ['config', 'user.name', 'Codex Test'])
  mustRun('git', ['config', 'user.email', 'codex@example.invalid'])
  mustRun('git', ['add', '--force', '.'])
  mustRun('git', ['commit', '--quiet', '-m', 'fixture'])

  mustRun('bash', ['scripts/package-source.sh', 'HEAD', 'first.tar.gz'])
  mustRun('bash', ['scripts/package-source.sh', 'HEAD', 'second.tar.gz'])

  const firstArchive = join(testRoot, 'output', 'first.tar.gz')
  const secondArchive = join(testRoot, 'output', 'second.tar.gz')
  if (sha256(firstArchive) !== sha256(secondArchive)) {
    throw new Error('same commit produced different archive hashes')
  }
  const checksum = readFileSync(`${firstArchive}.sha256`, 'utf8')
  if (checksum !== `${sha256(firstArchive)}  first.tar.gz\n`) {
    throw new Error(`unexpected checksum file: ${JSON.stringify(checksum)}`)
  }

  const contents = mustRun('tar', ['-tzf', firstArchive]).stdout
  for (const required of [
    '/safe.txt',
    '/.env.example',
    '/.env.production.example',
    '/.env.staging.example',
  ]) {
    if (!contents.includes(required)) {
      throw new Error(`archive is missing ${required}`)
    }
  }
  for (const forbidden of [
    '/output/',
    '/.codex/',
    '/.trellis/tasks/',
    '/.trellis/workspace/',
    '/frontend/.env.development',
  ]) {
    if (contents.includes(forbidden)) {
      throw new Error(`archive contains forbidden path ${forbidden}`)
    }
  }

  writeFileSync(join(testRoot, 'dirty.txt'), 'not committed\n')
  expectFailure(
    'bash',
    ['scripts/package-source.sh', 'HEAD', 'dirty.tar.gz'],
    'working tree must be clean',
  )
  if (existsSync(join(testRoot, 'output', 'dirty.tar.gz'))) {
    throw new Error('dirty-tree failure left a final archive')
  }

  mustRun('git', ['add', 'dirty.txt'])
  expectFailure(
    'bash',
    ['scripts/package-source.sh', 'HEAD', 'staged.tar.gz'],
    'working tree must be clean',
  )
  if (existsSync(join(testRoot, 'output', 'staged.tar.gz'))) {
    throw new Error('staged-tree failure left a final archive')
  }
  mustRun('git', ['restore', '--staged', 'dirty.txt'])
  rmSync(join(testRoot, 'dirty.txt'))

  writeFileSync(join(testRoot, 'safe.txt'), 'unstaged change\n')
  expectFailure(
    'bash',
    ['scripts/package-source.sh', 'HEAD', 'unstaged.tar.gz'],
    'working tree must be clean',
  )
  if (existsSync(join(testRoot, 'output', 'unstaged.tar.gz'))) {
    throw new Error('unstaged-tree failure left a final archive')
  }
  writeFileSync(join(testRoot, 'safe.txt'), 'tracked source\n')

  expectFailure(
    'bash',
    ['scripts/package-source.sh', 'missing-ref', 'missing.tar.gz'],
    'does not resolve to a commit',
  )
  expectFailure(
    'bash',
    ['scripts/package-source.sh', 'HEAD', '../unsafe.tar.gz'],
    'archive name must be a basename',
  )

  writeFileSync(join(testRoot, '.env.production'), 'SECRET=must-not-ship\n')
  mustRun('git', ['add', '--force', '.env.production'])
  mustRun('git', ['commit', '--quiet', '-m', 'forbidden env'])
  expectFailure(
    'bash',
    ['scripts/package-source.sh', 'HEAD', 'forbidden-env.tar.gz'],
    'forbidden environment file',
  )
  if (existsSync(join(testRoot, 'output', 'forbidden-env.tar.gz'))) {
    throw new Error('forbidden-content failure left a final archive')
  }

  console.log('Source package tests passed.')
} finally {
  rmSync(testRoot, { recursive: true, force: true })
}
