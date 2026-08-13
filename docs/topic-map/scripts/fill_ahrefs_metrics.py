#!/usr/bin/env python3
import json
import os
import getpass
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET


WORKBOOK = Path(__file__).resolve().parents[1] / "flatkey-ai-topic-map-2026-08.xlsx"
SECONDARY_CHECKPOINT = WORKBOOK.with_suffix(".secondary-checkpoint.json")
SHEET_XML = "xl/worksheets/sheet2.xml"
AHREFS_ENDPOINT = "https://api.ahrefs.com/v3/keywords-explorer/overview"
AHREFS_MATCHING_ENDPOINT = "https://api.ahrefs.com/v3/keywords-explorer/matching-terms"
AHREFS_RELATED_ENDPOINT = "https://api.ahrefs.com/v3/keywords-explorer/related-terms"
NS = {"a": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}


def load_shared_strings(zf):
    if "xl/sharedStrings.xml" not in zf.namelist():
        return []
    root = ET.fromstring(zf.read("xl/sharedStrings.xml"))
    return ["".join(si.itertext()) for si in root.findall("a:si", NS)]


def cell_text(cell, shared_strings):
    if cell.attrib.get("t") == "s":
        value = cell.find("a:v", NS)
        if value is None or value.text is None:
            return ""
        return shared_strings[int(value.text)]
    return "".join(cell.itertext())


def load_rows():
    with zipfile.ZipFile(WORKBOOK, "r") as zf:
        shared_strings = load_shared_strings(zf)
        xml = zf.read(SHEET_XML)
    root = ET.fromstring(xml)
    rows = root.findall("a:sheetData/a:row", NS)
    data = []
    for row in rows[1:]:
        cells = row.findall("a:c", NS)
        values = [cell_text(cell, shared_strings) for cell in cells]
        if len(values) >= 12:
            data.append((int(values[0]), values[5]))
    return data


def request_ahrefs(keywords, country):
    token = os.environ["AHREFS_API_KEY"]
    query = urllib.parse.urlencode(
        {
            "select": "keyword,volume,global_volume,difficulty",
            "country": country,
            "keywords": ",".join(keywords),
            "limit": len(keywords),
        }
    )
    request = urllib.request.Request(
        f"{AHREFS_ENDPOINT}?{query}",
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/json",
        },
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.loads(response.read().decode("utf-8"))


def request_keyword_ideas(endpoint, keyword, country):
    token = os.environ["AHREFS_API_KEY"]
    query = urllib.parse.urlencode(
        {
            "country": country,
            "keywords": keyword,
            "select": "keyword,global_volume,volume,difficulty,parent_topic,intents",
            "limit": 50,
            "terms": "all",
            "view_for": "top_10",
        }
    )
    request = urllib.request.Request(
        f"{endpoint}?{query}",
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/json",
        },
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json_with_retry(request_builder, attempts=5, base_sleep=2.0):
    for attempt in range(attempts):
        try:
            return request_builder()
        except urllib.error.HTTPError as exc:
            if exc.code != 429 or attempt == attempts - 1:
                raise
            time.sleep(base_sleep * (2 ** attempt))


def extract_metrics(payload):
    rows = payload.get("keywords") or payload.get("data") or []
    if isinstance(rows, dict):
        rows = rows.get("keywords") or rows.get("data") or []
    metrics = {}
    for row in rows:
        keyword = row.get("keyword")
        if keyword:
            volume = row.get("global_volume")
            if volume is None:
                volume = row.get("volume")
            metrics[keyword.lower()] = (volume, row.get("difficulty") or row.get("kd"))
    return metrics


def extract_idea_terms(payload):
    rows = payload.get("keywords") or payload.get("data") or []
    if isinstance(rows, dict):
        rows = rows.get("keywords") or rows.get("data") or []
    ideas = []
    for row in rows:
        keyword = (row.get("keyword") or "").strip()
        if not keyword:
            continue
        volume = row.get("global_volume")
        if volume is None:
            volume = row.get("volume")
        difficulty = row.get("difficulty")
        ideas.append((keyword, volume or 0, difficulty if difficulty is not None else 999))
    ideas.sort(key=lambda item: (-item[1], item[2], len(item[0])))
    return ideas


