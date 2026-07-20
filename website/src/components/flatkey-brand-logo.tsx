import Image from "next/image";
import { cn } from "@/lib/utils";

// Single brand standard (matches the official website / website-static lockup):
// the gradient "flatkey-mark" tile + a single-color "flatkey" wordmark (no ".ai")
// in Public Sans Bold, lowercase. Color is #1E1B4B on light, near-white on dark.
const FLATKEY_LOCKUP_LIGHT = "/flatkey-lockup-light.svg";
const FLATKEY_LOCKUP_DARK = "/flatkey-lockup-dark.svg";
const FLATKEY_MARK = "/flatkey-mark.svg";

const WORDMARK_FONT_FAMILY = "'Public Sans', Inter, 'SF Pro Display', Arial, sans-serif";

type FlatkeyBrandLogoProps = {
  alt?: string;
  className?: string;
  imageClassName?: string;
  variant?: "lockup" | "full";
};

export function FlatkeyBrandLogo({
  alt = "flatkey",
  className,
  imageClassName,
  variant = "lockup",
}: FlatkeyBrandLogoProps) {
  const imageClass = cn("h-full w-full object-contain", imageClassName);

  if (variant === "full") {
    return (
      <span className={cn("relative block overflow-hidden", className)}>
        <Image
          src={FLATKEY_LOCKUP_LIGHT}
          alt={alt}
          width={250}
          height={64}
          className={cn(imageClass, "block dark:hidden")}
        />
        <Image
          src={FLATKEY_LOCKUP_DARK}
          alt={alt}
          width={250}
          height={64}
          className={cn(imageClass, "hidden dark:block")}
        />
      </span>
    );
  }

  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <Image src={FLATKEY_MARK} alt="" aria-hidden width={32} height={32} className="h-8 w-8 shrink-0" />
      <span
        className="text-[20px] leading-none font-bold text-[#1E1B4B] dark:text-slate-50"
        style={{ fontFamily: WORDMARK_FONT_FAMILY, letterSpacing: "-0.04em" }}
      >
        flatkey
      </span>
    </span>
  );
}
