import {
  generateLocalizedConsoleRedirectParams,
  redirectToLocalizedConsolePath,
} from "@/app/localized-console-redirect";

type Props = {
  params: Promise<{ locale: string }>;
};

export function generateStaticParams() {
  return generateLocalizedConsoleRedirectParams();
}

export async function GET(request: Request, props: Props) {
  return redirectToLocalizedConsolePath(request, props, "/sign-up");
}
