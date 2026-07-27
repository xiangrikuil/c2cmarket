/** @type {import('@hey-api/openapi-ts').UserConfig} */
export default {
  input: '../docs/openapi/c2c-market-api-v1.yaml',
  output: 'src/api/generated/openapi',
  plugins: ['@hey-api/typescript'],
}
