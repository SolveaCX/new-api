"use client";

import Link from "next/link";
import { ModelLogo } from "@/components/pricing-model-browser";
import type { HomeCopy } from "@/lib/home-copy";
import type { HomePricedModel } from "@/lib/home-models";
import { modelIconKey } from "@/lib/home-models";
import { localizePath, type Locale, withIdFallback } from "@/lib/locales";
import { modelPublicPath } from "@/lib/model-public";

type Props = {
  copy: HomeCopy["table"];
  rows: HomePricedModel[];
  locale?: Locale;
};

type FeaturedModelRow = {
  discount: "none" | "nine" | "six" | "three";
  flatkeyPrice: string;
  model: string;
  officialPrice: string;
  type: "text" | "video";
};

const FEATURED_MODEL_ROWS: FeaturedModelRow[] = [
  { model: "MiniMax-H3", type: "video", flatkeyPrice: "$0.08 / req", officialPrice: "$0.08 / req", discount: "none" },
  { model: "Seedance2.0", type: "video", flatkeyPrice: "$6.3 / billing unit", officialPrice: "$7 / billing unit", discount: "nine" },
  { model: "kimi-k3", type: "text", flatkeyPrice: "in $1.8 / out $9 / 1M tokens", officialPrice: "in $3 / out $15 / 1M tokens", discount: "six" },
  { model: "gpt-5.6-sol", type: "text", flatkeyPrice: "in $1.5 / out $9 / 1M tokens", officialPrice: "in $5 / out $30 / 1M tokens", discount: "three" },
  { model: "gpt-4o-mini", type: "text", flatkeyPrice: "in $0.135 / out $0.54 / 1M tokens", officialPrice: "in $0.15 / out $0.6 / 1M tokens", discount: "nine" },
  { model: "claude-opus-5", type: "text", flatkeyPrice: "in $4.5 / out $22.5 / 1M tokens", officialPrice: "in $5 / out $25 / 1M tokens", discount: "nine" },
  { model: "deepseek-v4-flash", type: "text", flatkeyPrice: "in $0.084 / out $0.168 / 1M tokens", officialPrice: "in $0.14 / out $0.28 / 1M tokens", discount: "six" },
  { model: "claude-opus-4-6", type: "text", flatkeyPrice: "in $4.5 / out $22.5 / 1M tokens", officialPrice: "in $5 / out $25 / 1M tokens", discount: "nine" },
  { model: "gpt-5.6-luna", type: "text", flatkeyPrice: "in $0.06 / out $0.36 / 1M tokens", officialPrice: "in $0.2 / out $1.2 / 1M tokens", discount: "three" },
  { model: "claude-sonnet-5", type: "text", flatkeyPrice: "in $1.8 / out $9 / 1M tokens", officialPrice: "in $2 / out $10 / 1M tokens", discount: "nine" },
  { model: "claude-opus-4-8", type: "text", flatkeyPrice: "in $4.5 / out $22.5 / 1M tokens", officialPrice: "in $5 / out $25 / 1M tokens", discount: "nine" },
  { model: "gemini-3.6-flash", type: "text", flatkeyPrice: "in $1.35 / out $6.75 / 1M tokens", officialPrice: "in $1.5 / out $7.5 / 1M tokens", discount: "nine" },
  { model: "gpt-5.4", type: "text", flatkeyPrice: "in $0.75 / out $4.5 / 1M tokens", officialPrice: "in $2.5 / out $15 / 1M tokens", discount: "three" },
  { model: "glm-5.2", type: "text", flatkeyPrice: "in $0.84 / out $2.64 / 1M tokens", officialPrice: "in $1.4 / out $4.4 / 1M tokens", discount: "six" },
];

