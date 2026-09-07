import { permanentRedirect } from "next/navigation";
import { PROMPT_VIDEO_PATH } from "@/lib/cli-landing";
import { isLocale, localizePath } from "@/lib/locales";

type Props = { params: Promise<{ locale: string }> };

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") permanentRedirect(PROMPT_VIDEO_PATH);
  permanentRedirect(localizePath(PROMPT_VIDEO_PATH, params.locale));
}
