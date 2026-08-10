import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

export function seedVerifiedProbeConnection(ownerUserId) {
  if (!uuidPattern.test(ownerUserId)) {
    throw new Error(`invalid smoke owner user id: ${ownerUserId}`)
  }

  const connectionId = randomUUID()
  const sql = `
    INSERT INTO api_probe_connections (
      id, owner_user_id, name, base_url, normalized_base_url,
      credential_ciphertext, credential_nonce, credential_key_version,
      credential_cipher_format, credential_fingerprint,
      probe_model, probe_protocol, enabled, verification_status, verified_at
    ) VALUES (
      '${connectionId}', '${ownerUserId}', 'Smoke verified probe',
      'https://api.example.com/v1', 'https://api.example.com/v1',
      decode('0102', 'hex'), decode('000000000000000000000000', 'hex'),
      'smoke-v1', 'smoke-v1', decode('0304', 'hex'),
      'gpt-5-mini', 'openai_responses_v1', true, 'verified', now()
    );
  `

  try {
    execFileSync('docker', [
      'compose', 'exec', '-T', 'postgres', 'sh', '-c',
      'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$1"',
      'smoke-probe-seed', sql,
    ], { cwd: process.cwd(), encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
  } catch (error) {
    const detail = error?.stderr?.trim() || error?.message || String(error)
    throw new Error(`failed to seed verified smoke probe connection: ${detail}`)
  }

  return connectionId
}
