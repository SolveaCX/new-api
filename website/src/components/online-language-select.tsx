"use client";

import { Globe2 } from "lucide-react";
import { buildLanguagePreferenceCookie } from "@/lib/language-routing";
import { LOCALE_LABELS, LOCALES, type Locale, localizePath, stripLocale } from "@/lib/locales";

type Props = {
  locale: Locale;
  pathname: string;
};

const LOCALE_BADGES: Record<Locale, string> = {
  de: "DE",
  en: "EN",
  es: "ES",
  fr: "FR",
  id: "ID",
  ja: "JP",
  pt: "PT",
  ru: "RU",
  vi: "VI",
  zh: "中",
};

export function OnlineLanguageSelect(props: Props) {
  return (
    <details className="langIconSelect">
      <summary aria-label="Change language" title={LOCALE_LABELS[props.locale]}>
        <Globe2 aria-hidden="true" />
        <span className="langCurrent">{LOCALE_BADGES[props.locale]}</span>
      </summary>
      <div className="langMenu">
        {LOCALES.map((locale) => (
          <a
            aria-current={locale === props.locale ? "true" : undefined}
            href={localizePath(stripLocale(props.pathname), locale)}
            key={locale}
            onClick={() => {
              document.cookie = buildLanguagePreferenceCookie(locale as Locale);
            }}
          >
            {LOCALE_LABELS[locale]}
          </a>
        ))}
      </div>
    </details>
  );
}