const TABLE_COPY = withIdFallback({
  en: {
    model: "model",
    type: "type",
    flatkeyPrice: "flatkey price",
    officialPrice: "official price",
    discount: "discount",
    discounts: { none: "No discount", nine: "10% off", six: "40% off", three: "70% off" },
  },
  zh: {
    model: "model",
    type: "type",
    flatkeyPrice: "flatkey价格",
    officialPrice: "官方价格",
    discount: "折扣",
    discounts: { none: "无折扣", nine: "9折", six: "6折", three: "3折" },
  },
  es: {
    model: "modelo",
    type: "tipo",
    flatkeyPrice: "precio flatkey",
    officialPrice: "precio oficial",
    discount: "descuento",
    discounts: { none: "Sin descuento", nine: "10% desc.", six: "40% desc.", three: "70% desc." },
  },
  fr: {
    model: "modèle",
    type: "type",
    flatkeyPrice: "prix flatkey",
    officialPrice: "prix officiel",
    discount: "remise",
    discounts: { none: "Sans remise", nine: "-10 %", six: "-40 %", three: "-70 %" },
  },
  pt: {
    model: "modelo",
    type: "tipo",
    flatkeyPrice: "preço flatkey",
    officialPrice: "preço oficial",
    discount: "desconto",
    discounts: { none: "Sem desconto", nine: "10% off", six: "40% off", three: "70% off" },
  },
  ru: {
    model: "модель",
    type: "тип",
    flatkeyPrice: "цена flatkey",
    officialPrice: "официальная цена",
    discount: "скидка",
    discounts: { none: "Без скидки", nine: "-10%", six: "-40%", three: "-70%" },
  },
  ja: {
    model: "model",
    type: "type",
    flatkeyPrice: "flatkey価格",
    officialPrice: "公式価格",
    discount: "割引",
    discounts: { none: "割引なし", nine: "1割引", six: "4割引", three: "7割引" },
  },
  vi: {
    model: "model",
    type: "type",
    flatkeyPrice: "giá flatkey",
    officialPrice: "giá chính thức",
    discount: "ưu đãi",
    discounts: { none: "Không giảm", nine: "Giảm 10%", six: "Giảm 40%", three: "Giảm 70%" },
  },
  de: {
    model: "Modell",
    type: "Typ",
    flatkeyPrice: "Flatkey-Preis",
    officialPrice: "Offizieller Preis",
    discount: "Rabatt",
    discounts: { none: "Kein Rabatt", nine: "10% Rabatt", six: "40% Rabatt", three: "70% Rabatt" },
  },
});

export function ModelsDirectoryTable(props: Props) {
  const locale = props.locale ?? "en";
  const copy = TABLE_COPY[locale];

  return (
    <div className="overflow-x-auto rounded-[1.25rem] border border-[#101014]/10 bg-white shadow-sm dark:border-white/12 dark:bg-[#111116]/88">
      <table className="w-full min-w-[980px] border-collapse text-left text-[15px] text-[#2F3033] dark:text-white/82">
        <thead>
          <tr className="border-b border-[#101014]/12 text-[17px] font-black text-[#2F3033] dark:border-white/12 dark:text-white">
            <th className="px-5 py-4 font-black">{copy.model}</th>
            <th className="px-5 py-4 font-black">{copy.type}</th>
            <th className="px-5 py-4 font-black">{copy.flatkeyPrice}</th>
            <th className="px-5 py-4 font-black">{copy.officialPrice}</th>
            <th className="px-5 py-4 font-black">{copy.discount}</th>
          </tr>
        </thead>
        <tbody>
          {FEATURED_MODEL_ROWS.map((row) => (
            <tr key={row.model} className="border-b border-[#101014]/8 last:border-b-0 dark:border-white/10">
              <td className="px-5 py-4 text-[17px] font-semibold whitespace-nowrap text-[#2F3033] dark:text-white">
                <Link href={localizePath(modelPublicPath(row.model), locale)} className="inline-flex items-center gap-2.5 hover:text-[#5852FF]">
                  <span className="flex size-7 shrink-0 items-center justify-center rounded-lg border border-[#101014]/12 bg-[#F7F4EC] dark:border-white/14 dark:bg-white/8">
                    <ModelLogo iconKey={modelIconKey(row.model, row.type)} fallback={row.model.charAt(0).toUpperCase()} size={17} />
                  </span>
                  <span>{row.model}</span>
                </Link>
              </td>
              <td className="px-5 py-4 text-[17px] whitespace-nowrap">{row.type}</td>
              <td className="px-5 py-4 text-[17px] whitespace-nowrap">{row.flatkeyPrice}</td>
              <td className="px-5 py-4 text-[17px] whitespace-nowrap">{row.officialPrice}</td>
              <td className="px-5 py-4 text-[17px] font-semibold whitespace-nowrap">{copy.discounts[row.discount]}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
