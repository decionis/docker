import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";
import { readBoundedJsonResponse } from "./BoundedJsonResponse.mjs";

const apiVersion = "2022-11-28";
const defaultTimeoutMs = 10_000;
const defaultMaxAttempts = 2;
const maxTimeoutMs = 15_000;
const maxAttemptsLimit = 3;
const maxPages = 3;
const perPage = 100;
const branchCreationWorkflow = ".github/workflows/actions.yml";
const prBodySectionLimit = 8 * 1024;
const commitHeadlineLimit = 160;

export class GitHubApiError extends Error {
  constructor(message, status) {
    super(message);
    this.name = "GitHubApiError";
    this.status = status;
  }
}

function boundedInteger(value, fallback, maximum, name) {
  const resolved = value === undefined ? fallback : value;
  if (!Number.isSafeInteger(resolved) || resolved < 1) {
    throw new Error(name + " must be a positive safe integer");
  }
  return Math.min(resolved, maximum);
}

function normalizeMarkdown(value) {
  return typeof value === "string"
    ? value.replace(/\r\n?/g, "\n").replace(/\n{3,}/g, "\n\n").trim()
    : "";
}

function boundedHeadline(value, fallback) {
  const headline = normalizeMarkdown(value).split("\n")[0]?.trim() || fallback;
  return headline.length <= commitHeadlineLimit
    ? headline
    : headline.slice(0, commitHeadlineLimit - 3).trimEnd() + "...";
}

function boundedSection(value, fallback, limit = prBodySectionLimit) {
  const section = normalizeMarkdown(value) || fallback;
  if (section.length <= limit) return section;
  const suffix = "\n\n_Content truncated by Decionis Bot._";
  return section.slice(0, Math.max(0, limit - suffix.length)).trimEnd() + suffix;
}

