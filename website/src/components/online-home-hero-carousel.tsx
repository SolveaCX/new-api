"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { type Locale, localizePath } from "@/lib/locales";

export type HeroMode = {
  copy: string;
  cta: string;
  href: string;
  kind: "all" | "image" | "video" | "text" | "audio";
  kicker: string;
  image: string;
  metric: string;
  modelName: string;
  modelVendor: string;
  mode: string;
  subline: string;
  thumb: string;
  title: string;
};

type Props = {
  copy: {
    aria: string;
    eyebrow: string;
    switchAria: string;
  };
  heroModes: HeroMode[];
  locale: Locale;
};

const ROTATE_MS = 5600;

export function OnlineHomeHeroCarousel(props: Props) {
  const [activeIndex, setActiveIndex] = useState(0);
  const active = props.heroModes[activeIndex] ?? props.heroModes[0];

  useEffect(() => {
    if (props.heroModes.length <= 1) return;
    const timer = window.setTimeout(() => {
      setActiveIndex((current) => (current + 1) % props.heroModes.length);
    }, ROTATE_MS);
    return () => window.clearTimeout(timer);
  }, [activeIndex, props.heroModes.length]);

  if (!active) return null;

  return (
    <header className="hero heroUnified" data-active-mode={active.mode}>
      <div className="heroStageCarousel" aria-label={props.copy.aria}>
        {props.heroModes.map((item, index) => (
          <div className={`heroStageSlide${index === activeIndex ? " is-active" : ""}`} aria-hidden={index !== activeIndex} key={item.mode}>
            <img className="heroStageImage" src={item.image} alt="" />
            <div className="heroStageShade" />
          </div>
        ))}
      </div>

      <div className="heroGrid">
        <div className="heroCopy" key={active.mode}>
          <span className="eyebrow">{props.copy.eyebrow}</span>
          <h1 className="heroTitle">
            {active.title}
            <span>{active.subline}</span>
          </h1>
          <p className="heroSub">{active.copy}</p>
          <div className="heroCtas">
            <Link className="btn big heroPrimary" href={localizePath(active.href, props.locale)}>
              {active.cta}
            </Link>
          </div>
        </div>

        <aside className="heroModePanel" aria-live="polite" key={`${active.mode}-panel`}>
          <div className="heroModePanelTop">
            <span>{active.kicker}</span>
            <b>{active.metric}</b>
          </div>
          <strong>{active.modelName}</strong>
          <p>{active.copy}</p>
          <div className="heroModePanelMeta">
            <span>{active.modelVendor}</span>
            <span>{active.mode}</span>
          </div>
        </aside>

        <div className="heroStageList" aria-label={props.copy.switchAria}>
          {props.heroModes.map((item, index) => (
            <button
              aria-current={index === activeIndex}
              className={`heroStageNav${index === activeIndex ? " is-active" : ""}`}
              key={item.mode}
              onClick={() => setActiveIndex(index)}
              type="button"
            >
              <img src={item.thumb} alt="" />
              <span>{String(index + 1).padStart(2, "0")}</span>
              <b>{item.modelName}</b>
              <small>{item.modelVendor}</small>
            </button>
          ))}
          <div className="heroStagePager" aria-hidden="true">
            {props.heroModes.map((item, index) => (
              <i className={index === activeIndex ? "is-active" : undefined} key={item.mode} />
            ))}
          </div>
        </div>
      </div>
    </header>
  );
}
