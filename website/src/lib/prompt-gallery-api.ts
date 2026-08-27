export type PromptGalleryArtifact = {
  alt: string;
  kind: string;
  url: string;
};

export type PromptGallerySource = {
  capturedAt: string;
  label: string;
  platform: string;
  url: string;
};

export type PromptGalleryItem = {
  artifact?: PromptGalleryArtifact;
  category: string;
  model: string;
  outputTranslation?: string;
  prompt: string;
  slug: string;
  source?: PromptGallerySource;
  summary?: string;
  tags: string[];
  title: string;
  updatedAt: string;
};

export type PromptGalleryFetchResult = {
  error?: string;
  items: PromptGalleryItem[];
  total: number;
};

type ApiRecord = Record<string, unknown>;

const DEFAULT_PROMPT_GALLERY_API_ORIGIN =
  "https://newapi-staging-528088078482.us-west1.run.app";

function asRecord(value: unknown): ApiRecord | null {
  return value !== null && typeof value === "object" ? (value as ApiRecord) : null;
}

function asString(value: unknown, fallback = "") {
  return typeof value === "string" ? value : fallback;
}

function asStringArray(value: unknown) {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function normalizeItem(value: unknown): PromptGalleryItem | null {
  const record = asRecord(value);
  if (!record) return null;

  const slug = asString(record.slug).trim();
  const prompt = asString(record.prompt).trim();
  if (!slug || !prompt) return null;

  const titleRecord = asRecord(record.title);
  const artifactRecord = asRecord(record.artifact);
  const sourceRecord = asRecord(record.source);
  const outputRecord = asRecord(record.output);
  const summaryRecord = asRecord(record.summary);

  const artifactUrl = asString(artifactRecord?.url).trim();
  const artifact = artifactUrl
    ? {
        alt: asString(artifactRecord?.alt, asString(titleRecord?.en, slug)),
        kind: asString(artifactRecord?.kind, "image"),
        url: artifactUrl,
      }
    : undefined;
  const sourceUrl = asString(sourceRecord?.url, asString(record.source_url)).trim();
  const source = sourceUrl || asString(sourceRecord?.label).trim()
    ? {
        capturedAt: asString(sourceRecord?.captured_at),
        label: asString(sourceRecord?.label, "—"),
        platform: asString(sourceRecord?.platform, asString(record.source_platform)),
        url: sourceUrl,
      }
    : undefined;

  return {
    artifact,
    category: asString(record.category, "image"),
    model: asString(record.model, "—"),
    outputTranslation: asString(outputRecord?.translation).trim() || undefined,
    prompt,
    slug,
    source,
    summary: asString(summaryRecord?.en).trim() || undefined,
    tags: asStringArray(record.tags),
    title: asString(titleRecord?.en, slug),
    updatedAt: asString(record.updatedAt),
  };
}

/**
 * Fetch the current prompt-gallery feed for the temporary local picker.
 * Keep the origin configurable so the page can be pointed at another
 * environment without changing the UI code.
 */
export async function getPromptGalleryItems(): Promise<PromptGalleryFetchResult> {
  const origin = process.env.PROMPT_GALLERY_API_ORIGIN?.trim() || DEFAULT_PROMPT_GALLERY_API_ORIGIN;
  try {
    const endpoint = new URL("/api/prompt-library", origin);
    endpoint.searchParams.set("p", "1");
    endpoint.searchParams.set("page_size", "100");
    endpoint.searchParams.set("category", "image");

    const response = await fetch(endpoint, {
      cache: "no-store",
      headers: { accept: "application/json" },
    });
    if (!response.ok) {
      return { error: `HTTP ${response.status}`, items: [], total: 0 };
    }

    const payload = (await response.json()) as unknown;
    const payloadRecord = asRecord(payload);
    const dataRecord = asRecord(payloadRecord?.data);
    const rawItems = Array.isArray(dataRecord?.items) ? dataRecord.items : [];
    const items = rawItems.map(normalizeItem).filter((item): item is PromptGalleryItem => Boolean(item));
    const rawTotal = dataRecord?.total;
    const total = typeof rawTotal === "number" && Number.isFinite(rawTotal) ? rawTotal : items.length;
    return { items, total };
  } catch {
    return { error: "request-failed", items: [], total: 0 };
  }
}
