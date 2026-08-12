"use client";

import { Check, Globe2 } from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { buildLanguagePreferenceCookie } from "@/lib/language-routing";
import { LOCALE_LABELS, LOCALES, type Locale, localeLanguageTag, localizePath, stripLocale } from "@/lib/locales";

type Props = {
  locale: Locale;
  pathname: string;
  variant?: "dropdown" | "panel";
};

const languagePanelLabels: Record<Locale, string> = {
  en: "Language",
  zh: "语言",
  es: "Idioma",
  fr: "Langue",
  pt: "Idioma",
  ru: "Язык",
  ja: "言語",
  vi: "Ngôn ngữ",
  de: "Sprache",
  id: "Bahasa",
};

function persistLanguagePreference(locale: Locale) {
  document.cookie = buildLanguagePreferenceCookie(locale);
}

export function OnlineLanguageSelect(props: Props) {
  const [open, setOpen] = useState(false);
  const menuId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const strippedPath = stripLocale(props.pathname);
  const links = useMemo(
    () =>
      LOCALES.map((locale) => ({
        code: locale,
        href: localizePath(strippedPath, locale),
        label: LOCALE_LABELS[locale],
      })),
    [strippedPath]
  );

  useEffect(() => {
    if (!open) return;

    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const handleLanguageClick = (locale: Locale) => {
    persistLanguagePreference(locale);
    setOpen(false);
  };

  if (props.variant === "panel") {
    return (
      <details className="mobile-nav-section langmenu-sheet">
        <summary>
          <span className="langmenu-sheet-title">{languagePanelLabels[props.locale]}</span>
        </summary>
        <nav className="langmenu-sheet-panel" aria-label="Change language">
          {links.map((lang) => (
            <a
              key={lang.code}
              href={lang.href}
              hrefLang={localeLanguageTag(lang.code)}
              lang={localeLanguageTag(lang.code)}
              aria-current={props.locale === lang.code ? "page" : undefined}
              onClick={() => handleLanguageClick(lang.code)}
            >
              <span>{lang.label}</span>
              <Check aria-hidden="true" className={props.locale === lang.code ? "langmenu-check" : "langmenu-check is-hidden"} />
            </a>
          ))}
        </nav>
      </details>
    );
  }

  return (
    <div ref={rootRef} className="langmenu">
      <button
        type="button"
        className="langmenu-trigger"
        aria-label="Change language"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        onClick={() => setOpen((value) => !value)}
      >
        <Globe2 aria-hidden="true" />
      </button>
      <nav id={menuId} className={`langmenu-panel${open ? " is-open" : ""}`} aria-label="Change language">
        {links.map((lang) => (
          <a
            key={lang.code}
            href={lang.href}
            hrefLang={localeLanguageTag(lang.code)}
            lang={localeLanguageTag(lang.code)}
            aria-current={props.locale === lang.code ? "page" : undefined}
            onClick={() => handleLanguageClick(lang.code)}
          >
            <span>{lang.label}</span>
            <Check aria-hidden="true" className={props.locale === lang.code ? "langmenu-check" : "langmenu-check is-hidden"} />
          </a>
        ))}
      </nav>
    </div>
  );
}