def set_inline_value(cell, value):
    cell.attrib["t"] = "inlineStr"
    for child in list(cell):
        cell.remove(child)
    is_el = ET.SubElement(cell, f"{{{NS['a']}}}is")
    t_el = ET.SubElement(is_el, f"{{{NS['a']}}}t")
    t_el.text = str(value)


def normalize_terms(text):
    terms = []
    seen = set()
    for line in text.splitlines():
        term = line.strip()
        if not term:
            continue
        key = term.lower()
        if key in seen:
            continue
        seen.add(key)
        terms.append(term)
    return terms


def secondary_keywords(keyword, persona, product, page_type):
    k = keyword.strip()
    kl = k.lower()
    terms = []

    def add(*items):
        for item in items:
            item = item.strip()
            if item and item.lower() != kl and item not in terms:
                terms.append(item)

    if "seedance" in kl:
        add("Seedance API", "Seedance API docs", "Seedance API pricing", "Doubao Seedance API", "Volcano Engine video generation API")
        if "prompt" in kl:
            add("Seedance prompt examples", "video prompt examples", "short drama prompt template", "TikTok Shorts prompts", "AI video prompt template")
        elif "production" in kl or "short drama" in kl or "checklist" in kl or "guide" in kl or "workflow" in kl:
            add("Seedance production workflow", "AI short drama workflow", "video generation checklist", "character consistency", "batch video generation")
        elif "1.0" in kl or "pro" in kl:
            add("Seedance 1.0 Pro", "Seedance model version", "Seedance 2.0", "video model comparison")
        return "\n".join(terms[:8])

    if "claude code" in kl:
        add("Claude Code setup", "Claude Code API key", "Claude Code pricing", "Claude Code proxy", "Claude Code alternatives")
        if "pricing" in kl or "cost" in kl or "token" in kl:
            add("Claude Code cost", "Claude Code usage limit", "Claude Code billing", "cheap Claude API")
        if "proxy" in kl or "endpoint" in kl or "base_url" in kl:
            add("Claude Code base_url", "Claude Code custom endpoint", "OpenAI-compatible Claude API", "BYOK coding assistant")
        if "alternative" in kl or "vs" in kl:
            add("Codex vs Claude Code", "Claude Code alternatives", "Codex CLI", "AI coding agents")
        return "\n".join(terms[:8])

    if "codex" in kl:
        add("Codex CLI setup", "Codex CLI API key", "Codex CLI pricing", "Codex CLI proxy", "Codex CLI custom endpoint")
        if "pricing" in kl or "cost" in kl:
            add("Codex CLI cost", "Codex CLI usage limit", "AI coding tool cost comparison")
        if "proxy" in kl or "endpoint" in kl or "base_url" in kl:
            add("Codex CLI base_url", "Codex CLI custom API endpoint", "OpenAI-compatible API", "BYOK coding assistant")
        if "alternative" in kl or "vs" in kl:
            add("Claude Code alternatives", "Codex vs Claude Code", "AI coding agents", "terminal AI coding agent")
        return "\n".join(terms[:8])

    if "k3" in kl:
        add("K3 API docs", "K3 API pricing", "K3 API authentication", "OpenAI-compatible API", "LLM eval harness", "Chinese LLM API")
        return "\n".join(terms[:8])

    if "chinese llm" in kl or "kimi" in kl or "qwen" in kl or "deepseek" in kl:
        add("Chinese LLM API", "Kimi API", "Qwen API", "DeepSeek API", "OpenAI-compatible Chinese LLM", "Chinese LLM pricing", "Chinese LLM benchmark")
        return "\n".join(terms[:8])

    if "llm eval" in kl or "llm evaluation" in kl or "llm benchmark" in kl:
        add("LLM evaluation", "LLM eval harness", "LLM benchmark", "LLM evaluation metrics", "LLM regression testing", "eval dataset", "LLM judge")
        return "\n".join(terms[:8])

    if "llm observability" in kl or "llm tracing" in kl or "llm api gateway" in kl or "llm model router" in kl:
        add("LLM observability", "LLM tracing", "LLM API gateway", "LLM model routing", "fallback routing")
        return "\n".join(terms[:8])

    if "video" in kl:
        add("video generation API", "text-to-video API", "image-to-video API", "AI video API pricing", "AI video API examples", "video prompt template")
        return "\n".join(terms[:8])

    if "openai-compatible api" in kl:
        add("OpenAI-compatible API setup", "OpenAI-compatible API base_url", "OpenAI-compatible API pricing", "OpenAI API alternative", "BYOK AI tools")
        return "\n".join(terms[:8])

    if "cheap ai api" in kl or "ai api for indie" in kl or "ai coding agent api" in kl:
        add("cheap AI API", "AI API for indie developers", "OpenAI-compatible API", "AI coding agent API", "BYOK AI coding tools")
        return "\n".join(terms[:8])

    if "api" in kl:
        add(k + " docs", k + " pricing", k + " examples")
    if "checklist" in kl:
        add(k.replace("checklist", "guide"), k.replace("checklist", "template"))
    if "guide" in kl:
        add(k.replace("guide", "checklist"), k.replace("guide", "template"))
    return "\n".join(terms[:8])


