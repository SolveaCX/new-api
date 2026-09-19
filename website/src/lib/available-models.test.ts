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
      ? { success: true, data: { scope_mode: 'fixed_account', groups: [], account_model_ids: ['healthy', 'failed', 'legacy'], models: [
        { id: 'healthy', tags: 'HOT,Custom', availability_status: 'available', supported_endpoint_types: ['openai'] },
        { id: 'failed', availability_status: 'temporary_failure', supported_endpoint_types: [] },
        { id: 'legacy', availability_status: 'unknown', supported_endpoint_types: ['gemini'] },
      ] } }
      : { success: true, data: ['legacy', 'private', 'failed', 'healthy'].map(model_name => ({ model_name, tags: 'Old tag', model_ratio: 2, completion_ratio: 1, quota_type: 0 })) };
    return Response.json(payload);
  }) as typeof fetch;
  const result = await getAvailableModelPricingData();
  expect(result.models.map(model => model.model_name)).toEqual(['legacy', 'healthy']);
  expect(result.models[1].model_ratio).toBe(2);
  expect(result.models[1].tags).toBe('HOT,Custom');
  expect(result.models[0].tags).toBe('');
  expect(result.models[1].supported_endpoint_types).toEqual(['openai']);
  expect(requests.some(url => url.endsWith('/api/website/model-access?view=available_models'))).toBe(true);
  expect(requests.some(url => url.endsWith('/api/website/pricing?group=plg'))).toBe(true);
});

test('catalog failure never falls back to the broader pricing list', async () => {
  globalThis.fetch = (async (input: RequestInfo | URL) => String(input).includes('/model-access')
    ? new Response('unavailable', { status: 503 })
    : Response.json({ success: true, data: [{ model_name: 'private' }] })) as typeof fetch;
  expect((await getAvailableModelPricingData()).models).toEqual([]);
});


test('authenticated website matches the 21 account IDs, not the 104 metadata/pricing rows', async () => {
  const requests: { url: string; headers: Record<string, string> }[] = [];
  const models = Array.from({ length: 104 }, (_, i) => ({ id: `model-${i}`, availability_status: 'available', supported_endpoint_types: ['openai'] }));
  const allowed = models.slice(0, 21).map(model => model.id);
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    requests.push({ url, headers: init?.headers as Record<string, string> });
    if (url.includes('analytics-self')) return Response.json({ success: true, data: { id: 7 } });
    if (url.includes('/user/model-access')) return Response.json({ success: true, data: {
      scope_mode: 'fixed_account', groups: [], account_model_ids: allowed, models,
    } });
    return Response.json({ success: true, data: models.map(model => ({ model_name: model.id, model_ratio: 2, completion_ratio: 1, quota_type: 0 })) });
  }) as typeof fetch;
  const result = await getAvailableModelPricingData('session=fixture');
  expect(result.models.map(model => model.model_name)).toEqual(allowed);
  const catalogRequest = requests.find(request => request.url.includes('/user/model-access'))!;
  expect(catalogRequest.url).toEndWith('/api/user/model-access?view=available_models');
  expect(catalogRequest.headers).toMatchObject({ cookie: 'session=fixture', 'New-Api-User': '7' });
  expect(requests.some(request => request.url.includes('/website/model-access'))).toBe(false);
});

test('authenticated catalog failure does not fall back to the public catalog', async () => {
  const requests: string[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = String(input); requests.push(url);
    if (url.includes('analytics-self')) return Response.json({ success: true, data: { id: 7 } });
    if (url.includes('/user/model-access')) return new Response('unavailable', { status: 503 });
    return Response.json({ success: true, data: [] });
  }) as typeof fetch;
  expect((await getAvailableModelPricingData('session=fixture')).models).toEqual([]);
  expect(requests.some(url => url.includes('/website/model-access'))).toBe(false);
});

test('anonymous visitors use only the public account IDs, including when cookies are unrelated', async () => {
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes('analytics-self')) return new Response('', { status: 401 });
    if (url.includes('model-access')) return Response.json({ success: true, data: {
      scope_mode: 'fixed_account', groups: [], account_model_ids: ['public'], models: [
        { id: 'public', availability_status: 'available', supported_endpoint_types: [] },
        { id: 'private', availability_status: 'available', supported_endpoint_types: [] },
      ],
    } });
    return Response.json({ success: true, data: [{ model_name: 'public', model_ratio: 2, completion_ratio: 1, quota_type: 0 }] });
  }) as typeof fetch;
  expect((await getAvailableModelPricingData('consent=granted')).models.map(model => model.model_name)).toEqual(['public']);
});

