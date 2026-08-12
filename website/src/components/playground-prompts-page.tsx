import { PlaygroundPromptsExplorer } from "@/components/playground-prompts-explorer";
import { type Locale } from "@/lib/locales";
import { getPlaygroundPromptItems } from "@/lib/playground-prompts";
import { promptLibraryCopy } from "@/lib/prompt-library";
import { OnlineStaticShell } from "./online-static-shell";

export const PLAYGROUND_PROMPTS_PATH = "/playground";

export function getPlaygroundPromptsMetadata(locale: Locale) {
  const copy = promptLibraryCopy[locale] ?? promptLibraryCopy.en;
  return {
    title: copy.metaTitle,
    description: copy.metaDescription,
    pathname: PLAYGROUND_PROMPTS_PATH,
  };
}

export async function PlaygroundPromptsPage(props: { locale: Locale }) {
  const items = await getPlaygroundPromptItems();

  return (
    <OnlineStaticShell locale={props.locale} pathname={PLAYGROUND_PROMPTS_PATH}>
      <PlaygroundPromptsExplorer items={items} locale={props.locale} />
    </OnlineStaticShell>
  );
}