def build_secondary_keywords(keyword, persona, product, page_type, country):
    manual = normalize_terms(secondary_keywords(keyword, persona, product, page_type))
    ahrefs_terms = []
    for endpoint in (AHREFS_MATCHING_ENDPOINT, AHREFS_RELATED_ENDPOINT):
        try:
            payload = request_keyword_ideas(endpoint, keyword, country)
            for term, volume, difficulty in extract_idea_terms(payload):
                lower = term.lower()
                if lower == keyword.lower():
                    continue
                if any(existing.lower() == lower for existing in manual):
                    continue
                ahrefs_terms.append((term, volume, difficulty))
        except Exception:
            continue

    ahrefs_terms.sort(key=lambda item: (-item[1], item[2], len(item[0])))
    merged = []
    seen = set()
    for term in manual:
        key = term.lower()
        if key in seen:
            continue
        seen.add(key)
        merged.append(term)
    for term, _, _ in ahrefs_terms:
        key = term.lower()
        if key in seen:
            continue
        seen.add(key)
        merged.append(term)
        if len(merged) >= 8:
            break
    return "\n".join(merged[:8])


def get_priority(cell, shared_strings):
    return cell_text(cell, shared_strings).strip()


def update_workbook(metrics):
    tmp = WORKBOOK.with_suffix(".tmp.xlsx")
    with zipfile.ZipFile(WORKBOOK, "r") as zin:
        files = {name: zin.read(name) for name in zin.namelist()}
        shared_strings = load_shared_strings(zin)

    root = ET.fromstring(files[SHEET_XML])
    for row in root.findall("a:sheetData/a:row", NS)[1:]:
        cells = row.findall("a:c", NS)
        row_id = int(cell_text(cells[0], shared_strings))
        if row_id not in metrics:
            continue
        volume, kd = metrics[row_id]
        set_inline_value(cells[10], volume if volume is not None else "0")
        set_inline_value(cells[11], kd if kd is not None else "N/A")

    files[SHEET_XML] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
    with zipfile.ZipFile(tmp, "w", compression=zipfile.ZIP_DEFLATED) as zout:
        for name, content in files.items():
            zout.writestr(name, content)
    tmp.replace(WORKBOOK)


def load_secondary_checkpoint():
    if not SECONDARY_CHECKPOINT.exists():
        return set()
    try:
        payload = json.loads(SECONDARY_CHECKPOINT.read_text())
    except Exception:
        return set()
    return {int(item) for item in payload.get("completed_rows", [])}


