import { notFound } from "next/navigation";
import { NextResponse } from "next/server";
import { isLocale, LOCALES, type Locale } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type LocalizedRouteProps = {
  params: Promise<{ locale: string }>;
};

export function generateLocalizedConsoleRedirectParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function redirectToLocalizedConsolePath(
  request: Request,
  props: LocalizedRouteProps,
  pathname: string
) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();

  return NextResponse.redirect(
    consoleUrl(pathname, localizedConsoleSearch(request, params.locale)),
    301
  );
}

export function localizedConsoleSearch(request: Request, locale: Locale): string {
  const searchParams = new URL(request.url).searchParams;
  searchParams.set("lng", locale);
  return searchParams.toString();
}
