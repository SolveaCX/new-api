import { permanentRedirect } from "next/navigation";
import { PROMPT_VIDEO_PATH } from "@/lib/cli-landing";
import { isLocale, localizePath } from "@/lib/locales";

type Props = { params: Promise<{ locale: string; slug: string }> };

export default async function Page(props: Props) {
  const params = await props.params;
  const target = `${PROMPT_VIDEO_PATH}/${params.slug}`;
  if (!isLocale(params.locale) || params.locale === "en") permanentRedirect(target);
  permanentRedirect(localizePath(target, params.locale));
}
