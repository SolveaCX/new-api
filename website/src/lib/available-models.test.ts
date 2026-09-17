import { afterEach, expect, test } from 'bun:test';
import { getAvailableModelPricingData } from './available-models';

const originalFetch = globalThis.fetch;
afterEach(() => { globalThis.fetch = originalFetch; });

test('directory membership and health come from the public catalog, preserving pricing order', async () => {
  const requests: string[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = String(input);
    requests.push(url);
    const payload = url.includes('/model-access')
      ? { success: true, data: { models: [
        { id: 'healthy', availability_status: 'available', supported_endpoint_types: ['openai'] },
        { id: 'failed', availability_status: 'temporary_failure', supported_endpoint_types: [] },
        { id: 'legacy', availability_status: 'unknown', supported_endpoint_types: ['gemini'] },
      ] } }
      : { success: true, data: ['legacy', 'private', 'failed', 'healthy'].map(model_name => ({ model_name, model_ratio: 2, completion_ratio: 1, quota_type: 0 })) };
    return Response.json(payload);
  }) as typeof fetch;
  const result = await getAvailableModelPricingData();
  expect(result.models.map(model => model.model_name)).toEqual(['legacy', 'healthy']);
  expect(result.models[1].model_ratio).toBe(2);
  expect(result.models[1].supported_endpoint_types).toEqual(['openai']);
  expect(requests.some(url => url.endsWith('/api/website/model-access'))).toBe(true);
  expect(requests.some(url => url.endsWith('/api/website/pricing?group=plg'))).toBe(true);
});

test('catalog failure never falls back to the broader pricing list', async () => {
  globalThis.fetch = (async (input: RequestInfo | URL) => String(input).includes('/model-access')
    ? new Response('unavailable', { status: 503 })
    : Response.json({ success: true, data: [{ model_name: 'private' }] })) as typeof fetch;
  expect((await getAvailableModelPricingData()).models).toEqual([]);
});
