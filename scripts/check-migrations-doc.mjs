import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const migrationsDir = path.join(root, 'backend', 'migrations')
const readmePath = path.join(migrationsDir, 'README.md')
const postgresPath = path.join(root, 'backend', 'internal', 'database', 'postgres.go')

const currentVersionAssertions = [
  {
    file: 'backend/README.md',
    assertion: 'current expected migration version',
    pattern: /当前期望版本为 `([0-9]+)`/g,
  },
  {
    file: 'docs/backup-restore.md',
    assertion: 'restore drill target version',
    pattern: /require `version=([0-9]+)` and `dirty=false`/g,
  },
  {
    file: 'docs/backup-restore.md',
    assertion: 'production recovery target version',
    pattern: /After restore, require migration `([0-9]+):false`/g,
  },
  {
    file: 'docs/operations.md',
    assertion: 'readyz migration version',
    pattern: /PostgreSQL reachability and migration `([0-9]+):false`/g,
  },
  {
    file: 'docs/ops/deployment-runbook.md',
    assertion: 'current backend schema version',
    pattern: /expected schema version in the current backend is `([0-9]+)`/gi,
  },
  {
    file: 'docs/ops/deployment-runbook.md',
    assertion: 'version endpoint expectedMigrationVersion',
    pattern: /`expectedMigrationVersion=([0-9]+)`/g,
  },
  {
    file: 'docs/release-checklist.md',
    assertion: 'migration review target version',
    pattern: /through current migration ([0-9]+) were reviewed/g,
  },
  {
    file: 'docs/release-checklist.md',
    assertion: 'staging migration target version',
    pattern: /Staging migrated from its current version through ([0-9]+) with `dirty=false`/g,
  },
  {
    file: 'docs/release-checklist.md',
    assertion: 'production migration target version',
    pattern: /require `schema_migrations=([0-9]+):false`/g,
  },
  {
    file: '.trellis/spec/backend/release-contract.md',
    assertion: 'version response example expectedMigrationVersion',
    pattern: /"expectedMigrationVersion":\s*([0-9]+)/g,
  },
  {
    file: '.trellis/spec/backend/api-contracts.md',
    assertion: 'active ExpectedMigrationVersion contract',
    pattern: /ExpectedMigrationVersion\s*=\s*([0-9]+) \(current repository target\)/g,
  },
  {
    file: '.trellis/spec/backend/database-guidelines.md',
    assertion: 'active ExpectedMigrationVersion contract',
    pattern: /ExpectedMigrationVersion\s*=\s*([0-9]+) \(current repository target\)/g,
  },
]

const migrationFilePattern = /^(\d{6})_.+\.up\.sql$/
const files = await readdir(migrationsDir)
const migrations = files
  .filter(file => migrationFilePattern.test(file))
  .map(file => ({
    file,
    name: file.replace(/\.up\.sql$/, ''),
    version: Number(file.match(migrationFilePattern)?.[1]),
  }))
  .sort((a, b) => a.version - b.version)

if (migrations.length === 0) {
  throw new Error(`No migration *.up.sql files found in ${migrationsDir}`)
}

const readme = await readFile(readmePath, 'utf8')
const missingFromReadme = migrations
  .map(migration => migration.name)
  .filter(name => !readme.includes(`\`${name}\``))

const postgresSource = await readFile(postgresPath, 'utf8')
const expectedVersionMatch = postgresSource.match(/const\s+ExpectedMigrationVersion\s+int64\s*=\s*(\d+)/)
const latestVersion = migrations.at(-1)?.version
const expectedVersion = expectedVersionMatch ? Number(expectedVersionMatch[1]) : null

const failures = []
if (missingFromReadme.length > 0) {
  failures.push(`backend/migrations/README.md is missing: ${missingFromReadme.join(', ')}`)
}
if (expectedVersion === null) {
  failures.push('ExpectedMigrationVersion constant was not found in backend/internal/database/postgres.go')
} else if (expectedVersion !== latestVersion) {
  failures.push(`ExpectedMigrationVersion is ${expectedVersion}, but latest migration is ${latestVersion}`)
}

const currentDocumentSources = new Map()
for (const { file, assertion, pattern } of currentVersionAssertions) {
  let source = currentDocumentSources.get(file)
  if (source === undefined) {
    source = await readFile(path.join(root, file), 'utf8')
    currentDocumentSources.set(file, source)
  }

  const documentedVersions = [...source.matchAll(pattern)].map(match => Number(match[1]))
  if (documentedVersions.length === 0) {
    failures.push(`${file} is missing its ${assertion} assertion`)
  } else if (expectedVersion !== null && documentedVersions.some(version => version !== expectedVersion)) {
    failures.push(
      `${file} ${assertion} is ${documentedVersions.join(', ')}, but ExpectedMigrationVersion is ${expectedVersion}`,
    )
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log(
  `Migration docs check passed: ${migrations.length} migrations, latest version ${latestVersion}, current documentation matches.`,
)
