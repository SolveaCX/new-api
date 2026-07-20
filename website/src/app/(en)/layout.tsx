import Script from "next/script";
import { ATTRIBUTION_COOKIE_SCRIPT, RootDocument, rootMetadata } from "@/components/root-document";
import { DEFAULT_LOCALE } from "@/lib/locales";
import { getDocsUrl } from "@/lib/public-site-settings";
import "../globals.css";

export const metadata = rootMetadata;

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const docsUrl = await getDocsUrl();

  return (
    <RootDocument
      docsUrl={docsUrl}
      lang={DEFAULT_LOCALE}
      bodyStart={
        <Script id="flatkey-attribution-cookie" strategy="beforeInteractive">
          {ATTRIBUTION_COOKIE_SCRIPT}
        </Script>
      }
    >
      {children}
    </RootDocument>
  );
}
