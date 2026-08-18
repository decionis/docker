#!/usr/bin/env node
// MCP stdio smoke client for the decionis/mcp image: initialize handshake,
// exact tool inventory (the drift gate of rules/discovery.rules.md Rule 1.4),
// and an evaluate round-trip that must come back blocked. The container runs
// with --network none and --read-only so every pass re-proves the image's
// zero-network, no-state posture (rules/security.rules.md Rules 6.1, 7.4).
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const image = process.argv[2];
if (!image) {
  console.error("usage: SmokeClient.mjs <image>");
  process.exit(2);
}

const fixture = join(dirname(fileURLToPath(import.meta.url)), "fixtures", "DECIONIS_POLICY.md");
const EXPECTED_TOOLS = ["decionis_evaluate", "decionis_read_policy", "decionis_verdict_help"];

const child = spawn(
  "docker",
  [
    "run", "-i", "--rm",
    "--network", "none",
    "--read-only",
    "-v", `${fixture}:/work/DECIONIS_POLICY.md:ro`,
    image,
  ],
  { stdio: ["pipe", "pipe", "inherit"] },
);

const pending = new Map();
let nextId = 0;

const rl = createInterface({ input: child.stdout });
rl.on("line", (line) => {
  let msg;
  try {
    msg = JSON.parse(line);
  } catch {
    return;
  }
  if (msg.id !== undefined && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    if (msg.error) reject(new Error(`rpc error ${msg.error.code}: ${msg.error.message}`));
    else resolve(msg.result);
  }
});

function request(method, params) {
  const id = ++nextId;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
  });
}

function notify(method, params) {
  child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", method, params })}\n`);
}

function fail(message) {
  console.error(`FAIL: ${message}`);
  child.kill("SIGKILL");
  process.exit(1);
}

const timeout = setTimeout(() => fail("timed out after 60s"), 60_000);

const init = await request("initialize", {
  protocolVersion: "2025-06-18",
  capabilities: {},
  clientInfo: { name: "decionis-docker-smoke", version: "0.0.0" },
});
if (init?.serverInfo?.name !== "decionis-mcp") {
  fail(`unexpected serverInfo: ${JSON.stringify(init?.serverInfo)}`);
}
console.log(`ok: initialize (serverInfo.name=${init.serverInfo.name}, protocolVersion=${init.protocolVersion})`);
notify("notifications/initialized");

const tools = await request("tools/list", {});
const names = (tools?.tools ?? []).map((t) => t.name).sort();
if (JSON.stringify(names) !== JSON.stringify(EXPECTED_TOOLS)) {
  fail(`tool inventory drift: got ${JSON.stringify(names)}, want ${JSON.stringify(EXPECTED_TOOLS)}`);
}
console.log(`ok: tools/list matches pinned inventory (${names.join(", ")})`);

const evaluated = await request("tools/call", {
  name: "decionis_evaluate",
  arguments: {
    payload: { action: "production-deploy", change_freeze: true, agent_generated: true },
    mode: "enforce",
  },
});
if (evaluated?.isError) {
  fail(`evaluate returned isError: ${JSON.stringify(evaluated.content)}`);
}
const textBlock = (evaluated?.content ?? []).find((c) => c.type === "text");
if (!textBlock) fail(`evaluate returned no text content: ${JSON.stringify(evaluated)}`);
const verdict = JSON.parse(textBlock.text);
if (verdict.verdict !== "block" || verdict.outcome !== "REJECT") {
  fail(`expected verdict=block outcome=REJECT, got verdict=${verdict.verdict} outcome=${verdict.outcome}`);
}
if (verdict.selected_rule !== "Block deploys during a change freeze") {
  fail(`unexpected selected_rule: ${JSON.stringify(verdict.selected_rule)}`);
}
console.log(`ok: evaluate -> verdict=${verdict.verdict}, outcome=${verdict.outcome}, rule="${verdict.selected_rule}"`);

clearTimeout(timeout);
child.stdin.end();
const code = await new Promise((resolve) => child.on("close", resolve));
if (code !== 0) {
  console.error(`FAIL: container exited with code ${code}`);
  process.exit(1);
}
console.log("ok: clean shutdown on stdin close");
