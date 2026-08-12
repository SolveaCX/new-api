import { APP_CONSOLE_ORIGIN } from "./origins";
import { playgroundPromptItems, type PromptItem } from "./prompt-library";

type PromptLibraryApiResponse = {
  data?: {
    items?: PromptItem[];
    total?: number;
  };
  success?: boolean;
};

const promptLibraryRevalidateSeconds = 300;
const promptLibraryFetchLimit = 700;

export async function getPlaygroundPromptItems(): Promise<PromptItem[]> {
  try {
    const url = new URL("/api/website/prompt-library", APP_CONSOLE_ORIGIN);
    url.searchParams.set("limit", String(promptLibraryFetchLimit));
    const response = await fetch(url, {
      headers: { accept: "application/json" },
      next: { revalidate: promptLibraryRevalidateSeconds },
    });
    if (!response.ok) return playgroundPromptItems;
    const payload = (await response.json()) as PromptLibraryApiResponse;
    const items = payload.data?.items;
    if (!payload.success || !Array.isArray(items) || items.length === 0) {
      return playgroundPromptItems;
    }
    return items;
  } catch {
    return playgroundPromptItems;
  }
}
