import { ROUTER_ORIGIN } from "./origins";

/**
 * Code samples for the model page's Quick Start, mirroring the console's
 * integration dialog (web/default/src/features/dashboard/components/overview/
 * integration-snippets.ts).
 *
 * Two differences, both because this runs on the public site:
 * - the model is fixed to the page's model, so there is no model picker;
 * - there is no signed-in key, so samples read $FLATKEY_API_KEY from the
 *   environment rather than embedding a masked key.
 */

export type SnippetLanguage = "curl" | "node" | "python";
export type SnippetKind = "video" | "image" | "text";

const VIDEO_PROMPT = "A cinematic product shot with soft studio lighting.";
const IMAGE_PROMPT = "A cute cat";
const CHAT_PROMPT = "Say hello in one sentence.";

const API_KEY_SHELL = "$FLATKEY_API_KEY";
const API_KEY_NODE = "process.env.FLATKEY_API_KEY";
const API_KEY_PYTHON = 'os.environ["FLATKEY_API_KEY"]';

export function snippetKindForEndpoint(endpoint: string): SnippetKind {
  if (endpoint.includes("video")) return "video";
  if (endpoint.includes("image")) return "image";
  return "text";
}

function baseUrl(): string {
  return `${ROUTER_ORIGIN}/v1`;
}

function shellSingleQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

export function buildApiSnippet(language: SnippetLanguage, model: string, kind: SnippetKind): string {
  if (language === "node") return buildNode(model, kind);
  if (language === "python") return buildPython(model, kind);
  return buildCurl(model, kind);
}

/**
 * The SDK sample ships the dependency install alongside the client setup so it
 * is runnable from an empty project. Video generation is async and not covered
 * by the OpenAI SDK, so those samples install only what they import.
 */
export function buildSdkSnippet(language: SnippetLanguage, model: string, kind: SnippetKind): string {
  if (language === "node") {
    if (kind === "video") return buildNode(model, kind);
    return ["npm install openai", "", buildNode(model, kind)].join("\n");
  }
  if (language === "python") {
    const install = kind === "video" ? "pip install requests" : "pip install openai";
    return [install, "", buildPython(model, kind)].join("\n");
  }
  return buildCurl(model, kind);
}

function buildCurl(model: string, kind: SnippetKind): string {
  if (kind === "video") {
    // Video generation is asynchronous: one call submits the task, a second
    // polls it. Both are shown so the sample is runnable as-is rather than
    // returning a task id the reader has to work out what to do with.
    const endpoint = `${baseUrl()}/videos`;
    const body = JSON.stringify({
      model,
      content: [{ type: "text", text: VIDEO_PROMPT }],
      ratio: "16:9",
      duration: 5,
    });
    return [
      "# Step 1: Create the video generation task",
      `curl ${endpoint} \\`,
      '  -H "Content-Type: application/json" \\',
      `  -H "Authorization: Bearer ${API_KEY_SHELL}" \\`,
      `  -d ${shellSingleQuote(body)}`,
      "",
      "# Step 2: Poll for the result (replace TASK_ID with the id from Step 1)",
      `curl "${endpoint}/TASK_ID" \\`,
      `  -H "Authorization: Bearer ${API_KEY_SHELL}"`,
    ].join("\n");
  }

  const endpoint = kind === "image" ? `${baseUrl()}/images/generations` : `${baseUrl()}/chat/completions`;
  const body = JSON.stringify(
    kind === "image"
      ? { model, prompt: IMAGE_PROMPT, size: "1024x1024" }
      : { model, messages: [{ role: "user", content: CHAT_PROMPT }] }
  );
  return [
    `curl ${endpoint} \\`,
    '  -H "Content-Type: application/json" \\',
    `  -H "Authorization: Bearer ${API_KEY_SHELL}" \\`,
    `  -d ${shellSingleQuote(body)}`,
  ].join("\n");
}

