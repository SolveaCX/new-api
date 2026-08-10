import Image from "next/image";
import type { CSSProperties } from "react";
import { cn } from "@/lib/utils";

const FLATKEY_BRAND_LETTERS = ["f", "l", "a", "t", "k", "e", "y"] as const;

type FlatkeyMascotWordmarkProps = {
  ariaLabel?: string;
  className?: string;
  decorative?: boolean;
  priority?: boolean;
  size?: "display" | "lockup";
  withDots?: boolean;
};

export function FlatkeyMascotWordmark({
  ariaLabel = "flatkey",
  className,
  decorative = false,
  priority = false,
  size = "display",
  withDots = false,
}: FlatkeyMascotWordmarkProps) {
  return (
    <span
      aria-hidden={decorative || undefined}
      aria-label={decorative ? undefined : ariaLabel}
      className={cn("fk-brand-wordmark", `fk-brand-wordmark-${size}`, className)}
      role={decorative ? undefined : "img"}
    >
      {FLATKEY_BRAND_LETTERS.map((letter, index) => (
        <span key={`${letter}-${index}`} className={`fk-brand-letter fk-brand-letter-${letter}`} style={{ "--fk-letter-index": index } as CSSProperties}>
          <Image
            src={`/assets/mascots/flatkey-brand-letter-${letter}.webp`}
            alt=""
            width={760}
            height={760}
            priority={priority}
            className="fk-brand-letter-img"
          />
        </span>
      ))}
      {withDots ? (
        <span className="fk-brand-wordmark-dots">
          {[0, 1, 2].map((dot) => (
            <span key={dot} className="fk-brand-wordmark-dot" style={{ "--fk-dot-index": dot } as CSSProperties} />
          ))}
        </span>
      ) : null}
    </span>
  );
}
