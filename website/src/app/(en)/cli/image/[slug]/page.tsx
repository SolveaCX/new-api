import { permanentRedirect } from "next/navigation";
import { PROMPT_IMAGE_PATH } from "@/lib/cli-landing";

type Props = { params: Promise<{ slug: string }> };

export default async function Page(props: Props) {
  const params = await props.params;
  permanentRedirect(`${PROMPT_IMAGE_PATH}/${params.slug}`);
}
