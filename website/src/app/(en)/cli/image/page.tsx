import { permanentRedirect } from "next/navigation";
import { PROMPT_IMAGE_PATH } from "@/lib/cli-landing";

export default function Page() {
  permanentRedirect(PROMPT_IMAGE_PATH);
}
