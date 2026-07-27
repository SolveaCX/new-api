/*
 * Derives the Amazon review specs from VOC's own MCP tool list rather than
 * hand-writing them, so the input schemas stay whatever the upstream actually
 * accepts. Source of truth is the generated catalogue in voc-tools-hub, which
 * was itself produced from a live tools/list call.
 *
 * Usage: node scripts/gen-voc-amazon-specs.mjs
 */
import { readFileSync, writeFileSync } from 'node:fs'

const SOURCE = '/Users/hunter/voc-tools-hub/registry/specs/voc-generated.json'
const TARGET = 'data/tools/specs/voc-amazon.json'

// Amazon review coverage only. The rest of VOC's 480 endpoints are a separate
// decision — shipping a tool means committing to its upstream contract.
const WANTED = new Set([
  'voc.amazon_reviews_fetch_realtime',
  'voc.amazon_reviews_fetch_history',
  'voc.amazon_reviews_analyze',
])

const source = JSON.parse(readFileSync(SOURCE, 'utf8'))
const tools = Array.isArray(source) ? source : (source.tools ?? source.specs ?? [])

/** voc-tools-hub writes auth as {envKey, header}; this repo uses {type,name,envKey}. */
function convertAuth(auth) {
  if (!auth) return { type: 'none' }
  if (auth.bearer) return { type: 'bearer', envKey: auth.envKey }
  return { type: 'header', name: auth.header || 'X-API-Key', envKey: auth.envKey }
}

const out = []
for (const tool of tools) {
  if (!WANTED.has(tool.id)) continue
  out.push({
    id: tool.id,
    name: tool.name,
    // The marketplace platform chip is the segment after the colon.
    provider: 'voc:amazon',
    mode: tool.mode ?? 'native',
    categories: ['ecommerce', 'reviews', 'amazon'],
    description: tool.description,
    keywords: [...new Set([...(tool.keywords ?? []), 'amazon', 'reviews', 'asin', 'ecommerce'])],
    input: tool.input,
    pricing: {
      model: tool.pricing?.model ?? 'per_call',
      amount: tool.pricing?.amount ?? 0,
      cost: tool.pricing?.cost ?? 0,
      payOnMatch: Boolean(tool.pricing?.payOnMatch),
    },
    adapter: {
      kind: 'mcp',
      url: tool.adapter.url,
      toolName: tool.adapter.toolName,
      auth: convertAuth(tool.adapter.auth),
    },
  })
}

if (out.length !== WANTED.size) {
  throw new Error(`expected ${WANTED.size} specs, derived ${out.length}`)
}

writeFileSync(TARGET, `${JSON.stringify(out, null, 2)}\n`)
console.log(`${TARGET}: ${out.length} specs`)
for (const s of out) console.log(`  ${s.id}  ${s.adapter.kind}  $${s.pricing.amount}`)