function normalizedHeading(heading) {
  return heading.toLowerCase().replace(/[`*_]/g, "").trim();
}

function categoryFromHeading(heading) {
  const normalized = normalizedHeading(heading);
  if (/\b(test plan|tests?|testing|verification|checks?|qa)\b/.test(normalized)) {
    return "test";
  }
  if (/\b(validation|acceptance|proof)\b/.test(normalized)) return "validation";
  if (normalized === "what & why" || normalized === "what and why") return "result";
  if (/\b(problem|why|motivation|context|background|root cause)\b/.test(normalized)) {
    return "problem";
  }
  if (
    /\b(result|changes?|what changed|implementation|impact|solution|boundary|publish gates?)\b/.test(
      normalized,
    )
  ) {
    return "result";
  }
  if (/\b(summary|overview)\b/.test(normalized)) return "summary";
  return "result";
}

function shouldPreserveHeading(heading, category) {
  const normalized = normalizedHeading(heading);
  const canonicalHeadings = {
    summary: ["summary"],
    problem: ["problem"],
    result: ["result"],
    test: ["test", "tests", "testing"],
    validation: ["validation"],
  };
  return !canonicalHeadings[category].includes(normalized);
}

function parseCommitMessage(commit, index) {
  const message = normalizeMarkdown(commit?.commit?.message);
  const [subject = "", ...bodyLines] = message.split("\n");
  const headline = boundedHeadline(subject, "Commit " + (index + 1));
  const sections = { summary: [], problem: [], result: [], test: [], validation: [] };
  let category = "summary";
  let heading;
  let lines = [];

  const flush = () => {
    const text = normalizeMarkdown(lines.join("\n"));
    if (text) sections[category].push({ heading, text });
    lines = [];
  };

  for (const line of bodyLines) {
    const headingMatch = line.match(/^#{1,6}\s+(.+?)\s*$/);
    if (headingMatch) {
      flush();
      heading = headingMatch[1].trim();
      category = categoryFromHeading(heading);
    } else {
      lines.push(line);
    }
  }
  flush();
  return { headline, sections };
}

function formatCommitSections(commits, category) {
  const groups = commits
    .map(({ headline, sections }) => ({ headline, sections: sections[category] }))
    .filter(({ sections }) => sections.length > 0);
  if (groups.length === 0) return "";

  return groups
    .map(({ headline, sections }) => {
      const content = sections
        .map(({ heading, text }) =>
          heading && shouldPreserveHeading(heading, category)
            ? "**" + heading + "**\n\n" + text
            : text,
        )
        .join("\n\n");
      return groups.length > 1 ? "### " + headline + "\n\n" + content : content;
    })
    .join("\n\n");
}

function inlineCode(value) {
  const quote = String.fromCharCode(96);
  return quote + String(value).replaceAll(quote, "\\" + quote) + quote;
}

export function pullRequestBodyFromCommits({
  branch,
  defaultBranch,
  expectedAuthorLogin,
  reason,
  commits,
}) {
  const parsedCommits = (Array.isArray(commits) ? commits : []).map(parseCommitMessage);
  if (parsedCommits.length === 0) {
    parsedCommits.push(parseCommitMessage({ commit: { message: "Changes from " + branch } }, 0));
  }

  const headlines = parsedCommits.map(({ headline }) => "- " + headline).join("\n");
  const summaryDetails = formatCommitSections(parsedCommits, "summary");
  const summary = [headlines, summaryDetails].filter(Boolean).join("\n\n");
  const problem = formatCommitSections(parsedCommits, "problem");
  const result = formatCommitSections(parsedCommits, "result");
  const test = formatCommitSections(parsedCommits, "test");
  const validationDetails = formatCommitSections(parsedCommits, "validation");
  const commitLabel = parsedCommits.length === 1 ? "commit" : "commits";
  const validationFacts = [
    "- Trust check: " + reason + ".",
    "- " +
      parsedCommits.length +
      " " +
      commitLabel +
      " ahead of " +
      inlineCode(defaultBranch) +
      ".",
    "- Existing commit authorship is unchanged.",
    "- A CODEOWNER review from @" + expectedAuthorLogin + " is required before merge.",
    "- Decionis Bot cannot push commits, approve reviews, or merge this pull request.",
  ].join("\n");
  const validationDetailLimit = prBodySectionLimit - validationFacts.length - 2;
  const validation = [
    validationDetails ? boundedSection(validationDetails, "", validationDetailLimit) : "",
    validationFacts,
  ]
    .filter(Boolean)
    .join("\n\n");

  return [
    "## Summary",
    "",
    boundedSection(summary, "Changes from " + inlineCode(branch) + "."),
    "",
    "## Problem",
    "",
    boundedSection(problem, "No explicit problem statement was provided in the commit messages."),
    "",
    "## Result",
    "",
    boundedSection(result, "The branch delivers the changes listed in the Summary."),
    "",
    "## Test",
    "",
    boundedSection(test, "No test details were provided in the commit messages."),
    "",
    "## Validation",
    "",
    validation,
  ].join("\n");
}

export class GitHubApiClient {
  constructor({
    token,
    fetchImpl = globalThis.fetch,
    timeoutMs = defaultTimeoutMs,
    maxAttempts = defaultMaxAttempts,
  }) {
    if (!token) throw new Error("GITHUB_TOKEN is required");
    if (typeof fetchImpl !== "function") throw new Error("A fetch implementation is required");
    this.token = token;
    this.fetchImpl = fetchImpl;
    this.timeoutMs = boundedInteger(timeoutMs, defaultTimeoutMs, maxTimeoutMs, "timeoutMs");
    this.maxAttempts = boundedInteger(
      maxAttempts,
      defaultMaxAttempts,
      maxAttemptsLimit,
      "maxAttempts",
    );
  }

  async request(method, path, { query, body } = {}) {
    const url = new URL(path, "https://api.github.com");
    for (const [key, value] of Object.entries(query ?? {})) {
      if (value !== undefined && value !== null && value !== "") {
        url.searchParams.set(key, String(value));
      }
    }

    let lastError;
    for (let attempt = 1; attempt <= this.maxAttempts; attempt += 1) {
      try {
        const response = await this.fetchImpl(url, {
          method,
          headers: {
            Accept: "application/vnd.github+json",
            Authorization: "Bearer " + this.token,
            "X-GitHub-Api-Version": apiVersion,
            ...(body === undefined ? {} : { "Content-Type": "application/json" }),
          },
          body: body === undefined ? undefined : JSON.stringify(body),
          signal: AbortSignal.timeout(this.timeoutMs),
        });
        if (response.ok) return await readBoundedJsonResponse(response);

        const error = new GitHubApiError(
          "GitHub API request failed: " +
            method +
            " " +
            url.pathname +
            " (" +
            response.status +
            ")",
          response.status,
        );
        if (!this.isRetryable(response.status) || attempt === this.maxAttempts) throw error;
        lastError = error;
      } catch (error) {
        if (error instanceof GitHubApiError) {
          if (!this.isRetryable(error.status) || attempt === this.maxAttempts) throw error;
        } else if (attempt === this.maxAttempts) {
          throw error;
        }
        lastError = error;
      }
      await this.wait(attempt);
    }
    throw lastError ?? new Error("GitHub API request failed");
  }

  async paginate(path, query = {}) {
    const values = [];
    for (let page = 1; page <= maxPages; page += 1) {
      const response = await this.request("GET", path, {
        query: { ...query, per_page: perPage, page },
      });
      if (!Array.isArray(response)) throw new Error("Expected an array from " + path);
      values.push(...response);
      if (response.length < perPage) return values;
    }
    throw new Error("Pagination limit exceeded for " + path);
  }

  isRetryable(status) {
    return status === 429 || status >= 500;
  }

  async wait(attempt) {
    const delayMs = 200 * attempt + Math.floor(Math.random() * 100);
    await new Promise((resolve) => setTimeout(resolve, delayMs));
  }
}

export class PullRequestBot {
  constructor({ api, repository, expectedAuthorLogin, defaultBranch }) {
    const [owner, name, extra] = repository.split("/");
    if (!owner || !name || extra) throw new Error("GITHUB_REPOSITORY must be owner/name");
    if (!expectedAuthorLogin) throw new Error("EXPECTED_AUTHOR_LOGIN is required");
    if (!defaultBranch) throw new Error("DEFAULT_BRANCH is required");
    this.api = api;
    this.owner = owner;
    this.name = name;
    this.repository = repository;
    this.expectedAuthorLogin = expectedAuthorLogin.toLowerCase();
    this.defaultBranch = defaultBranch;
  }

  async run(eventName, event) {
    if (eventName === "create" && event?.ref_type !== "branch") {
      return [{ branch: event?.ref, status: "skipped", reason: "non-branch-create-event" }];
    }
    const requestedBranch =
      eventName === "workflow_dispatch"
        ? this.readInput(event, "branch")
        : eventName === "create" && event?.ref_type === "branch"
          ? event.ref
          : undefined;
    const creationRunId =
      eventName === "workflow_dispatch" ? this.readInput(event, "creation_run_id") : undefined;
    const branches = requestedBranch ? [requestedBranch] : await this.listCandidateBranches();
    const outcomes = [];

    for (const branch of branches) {
      outcomes.push(await this.ensurePullRequest(branch, creationRunId));
    }
    return outcomes;
  }

  readInput(event, name) {
    const value = event?.inputs?.[name];
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : undefined;
  }

  async listCandidateBranches() {
    const branches = await this.api.paginate("/repos/" + this.repository + "/branches");
    return branches
      .map(({ name }) => name)
      .filter((name) => typeof name === "string" && name !== this.defaultBranch);
  }

  async ensurePullRequest(branch, creationRunId) {
    if (!branch || branch === this.defaultBranch) {
      return { branch, status: "skipped", reason: "default-or-empty-branch" };
    }

    const existing = await this.findExistingPullRequest(branch);
    if (existing) {
      return { branch, status: "skipped", reason: "pull-request-exists", url: existing.html_url };
    }

    const comparison = await this.compareBranch(branch);
    if (!comparison || comparison.ahead_by < 1) {
      return { branch, status: "skipped", reason: "no-commits-ahead" };
    }
    if (
      !Array.isArray(comparison.commits) ||
      comparison.total_commits !== comparison.commits.length
    ) {
      return { branch, status: "skipped", reason: "incomplete-commit-comparison" };
    }

    const createdByExpectedAuthor = creationRunId
      ? await this.isTrustedCreationRun(creationRunId, branch)
      : await this.hasTrustedCreationRun(branch);
    const commitsByExpectedAuthor = comparison.commits.every(
      (commit) => commit.author?.login?.toLowerCase() === this.expectedAuthorLogin,
    );
    if (!createdByExpectedAuthor && !commitsByExpectedAuthor) {
      return { branch, status: "skipped", reason: "untrusted-branch-and-commit-authors" };
    }

    const reason = createdByExpectedAuthor
      ? "branch creation was attributed to @" + this.expectedAuthorLogin
      : "every commit ahead of " +
        this.defaultBranch +
        " was attributed to @" +
        this.expectedAuthorLogin;
    const latestCommit = comparison.commits.at(-1);
    const title = titleFromMessage(latestCommit?.commit?.message, branch);
    const pullRequest = await this.api.request("POST", "/repos/" + this.repository + "/pulls", {
      body: {
        title,
        head: branch,
        base: this.defaultBranch,
        body: this.buildBody(branch, reason, comparison.commits),
        draft: false,
        maintainer_can_modify: true,
      },
    });
    return { branch, status: "created", reason, url: pullRequest.html_url };
  }

  async findExistingPullRequest(branch) {
    const pulls = await this.api.request("GET", "/repos/" + this.repository + "/pulls", {
      query: {
        state: "all",
        head: this.owner + ":" + branch,
        base: this.defaultBranch,
        per_page: perPage,
      },
    });
    if (!Array.isArray(pulls)) throw new Error("Expected pull request list");
    return pulls[0];
  }

  async compareBranch(branch) {
    const base = encodeURIComponent(this.defaultBranch);
    const head = encodeURIComponent(branch);
    try {
      return await this.api.request(
        "GET",
        "/repos/" + this.repository + "/compare/" + base + "..." + head,
        { query: { per_page: perPage } },
      );
    } catch (error) {
      if (error instanceof GitHubApiError && error.status === 404) return null;
      throw error;
    }
  }

  async isTrustedCreationRun(runId, branch) {
    if (!/^\d+$/.test(String(runId))) return false;
    const run = await this.api.request(
      "GET",
      "/repos/" + this.repository + "/actions/runs/" + runId,
    );
    return this.creationRunMatches(run, branch);
  }

  async hasTrustedCreationRun(branch) {
    const response = await this.api.request("GET", "/repos/" + this.repository + "/actions/runs", {
      query: {
        event: "create",
        branch,
        actor: this.expectedAuthorLogin,
        per_page: perPage,
      },
    });
    const runs = response?.workflow_runs;
    if (!Array.isArray(runs)) throw new Error("Expected workflow run list");
    return runs.some((run) => this.creationRunMatches(run, branch));
  }

  creationRunMatches(run, branch) {
    const workflowPath = run?.path?.split("@")[0];
    return (
      run?.event === "create" &&
      run?.head_branch === branch &&
      run?.actor?.login?.toLowerCase() === this.expectedAuthorLogin &&
      run?.repository?.full_name === this.repository &&
      workflowPath === branchCreationWorkflow
    );
  }

  buildBody(branch, reason, commits) {
    return pullRequestBodyFromCommits({
      branch,
      defaultBranch: this.defaultBranch,
      expectedAuthorLogin: this.expectedAuthorLogin,
      reason,
      commits,
    });
  }
}

export function titleFromMessage(message, branch) {
  const headline = message?.split("\n")[0]?.trim() || "Changes from " + branch;
  return headline.length <= 72 ? headline : headline.slice(0, 69) + "...";
}

async function main() {
  const eventPath = process.env.GITHUB_EVENT_PATH;
  if (!eventPath) throw new Error("GITHUB_EVENT_PATH is required");
  const event = JSON.parse(await readFile(eventPath, "utf8"));
  const api = new GitHubApiClient({ token: process.env.GITHUB_TOKEN });
  const bot = new PullRequestBot({
    api,
    repository: process.env.GITHUB_REPOSITORY,
    expectedAuthorLogin: process.env.EXPECTED_AUTHOR_LOGIN,
    defaultBranch: process.env.DEFAULT_BRANCH,
  });
  const outcomes = await bot.run(process.env.GITHUB_EVENT_NAME, event);
  for (const outcome of outcomes) {
    process.stdout.write(JSON.stringify(outcome) + "\n");
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    process.stderr.write((error instanceof Error ? error.message : "PR bot failed") + "\n");
    process.exitCode = 1;
  });
}