test('keeps available models absent from pricing instead of silently shrinking the list', async () => {
  globalThis.fetch = (async (input: RequestInfo | URL) => String(input).includes('model-access')
    ? Response.json({ success: true, data: {
      scope_mode: 'fixed_account', groups: [], account_model_ids: ['catalog-only'],
      models: [{ id: 'catalog-only', tags: 'HOT', availability_status: 'available', supported_endpoint_types: ['openai'] }],
    } })
    : Response.json({ success: true, data: [{ model_name: 'pricing-only', model_ratio: 2, completion_ratio: 1, quota_type: 0 }] })) as typeof fetch;
  const result = await getAvailableModelPricingData();
  expect(result.models.map(model => model.model_name)).toEqual(['catalog-only']);
  expect(Number.isNaN(result.models[0].model_ratio)).toBe(true);
});

for (const authenticated of [false, true]) {
  test(`preserves pricing and directory fields when merging the ${authenticated ? 'authenticated' : 'public'} catalog`, async () => {
    const metadata = {
      author: 'Author', providers: ['Provider'], modalities: ['text', 'image'],
      context_tokens: 128000, series: 'Series', categories: ['coding'],
      released_at: '2026-01-01', distillable: false, popularity_rank: 3,
    };
    const priceRow = {
      model_name: 'model', id: 12, icon: 'icon', vendor_id: 4,
      vendor_description: 'Vendor description', quota_type: 0,
      model_ratio: 2, completion_ratio: 3, model_price: 0.1,
      cache_ratio: 0.2, create_cache_ratio: 1.25, image_ratio: 2,
      audio_ratio: 4, audio_completion_ratio: 5, enable_groups: ['plg'],
      group_ratio: { plg: 0.8 }, group_model_ratio: { plg: 0.6 },
      billing_mode: 'tiered', billing_expr: 'p * 2', pricing_version: 'v1',
      featured_order: 2, featured_config: { display_name: 'Featured', tags: 'hero' },
      directory_metadata: metadata, availability_reason: 'healthy',
      availability_detected_at: 123, availability_checked_at: 456,
    };
    const display = { billing_kind: 'token', prices: { input: { configured: 4, plg: 2.4 } } };
    const extras = {
      vendors: [{ id: 4, name: 'Vendor', icon: 'icon' }],
      group_ratio: { plg: 0.8 }, group_model_ratio: { plg: { model: 0.6 } },
      usable_group: { plg: 'Public' }, supported_endpoint: { openai: { path: '/v1/chat/completions' } },
      auto_groups: ['plg'],
    };
    globalThis.fetch = (async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('analytics-self')) return Response.json({ success: true, data: { id: 7 } });
      if (url.includes('model-access')) return Response.json({ success: true, data: {
        scope_mode: 'fixed_account', groups: [], account_model_ids: ['model'], models: [{
          id: 'model', description: 'Catalog description', tags: 'HOT,Custom', display_weight: 9,
          vendor: { id: 4, name: 'Vendor', icon: 'icon' },
          supported_endpoint_types: ['openai', 'image-generation'], availability_status: 'available',
        }],
      } });
      return Response.json({ success: true, data: [priceRow], ...extras,
        ...(url.includes('/website/pricing') ? { display_pricing: { model: display } } : {}),
      });
    }) as typeof fetch;
    const result = await getAvailableModelPricingData(authenticated ? 'session=fixture' : '');
    expect(result.models).toHaveLength(1);
    expect(result.models[0]).toMatchObject({ ...priceRow, display_pricing: display,
      description: 'Catalog description', tags: 'HOT,Custom', display_weight: 9,
      vendor_name: 'Vendor', vendor_icon: 'icon',
      supported_endpoint_types: ['openai', 'image-generation'], availability_status: 'available',
    });
    expect(result).toMatchObject({ vendors: extras.vendors, groupRatio: extras.group_ratio,
      groupModelRatio: extras.group_model_ratio, usableGroup: extras.usable_group,
      supportedEndpoint: extras.supported_endpoint, autoGroups: extras.auto_groups });
  });
}

test('selectable scope excludes metadata belonging only to other groups', async () => {
  globalThis.fetch = (async (input: RequestInfo | URL) => String(input).includes('model-access')
    ? Response.json({ success: true, data: {
      scope_mode: 'selectable', create_default_scope: 'selected',
      groups: [{ id: 'other', model_ids: ['other'] }, { id: 'selected', model_ids: ['selected'] }],
      models: ['selected', 'other'].map(id => ({ id, availability_status: 'available', supported_endpoint_types: ['openai'] })),
    } }) : Response.json({ success: true, data: [] })) as typeof fetch;
  expect((await getAvailableModelPricingData()).models.map(model => model.model_name)).toEqual(['selected']);
});
