import manifest from "./model-media-generation-manifest.json";

const GENERATOR_LABELS: Record<string, string> = {
  "gpt-image-2": "Image 2",
  "seedance-2.0": "Seedance 2.0",
};

/** Credit the actual generator when showcase artwork uses a different model. */
export function getShowcaseGeneratorLabel(asset?: string): string | undefined {
  if (!asset) return undefined;
  const entry = manifest.entries.find((item) => item.poster === asset || ("video" in item && item.video === asset));
  return entry && "generationModel" in entry && typeof entry.generationModel === "string"
    ? GENERATOR_LABELS[entry.generationModel]
    : undefined;
}