function buildNode(model: string, kind: SnippetKind): string {
  if (kind === "video") {
    return [
      `const endpoint = ${JSON.stringify(`${baseUrl()}/videos`)}`,
      `const headers = {`,
      `  'Content-Type': 'application/json',`,
      "  Authorization: `Bearer ${" + API_KEY_NODE + "}`,",
      `}`,
      "",
      "const created = await fetch(endpoint, {",
      "  method: 'POST',",
      "  headers,",
      "  body: JSON.stringify({",
      `    model: ${JSON.stringify(model)},`,
      `    content: [{ type: 'text', text: ${JSON.stringify(VIDEO_PROMPT)} }],`,
      "    ratio: '16:9',",
      "    duration: 5,",
      "  }),",
      "}).then((res) => res.json())",
      "",
      "// Poll until the task reports a finished status.",
      "const task = await fetch(`${endpoint}/${created.id}`, { headers }).then((res) => res.json())",
      "console.log(task)",
    ].join("\n");
  }

  const client = [
    "import OpenAI from 'openai'",
    "",
    "const client = new OpenAI({",
    `  apiKey: ${API_KEY_NODE},`,
    `  baseURL: ${JSON.stringify(baseUrl())},`,
    "})",
    "",
  ];

  if (kind === "image") {
    return [
      ...client,
      "const image = await client.images.generate({",
      `  model: ${JSON.stringify(model)},`,
      `  prompt: ${JSON.stringify(IMAGE_PROMPT)},`,
      "  size: '1024x1024',",
      "})",
      "",
      "console.log(image.data[0].url)",
    ].join("\n");
  }

  return [
    ...client,
    "const completion = await client.chat.completions.create({",
    `  model: ${JSON.stringify(model)},`,
    `  messages: [{ role: 'user', content: ${JSON.stringify(CHAT_PROMPT)} }],`,
    "})",
    "",
    "console.log(completion.choices[0].message.content)",
  ].join("\n");
}

function buildPython(model: string, kind: SnippetKind): string {
  if (kind === "video") {
    return [
      "import os",
      "import requests",
      "",
      `endpoint = ${JSON.stringify(`${baseUrl()}/videos`)}`,
      `headers = {"Authorization": f"Bearer {${API_KEY_PYTHON}}"}`,
      "",
      "created = requests.post(",
      "    endpoint,",
      "    headers=headers,",
      "    json={",
      `        "model": ${JSON.stringify(model)},`,
      `        "content": [{"type": "text", "text": ${JSON.stringify(VIDEO_PROMPT)}}],`,
      '        "ratio": "16:9",',
      '        "duration": 5,',
      "    },",
      ").json()",
      "",
      "# Poll until the task reports a finished status.",
      'task = requests.get(f"{endpoint}/{created[\'id\']}", headers=headers).json()',
      "print(task)",
    ].join("\n");
  }

  const client = [
    "import os",
    "",
    "from openai import OpenAI",
    "",
    "client = OpenAI(",
    `    api_key=${API_KEY_PYTHON},`,
    `    base_url=${JSON.stringify(baseUrl())},`,
    ")",
    "",
  ];

  if (kind === "image") {
    return [
      ...client,
      "image = client.images.generate(",
      `    model=${JSON.stringify(model)},`,
      `    prompt=${JSON.stringify(IMAGE_PROMPT)},`,
      '    size="1024x1024",',
      ")",
      "",
      "print(image.data[0].url)",
    ].join("\n");
  }

  return [
    ...client,
    "completion = client.chat.completions.create(",
    `    model=${JSON.stringify(model)},`,
    `    messages=[{"role": "user", "content": ${JSON.stringify(CHAT_PROMPT)}}],`,
    ")",
    "",
    "print(completion.choices[0].message.content)",
  ].join("\n");
}

export const CLI_INSTALL_COMMAND = "npm i -g @flatkey-ai/cli";
export const CLI_LOGIN_COMMAND = "flatkey login";

export type AgentPlatform = "mac" | "linux" | "windows";

const AGENT_INSTALL_COMMANDS: Record<AgentPlatform, string> = {
  mac: "curl -fsSL {{origin}}/install.sh | bash",
  linux: "curl -fsSL {{origin}}/install.sh | bash",
  windows: "iwr {{origin}}/install.ps1 -UseBasicParsing | iex",
};

export function buildAgentInstallCommand(platform: AgentPlatform, websiteOrigin: string): string {
  return AGENT_INSTALL_COMMANDS[platform].replace("{{origin}}", websiteOrigin.replace(/\/+$/, ""));
}

export function detectAgentPlatform(userAgent: string): AgentPlatform {
  const ua = userAgent.toLowerCase();
  if (ua.includes("win")) return "windows";
  if (ua.includes("mac")) return "mac";
  return "linux";
}
