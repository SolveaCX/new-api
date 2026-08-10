import Image from "next/image";
import { cn } from "@/lib/utils";

const FLATKEY_MARK = "/flatkey-mark.svg";

const WORDMARK_FONT_FAMILY = "'Space Grotesk', 'Flatkey Line Seed Sans', 'Public Sans', Inter, -apple-system, sans-serif";

type FlatkeyBrandLogoProps = {
  alt?: string;
  className?: string;
};

export function FlatkeyBrandLogo({
  alt = "flatkey",
  className,
}: FlatkeyBrandLogoProps) {
  return (
    <span
      data-flatkey-brand="lockup"
      aria-label={alt}
      className={cn(
        "group/flatkey-brand inline-flex shrink-0 items-center gap-[9px] min-[901px]:gap-2 min-[1481px]:gap-[9px]",
        className
      )}
    >
      <Image
        src={FLATKEY_MARK}
        alt=""
        aria-hidden
        width={40}
        height={40}
        className="h-[38px] w-[38px] shrink-0 min-[901px]:h-9 min-[901px]:w-9 min-[1481px]:h-10 min-[1481px]:w-10"
      />
      <span
        data-flatkey-wordmark="true"
        className="relative inline-flex text-[30px] leading-none font-bold tracking-normal text-[#0B0B0F] after:absolute after:right-0 after:-bottom-1 after:left-0 after:h-2 after:rounded-full after:bg-[#C8A8FF]/55 after:content-[''] min-[901px]:text-[28px] min-[1481px]:text-[32px] dark:text-[#F5F5F2] dark:after:bg-[#7C3AED]/45 max-[420px]:hidden"
        style={{ fontFamily: WORDMARK_FONT_FAMILY }}
      >
        <span className="relative z-10">flatkey</span>
      </span>
    </span>
  );
}