def save_secondary_checkpoint(completed_rows):
    SECONDARY_CHECKPOINT.write_text(
        json.dumps({"completed_rows": sorted(completed_rows)}, ensure_ascii=False, indent=2)
    )


def clear_secondary_checkpoint():
    if SECONDARY_CHECKPOINT.exists():
        SECONDARY_CHECKPOINT.unlink()


def main():
    if "AHREFS_API_KEY" not in os.environ:
        token = getpass.getpass("AHREFS_API_KEY: ").strip()
        if not token:
            raise SystemExit("Missing AHREFS_API_KEY.")
        os.environ["AHREFS_API_KEY"] = token

    country = os.environ.get("AHREFS_COUNTRY", "us")
    sleep_seconds = float(os.environ.get("AHREFS_SLEEP_SECONDS", "0.3"))
    batch_size = int(os.environ.get("AHREFS_BATCH_SIZE", "10"))
    rows = load_rows()
    metrics = {}
    failures = []
    for start in range(0, len(rows), batch_size):
        batch = rows[start : start + batch_size]
        keywords = [keyword for _, keyword in batch]
        try:
            payload = post_json_with_retry(lambda: request_ahrefs(keywords, country))
            batch_metrics = extract_metrics(payload)
            for row_id, keyword in batch:
                value = batch_metrics.get(keyword.lower(), (None, None))
                metrics[row_id] = value
                print(f"{row_id}\t{keyword}\t{value[0]}\t{value[1]}")
        except urllib.error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace")
            failures.append((start + 1, keywords[0], exc.code, body[:200]))
            print(f"FAIL\tbatch {start + 1}-{start + len(batch)}\tHTTP {exc.code}\t{body[:120]}")
        except Exception as exc:
            failures.append((start + 1, keywords[0], "ERR", str(exc)))
            print(f"FAIL\tbatch {start + 1}-{start + len(batch)}\t{exc}")
        time.sleep(sleep_seconds)

    if metrics:
        update_workbook(metrics)

    # Refresh secondary keywords after the volume pass so the workbook is aligned
    # with Ahrefs keyword-idea reports instead of pure heuristic expansion.
    with zipfile.ZipFile(WORKBOOK, "r") as zin:
        files = {name: zin.read(name) for name in zin.namelist()}
        shared_strings = load_shared_strings(zin)
    root = ET.fromstring(files[SHEET_XML])
    completed_rows = load_secondary_checkpoint()
    refreshed = 0
    for row in root.findall("a:sheetData/a:row", NS)[1:]:
        cells = row.findall("a:c", NS)
        row_id = int(cell_text(cells[0], shared_strings))
        priority = get_priority(cells[12], shared_strings)
        if priority not in {"P0", "P1"}:
            continue
        if row_id in completed_rows:
            continue
        keyword = cell_text(cells[5], shared_strings)
        persona = cell_text(cells[1], shared_strings)
        product = cell_text(cells[4], shared_strings)
        page_type = cell_text(cells[8], shared_strings)
        sec = post_json_with_retry(lambda: build_secondary_keywords(keyword, persona, product, page_type, country))
        set_inline_value(cells[6], sec)
        completed_rows.add(row_id)
        refreshed += 1
        files[SHEET_XML] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
        with zipfile.ZipFile(tmp := WORKBOOK.with_suffix(".tmp.xlsx"), "w", compression=zipfile.ZIP_DEFLATED) as zout:
            for name, content in files.items():
                zout.writestr(name, content)
        tmp.replace(WORKBOOK)
        save_secondary_checkpoint(completed_rows)
    files[SHEET_XML] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
    clear_secondary_checkpoint()
    print(f"Refreshed secondary keywords for {refreshed} rows.")

    if failures:
        print(f"Completed with {len(failures)} failures.")
    else:
        print("Completed.")


if __name__ == "__main__":
    main()
