import type { Locale } from "@/lib/locales";

const DEFAULT_TALLY_FORM_ID = "1A6gM4";
const TALLY_FORM_IDS: Partial<Record<Locale, string>> = {
  zh: "BzRylN",
  ja: "vG9Z1Q",
};
const TALLY_EMBED_OPTIONS =
  "alignLeft=1&hideTitle=1&transparentBackground=1&dynamicHeight=1";

export function getTallyFormId(locale: Locale): string {
  return TALLY_FORM_IDS[locale] ?? DEFAULT_TALLY_FORM_ID;
}

export function getTallyFormUrl(locale: Locale): string {
  return `https://tally.so/r/${getTallyFormId(locale)}`;
}

export function getTallyEmbedUrl(locale: Locale): string {
  return `https://tally.so/embed/${getTallyFormId(locale)}?${TALLY_EMBED_OPTIONS}`;
}
