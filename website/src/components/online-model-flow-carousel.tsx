"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

export type ModelFlowCarouselRow = {
  discount: string;
  flatkey: string;
  href: string;
  model: string;
  official: string;
  vendor: string;
};

export type ModelFlowCarouselItem = {
  api: string;
  copy: string;
  cta: string;
  href: string;
  kind: "text" | "image" | "video" | "audio";
  models: string[];
  rows: ModelFlowCarouselRow[];
  title: string;
};

export type ModelFlowCarouselLabels = {
  discount: string;
  flatkey: string;
  model: string;
  official: string;
  provider: string;
};

type Props = {
  allModelsHref: string;
  allModelsLabel: string;
  ariaLabel: string;
  directoryFallback: string;
  items: ModelFlowCarouselItem[];
  labels: ModelFlowCarouselLabels;
  priceTitle: string;
};

const ROTATE_MS = 5200;

export function OnlineModelFlowCarousel(props: Props) {
  const [activeIndex, setActiveIndex] = useState(0);
  const active = props.items[activeIndex] ?? props.items[0];

  useEffect(() => {
    if (props.items.length <= 1) return;
    const timer = window.setTimeout(() => {
      setActiveIndex((current) => (current + 1) % props.items.length);
    }, ROTATE_MS);
    return () => window.clearTimeout(timer);
  }, [activeIndex, props.items.length]);

  if (!active) return null;

  return (
    <div className="modelFlowWorkbench">
      <div className="modelFlowTabs" aria-label={props.ariaLabel}>
        {props.items.map((item, index) => (
          <button
            aria-current={index === activeIndex}
            className={`modelFlowTab modelFlowTab-${item.kind}${index === activeIndex ? " is-active" : ""}`}
            key={item.kind}
            onClick={() => setActiveIndex(index)}
            type="button"
          >
            <span>{item.api}</span>
            <b>{item.title}</b>
          </button>
        ))}
      </div>
      <div className="modelFlowTables">
        <article className={`modelFlowTableCard modelFlowTableCard-${active.kind}`} id={`model-flow-${active.kind}`} key={active.kind}>
          <div className="modelFlowTableHead">
            <div>
              <span>{active.api}</span>
              <h3>{active.title}</h3>
            </div>
            <Link href={active.href}>{active.cta}</Link>
          </div>
          <p>{active.copy}</p>
          <div className="modelFlowMiniModels">
            {(active.models.length > 0 ? active.models : [props.directoryFallback]).map((model) => (
              <i key={model}>{model}</i>
            ))}
          </div>
          <div className="modelFlowTable" role="table" aria-label={`${active.title} ${props.priceTitle}`}>
            <div className="modelFlowTableRow modelFlowTableHeader" role="row">
              <span role="columnheader">{props.labels.model}</span>
              <span role="columnheader">{props.labels.provider}</span>
              <span role="columnheader">{props.labels.flatkey}</span>
              <span role="columnheader">{props.labels.official}</span>
              <span role="columnheader">{props.labels.discount}</span>
            </div>
            {active.rows.map((row) => (
              <Link className="modelFlowTableRow" href={row.href} key={row.model} role="row">
                <strong role="cell">{row.model}</strong>
                <span role="cell">{row.vendor}</span>
                <b role="cell">{row.flatkey}</b>
                <s role="cell">{row.official}</s>
                <i role="cell">{row.discount}</i>
              </Link>
            ))}
          </div>
        </article>
      </div>
      <Link className="modelFlowAllModels" href={props.allModelsHref}>
        {props.allModelsLabel}
      </Link>
    </div>
  );
}
