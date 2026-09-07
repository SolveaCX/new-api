import { permanentRedirect } from "next/navigation";
import { PROMPT_VIDEO_PATH } from "@/lib/cli-landing";

type Props = { params: Promise<{ slug: string }> };

export default async function Page(props: Props) {
  const params = await props.params;
  permanentRedirect(`${PROMPT_VIDEO_PATH}/${params.slug}`);
}
