import type { Locale } from "@/lib/locales";

const DISCLOSURE: Record<Locale, [string, string]> = {
  en: ["View full prompt", "Collapse prompt"],
  zh: ["展开完整提示词", "收起提示词"],
  es: ["Ver el prompt completo", "Contraer el prompt"],
  fr: ["Voir le prompt complet", "Réduire le prompt"],
  pt: ["Ver o prompt completo", "Recolher o prompt"],
  ru: ["Показать весь промпт", "Свернуть промпт"],
  ja: ["プロンプト全文を表示", "プロンプトを折りたたむ"],
  vi: ["Xem toàn bộ lời nhắc", "Thu gọn lời nhắc"],
  de: ["Vollständigen Prompt anzeigen", "Prompt einklappen"],
  id: ["Lihat prompt lengkap", "Tutup prompt"],
};

/** Formatting is presentation-only: copying and request handoff use the source. */
export function MediaPromptDisplay({ prompt, locale }: { prompt: string; locale: Locale }) {
  const blocks = prompt.split(/\n\s*\n/).filter(Boolean);
  const summary = blocks[0].replace(/^(?:Creative direction|Scene):\s*/, "");
  const [expand, collapse] = DISCLOSURE[locale];

  return (
    <div className="media-prompt">
      <p className="media-prompt-excerpt" lang="en">{summary}</p>
      <details className="media-prompt-details">
        <summary>
          <span className="media-prompt-expand">{expand}</span>
          <span className="media-prompt-collapse">{collapse}</span>
          <span className="media-prompt-chevron" aria-hidden="true">⌄</span>
        </summary>
        <div className="media-prompt-sections" lang="en">
          {blocks.map((block, index) => {
            const [first, ...lines] = block.split("\n");
            const heading = first.match(/^([A-Za-z][A-Za-z ,&-]{1,48}):\s*(.*)$/);
            return (
              <div className="media-prompt-section" key={index}>
                {heading ? <><h4>{heading[1]}</h4><p>{heading[2]}</p></> : <p>{first}</p>}
                {lines.length > 0 && (
                  <ol className="media-prompt-timeline">
                    {lines.map((line, lineIndex) => {
                      const beat = line.match(/^(\d{2}:\d{2}[–-]\d{2}:\d{2}):\s*(.*)$/);
                      return beat
                        ? <li key={lineIndex}><span className="media-prompt-time">{beat[1]}</span><p>{beat[2]}</p></li>
                        : <li key={lineIndex}><p>{line}</p></li>;
                    })}
                  </ol>
                )}
              </div>
            );
          })}
        </div>
      </details>
    </div>
  );
}
