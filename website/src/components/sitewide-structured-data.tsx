import type { Locale } from "@/lib/locales";
import { buildBasePageSchema, stringifyJsonLd } from "@/lib/schema";

type Props = {
  locale: Locale;
  pathname: string;
};

export function SitewideStructuredData(props: Props) {
  return (
    <script
      data-sitewide-schema="true"
      type="application/ld+json"
      dangerouslySetInnerHTML={{
        __html: stringifyJsonLd(
          buildBasePageSchema({
            locale: props.locale,
            pathname: props.pathname,
          })
        ),
      }}
    />
  );
}
