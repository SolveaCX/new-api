import Image from "next/image";
import { cn } from "@/lib/utils";

type LogoSpec = {
  background: string;
  border: string;
  color: string;
  label: string;
  src?: string;
};

type HomeModelLogoProps = {
  className?: string;
  fallback?: string;
  iconKey?: string;
  imageSize?: number;
  modelName: string;
  surfaceSize?: number;
  vendor?: string;
};

const DEFAULT_LOGO: LogoSpec = {
  background: "#F8F7FF",
  border: "rgba(109, 40, 217, 0.18)",
  color: "#4C1D95",
  label: "Model",
};

const LOGO_SPECS: Array<{ match: RegExp; spec: LogoSpec }> = [
  {
    match: /openai|gpt|(^|\s)o[1-9]|dall-e|sora|codex/,
    spec: { src: "/logos/openai.svg", label: "OpenAI", background: "#F4F0FF", border: "rgba(65, 41, 145, 0.18)", color: "#412991" },
  },
  {
    match: /anthropic|claude/,
    spec: { src: "/logos/claude.svg", label: "Claude", background: "#FFF3EE", border: "rgba(217, 119, 87, 0.22)", color: "#B65332" },
  },
  {
    match: /google|gemini|imagen|veo/,
    spec: { src: "/logos/googlegemini.svg", label: "Gemini", background: "#F4F0FF", border: "rgba(142, 117, 178, 0.22)", color: "#5F4B86" },
  },
  {
    match: /deepseek|deep-seek/,
    spec: { src: "/logos/deepseek.svg", label: "DeepSeek", background: "#EEF3FF", border: "rgba(77, 107, 254, 0.22)", color: "#3654D6" },
  },
  {
    match: /kimi|moonshot/,
    spec: { src: "/logos/moonshotai.svg", label: "Kimi", background: "#F2F5F8", border: "rgba(22, 25, 30, 0.16)", color: "#16191E" },
  },
  {
    match: /qwen|alibaba|aliyun|通义/,
    spec: { src: "/logos/qwen.svg", label: "Qwen", background: "#F2F1FF", border: "rgba(97, 92, 237, 0.2)", color: "#4E49D6" },
  },
  {
    match: /minimax/,
    spec: { src: "/logos/minimax.svg", label: "MiniMax", background: "#FFF0F3", border: "rgba(242, 63, 93, 0.2)", color: "#D92345" },
  },
  {
    match: /bytedance|doubao|seedance/,
    spec: { src: "/logos/bytedance.svg", label: "ByteDance", background: "#EEF6FF", border: "rgba(60, 140, 255, 0.2)", color: "#1E70D8" },
  },
  {
    match: /meta|llama/,
    spec: { src: "/logos/meta.svg", label: "Meta", background: "#EFF6FF", border: "rgba(4, 103, 223, 0.2)", color: "#0467DF" },
  },
  {
    match: /mistral/,
    spec: { src: "/logos/mistralai.svg", label: "Mistral", background: "#FFF7ED", border: "rgba(217, 119, 6, 0.2)", color: "#B45309" },
  },
  {
    match: /perplexity/,
    spec: { src: "/logos/perplexity.svg", label: "Perplexity", background: "#EFFBFC", border: "rgba(32, 128, 141, 0.2)", color: "#17717C" },
  },
  {
    match: /ollama/,
    spec: { src: "/logos/ollama.svg", label: "Ollama", background: "#F4F4FA", border: "rgba(23, 22, 58, 0.18)", color: "#17163A" },
  },
  {
    match: /nvidia/,
    spec: { src: "/logos/nvidia.svg", label: "NVIDIA", background: "#F1FAE8", border: "rgba(118, 185, 0, 0.22)", color: "#4C7D00" },
  },
  {
    match: /baidu|ernie|文心/,
    spec: { src: "/logos/baidu.svg", label: "Baidu", background: "#EEF3FF", border: "rgba(36, 93, 255, 0.2)", color: "#245DFF" },
  },
  {
    match: /huggingface|hugging-face/,
    spec: { src: "/logos/huggingface.svg", label: "Hugging Face", background: "#FFF8E7", border: "rgba(255, 188, 5, 0.28)", color: "#8A5A00" },
  },
];

export function resolveHomeModelLogo(input: { iconKey?: string; modelName: string; vendor?: string }): LogoSpec {
  const identity = `${input.modelName} ${input.vendor ?? ""}`
    .toLowerCase()
    .replace(/[_./:]+/g, " ");
  const fromIdentity = LOGO_SPECS.find((entry) => entry.match.test(identity))?.spec;
  if (fromIdentity) return fromIdentity;

  const iconKey = (input.iconKey ?? "").toLowerCase().replace(/[_./:]+/g, " ");
  return LOGO_SPECS.find((entry) => entry.match.test(iconKey))?.spec ?? DEFAULT_LOGO;
}

export function HomeModelLogo(props: HomeModelLogoProps) {
  const spec = resolveHomeModelLogo(props);
  const surfaceSize = props.surfaceSize ?? 32;
  const imageSize = props.imageSize ?? Math.round(surfaceSize * 0.64);
  const fallback = (props.fallback || props.modelName.charAt(0) || "?").slice(0, 2).toUpperCase();

  return (
    <span
      aria-label={spec.label}
      title={spec.label}
      className={cn("inline-grid shrink-0 place-items-center overflow-hidden border shadow-[0_10px_24px_-18px_rgba(16,16,20,0.45)]", props.className)}
      style={{
        width: surfaceSize,
        height: surfaceSize,
        borderColor: spec.border,
        borderRadius: Math.max(8, Math.round(surfaceSize * 0.26)),
        background: spec.background,
        color: spec.color,
      }}
    >
      {spec.src ? (
        <Image
          src={spec.src}
          alt=""
          width={imageSize}
          height={imageSize}
          className="block object-contain object-center"
          style={{ width: imageSize, height: imageSize }}
        />
      ) : (
        <span className="font-sans text-[11px] font-black leading-none">{fallback}</span>
      )}
    </span>
  );
}
