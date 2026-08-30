"use client";

type ModelStripItem = readonly [logo: string, name: string, description: string];

type ModelStripCarouselProps = {
  items: readonly ModelStripItem[];
};

export function ModelStripCarousel({ items }: ModelStripCarouselProps) {
  return (
    <div className="modelStripCarousel">
      <div className="modelStripViewport">
        <div className="modelStripTrack">
          {[false, true].map((isClone) => (
            <div
              aria-hidden={isClone ? "true" : undefined}
              className={`modelStripSet${isClone ? " modelStripSetClone" : ""}`}
              key={isClone ? "clone" : "primary"}
            >
              {items.map(([logo, name, description]) => (
                <div className="modelStripItem" key={`${isClone ? "clone-" : ""}${name}`}>
                  <img src={`/assets/logos/${logo}`} alt="" aria-hidden="true" />
                  <div>
                    <strong>{name}</strong>
                    <span>{description}</span>
                  </div>
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
