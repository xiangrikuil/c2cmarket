import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const migrationsDir = path.join(root, 'backend', 'migrations')
const readmePath = path.join(migrationsDir, 'README.md')
const postgresPath = path.join(root, 'backend', 'internal', 'database', 'postgres.go')

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

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log(`Migration docs check passed: ${migrations.length} migrations, latest version ${latestVersion}.`)
