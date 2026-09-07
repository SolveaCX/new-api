import { permanentRedirect } from "next/navigation";
import { PROMPT_VIDEO_PATH } from "@/lib/cli-landing";

export default function Page() {
  permanentRedirect(PROMPT_VIDEO_PATH);
}
