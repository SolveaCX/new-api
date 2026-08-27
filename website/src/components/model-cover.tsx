import { modelCoverAssetUrl } from "@/lib/model-covers";
import { cn } from "@/lib/utils";

type Props = {
  modelName: string;
  vendor?: string;
  className?: string;
  compact?: boolean;
};

/** Shared Flatkey artwork for the Related model APIs cards. */
export function ModelCover({ modelName, className, compact = false }: Props) {
  return (
    <span
      className={cn(
        "relative isolate block overflow-hidden rounded-xl bg-[#f7f3ff]",
        compact ? "h-14 w-14 shrink-0" : "aspect-square w-full",
        className
      )}
      aria-label={`${modelName} — flatkey.ai`}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={modelCoverAssetUrl()} alt="" className="absolute inset-0 size-full object-cover" loading="lazy" />
      <img
        src="/flatkey-lockup-light.svg"
        alt="flatkey.ai"
        className={cn(
          "pointer-events-none absolute z-10 h-auto object-contain object-left-top",
          compact ? "top-2 left-2 w-10" : "top-[7%] left-[7%] w-[30%] max-w-[180px] min-w-[72px]"
        )}
      />
    </span>
  );
}
