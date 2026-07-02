import { describe, expect, test } from "bun:test";
import {
  buildModelQuickStart,
  getIndexableModelDetailPathnames,
  getIndexableRelatedModels,
  getModelDetailPath,
  getRoutableModelDetailPathnames,
  modelDetailMetadataCopy,
  isModelDetailIndexable,
  isModelDetailRoutable,
  modelSlugFromName,
  resolveModelBySlug,
} from "./model-detail";
import { resolveVendorName, type PricingModel } from "./pricing";

const models: PricingModel[] = [
  {
    model_name: "openai/gpt-4.1",
    vendor_name: "OpenAI",
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 4,
    supported_endpoint_types: ["chat"],
    availability_status: "available",
  },
  {
    model_name: "gemini-2.5-flash",
    vendor_name: "Google",
    quota_type: 0,
    model_ratio: 0.1,
    completion_ratio: 4,
    supported_endpoint_types: ["chat"],
    availability_status: "available",
  },
];

describe("model detail routes", () => {
  test("normalizes model names into stable URL slugs", () => {
    expect(modelSlugFromName("openai/gpt-4.1")).toBe("openai-gpt-4-1");
    expect(modelSlugFromName(" gemini_2.5 flash ")).toBe("gemini-2-5-flash");
  });

  test("resolves a pricing model from its slug", () => {
    expect(resolveModelBySlug(models, "openai-gpt-4-1")?.model_name).toBe("openai/gpt-4.1");
    expect(resolveModelBySlug(models, "missing-model")).toBeNull();
  });

  test("does not resolve ambiguous slug collisions", () => {
    const collidingModels = [
      models[0],
      { ...models[1], model_name: "openai gpt 4.1", vendor_name: "Other" },
    ];

    expect(resolveModelBySlug(collidingModels, "openai-gpt-4-1")).toBeNull();
  });

  test("only exposes indexable model detail paths for stable, useful pages", () => {
    const indexable = models[0];
    const missingEndpoint = {
      ...models[1],
      supported_endpoint_types: undefined,
      availability_status: undefined,
    };
    const missingVendor = {
      ...models[1],
      model_name: "vendorless-model",
      vendor_name: undefined,
    };
    const candidates = [indexable, missingEndpoint, missingVendor];

    expect(isModelDetailIndexable(indexable, candidates)).toBe(true);
    expect(isModelDetailIndexable(missingEndpoint, candidates)).toBe(false);
    expect(isModelDetailIndexable(missingVendor, candidates)).toBe(false);
    expect(getIndexableModelDetailPathnames(candidates)).toEqual(["/models/openai-gpt-4-1"]);
  });

  test("keeps stable routable paths separate from SEO indexability", () => {
    const missingEndpoint = {
      ...models[0],
      model_name: "stable-but-noindex",
      supported_endpoint_types: undefined,
      availability_status: undefined,
    };

    expect(isModelDetailRoutable(missingEndpoint, [missingEndpoint])).toBe(true);
    expect(isModelDetailIndexable(missingEndpoint, [missingEndpoint])).toBe(false);
    expect(getRoutableModelDetailPathnames([missingEndpoint])).toEqual(["/models/stable-but-noindex"]);
    expect(getIndexableModelDetailPathnames([missingEndpoint])).toEqual([]);
  });

  test("requires real vendor metadata before exposing automatic detail paths", () => {
    const unresolvedVendorModel: PricingModel = {
      ...models[0],
      model_name: "unresolved-vendor-model",
      vendor_name: resolveVendorName({ ...models[0], vendor_name: undefined, vendor_id: 404 }, []),
      vendor_id: 404,
    };

    expect(isModelDetailIndexable(unresolvedVendorModel, [unresolvedVendorModel])).toBe(false);
    expect(getIndexableModelDetailPathnames([unresolvedVendorModel])).toEqual([]);
  });

  test("filters related models through the same indexability gate", () => {
    const current = models[0];
    const indexableRelated = {
      ...models[1],
      model_name: "openai/gpt-4.1-mini",
      vendor_name: "OpenAI",
    };
    const collidingRelated = {
      ...models[1],
      model_name: "openai gpt 4.1 mini",
      vendor_name: "OpenAI",
    };
    const hiddenRelated = {
      ...models[1],
      model_name: "openai/internal",
      vendor_name: "OpenAI",
      supported_endpoint_types: undefined,
      availability_status: undefined,
    };

    expect(getIndexableRelatedModels(current, [current, indexableRelated, collidingRelated, hiddenRelated])).toEqual([]);
    expect(getIndexableRelatedModels(current, [current, indexableRelated])).toEqual([indexableRelated]);
  });

  test("localizes automatic model detail metadata", () => {
    expect(modelDetailMetadataCopy("zh", models[0]).title).toBe("openai/gpt-4.1 API 价格与可用性");
    expect(modelDetailMetadataCopy("ja", models[0]).description).toContain("openai/gpt-4.1");
    expect(modelDetailMetadataCopy("ja", models[0]).description).not.toContain("Live flatkey pricing");
  });

  test("builds endpoint-specific quickstarts for non-chat models", () => {
    const embeddingModel: PricingModel = {
      ...models[0],
      model_name: "text-embedding-3-small",
      supported_endpoint_types: ["embeddings"],
    };
    const imageModel: PricingModel = {
      ...models[0],
      model_name: "gpt-image-1",
      supported_endpoint_types: ["image-generation"],
    };

    expect(buildModelQuickStart(embeddingModel, "https://router.flatkey.ai")).toContain("client.embeddings.create");
    expect(buildModelQuickStart(embeddingModel, "https://router.flatkey.ai")).not.toContain("chat.completions.create");
    expect(buildModelQuickStart(imageModel, "https://router.flatkey.ai")).toContain("client.images.generate");
  });

  test("builds localized model detail paths", () => {
    expect(getModelDetailPath(models[0], "en")).toBe("/models/openai-gpt-4-1");
    expect(getModelDetailPath(models[0], "zh")).toBe("/zh/models/openai-gpt-4-1");
  });
});
