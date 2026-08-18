Yes. I think there is a worthwhile product here — but I would **narrow Gemini’s proposal substantially**.

The opportunity is not really “Decionis for Docker.” It is:

> **Make Decionis the authorization layer between AI agents and the tools they can execute in containerized developer environments.**

That fits Decionis extremely well, and Docker has moved much closer to this problem than the Gemini proposal seems to recognize.

Docker Desktop now has an MCP Toolkit, an MCP Catalog with **300+ verified servers**, profiles shared across AI clients, and an MCP Gateway that sits between clients such as Claude/Cursor and MCP servers. Docker explicitly describes the Gateway as handling routing, authentication, credentials, and access control. ([Docker Documentation][1])

That means the distribution thesis is good, but **the architecture should ride Docker's emerging MCP infrastructure rather than recreate it.**

## I would build it — but in this order

| Component                            | Build?   | Priority | Why                              |
| ------------------------------------ | -------- | -------: | -------------------------------- |
| **Decionis MCP Server container**    | **Yes**  |    🟣 P0 | Distribution wedge               |
| **Docker Desktop Extension**         | **Yes**  |    🟣 P0 | Visual authority/audit surface   |
| **MCP authorization integration**    | **Yes**  |    🟣 P0 | Strongest strategic fit          |
| **Presence approval from Extension** | **Yes**  |    🟣 P0 | Differentiator                   |
| Dev Container Feature                | Yes      |       P1 | Excellent developer distribution |
| `authority-proxy` sidecar            | Yes      |       P1 | Useful beyond MCP                |
| Docker socket interception           | Later    |       P2 | Complex + potentially brittle    |
| Hardened base images                 | No/Maybe |       P3 | Distracts from core              |
| `presence-agent` biometric container | **No**   |        — | Wrong abstraction                |

### The important discovery: Docker is already building the gateway

This changes the strategy.

Docker's MCP Gateway is already a centralized proxy between AI clients and MCP servers. In Docker Desktop, it runs automatically when MCP Toolkit is enabled. ([Docker Documentation][2])

So I would **not build another "Local MCP Gateway."**

Instead:

```text
Claude / Cursor / VS Code / Agent
               │
               ▼
       Docker MCP Gateway
               │
               ▼
        MCP Tool Request
               │
               ▼
      ┌──────────────────┐
      │     DECIONIS     │
      │ Execution Auth.  │
      │                  │
      │ PROCEED          │
      │ HOLD             │
      │ BLOCK            │
      └────────┬─────────┘
               │
       ┌───────┴────────┐
       │                │
    PROCEED            HOLD
       │                │
       ▼                ▼
 MCP Server          PRESENCE
       │          Human approval
       │                │
       │          verified approval
       │                │
       └───────┬────────┘
               ▼
            Execute
               │
               ▼
       Decision Dossier
```

That is much more powerful strategically.

Docker answers:

> **“How does the agent reach its tools?”**

Decionis answers:

> **“Is this agent authorized to use this tool, under these circumstances, right now?”**

Presence answers:

> **“When policy requires a human, who is authorizing this execution?”**

Those are cleanly separated layers.

---

# 1. P0: `decionis/mcp-server`

I'd do this first.

You already have the Decionis MCP server and have been testing it through Claude and ChatGPT. Containerizing that existing capability gives you another distribution channel without creating another product.

Docker's MCP Toolkit supports containerized MCP servers and connects them to multiple AI clients. ([Docker Documentation][1])

The developer experience should eventually be something as simple as:

```text
Docker Desktop
   ↓
MCP Toolkit
   ↓
Catalog
   ↓
Decionis Execution Authority
   ↓
Add
```

Then Claude/Cursor/VS Code can discover Decionis through their Docker MCP profile.

This is **distribution leverage**, not integration proliferation.

That's an important distinction from some of the integrations we've previously discussed.

---

# 2. P0: Docker Desktop Extension

This one I particularly like.

Docker Extensions can contain a UI, a long-running containerized backend, and even host executables. The frontend can invoke Docker commands and communicate with the backend. ([Docker Documentation][3])

So imagine opening Docker Desktop and seeing:

```text
DECIONIS
Execution Authority
────────────────────────────────────

Environment              ● Protected

Recent Decisions

13:42  github.create_release       PROCEED
13:40  postgres.drop_database      BLOCK
13:38  terraform.apply             HOLD
13:37  npm.publish                 PROCEED
13:31  docker.run privileged       BLOCK

────────────────────────────────────

Pending Human Approval

terraform.apply
production-eu

Risk       HIGH
Policy     production-infra-v7
Requested  Claude Code

        [ Review with Presence ]

────────────────────────────────────

Policy drift                 2
Overrides today              1
Signed dossiers             47
```

Now Decionis suddenly becomes **visible infrastructure**.

That is important for developer adoption.

Instead of developers understanding Decionis from documentation, they can watch the control plane work.

And Docker Extensions are distributed as container images through Docker Hub and can be installed through Docker Desktop's Extensions Marketplace. ([Docker Documentation][3])

---

# 3. Presence makes this considerably more interesting

I disagree with Gemini's proposed:

> `decionis/presence-agent`

Presence shouldn't primarily be a daemon container handling biometrics.

The actual authentication ceremony belongs with the **human's trusted device/browser/OS authenticator**, while Docker is merely the surface from which the challenge gets initiated.

For example:

```text
Agent → terraform.apply
             │
             ▼
          Decionis
             │
            HOLD
             │
             ▼
Docker Desktop Extension

"Human authorization required"

        Approve
           │
           ▼
      Presence session
           │
     WebAuthn / Passkey
           │
     Touch ID / Phone
           │
           ▼
  Presence Proof Token
           │
           ▼
        Decionis
           │
        PROCEED
           │
           ▼
    terraform.apply
```

This would be a beautiful demonstration of the Decionis + Presence architecture because the agent doesn't ask:

> “Festus, may I continue?”

It asks the **authority system**.

The policy decides whether human authority is required.

Presence proves the authorized human supplied that authority.

Then execution resumes.

That's substantially stronger than an AI application's own “Allow this command?” modal.

---

# 4. Dev Container Feature: absolutely

This is probably the cheapest useful thing on the whole roadmap.

Dev Container Features are self-contained installation/configuration units and can be published as OCI artifacts. ([Containers.dev][4])

You could publish something like:

```text
ghcr.io/decionis/features/govern:1
```

Then:

```json
{
  "features": {
    "ghcr.io/decionis/features/govern:1": {}
  }
}
```

Now a team can make a repository **Decionis-governed by default**.

That creates an interesting enterprise pattern:

```text
git clone
   ↓
Open in Dev Container
   ↓
Decionis automatically installed
   ↓
Agent starts coding
   ↓
Sensitive execution governed
```

And it travels **with the repository**.

That's potentially more strategically important than it initially appears.

---

# 5. I would change the sidecar concept slightly

Gemini suggests:

> intercept all outgoing HTTP calls.

I wouldn't start there.

Generic HTTP interception creates a semantic problem.

Decionis doesn't merely need:

```text
POST https://api.stripe.com/...
```

It wants:

```text
decision_type: FINANCIAL_ROUTING
amount: 250000
transaction_type: WIRE_TRANSFER
risk_score: .93
...
```

Likewise:

```text
POST api.github.com/repos/...
```

is much less useful than:

```text
action: github.release.create
repository: production
actor: claude-code
environment: production
```

So I'd make the proxy **action-aware rather than packet-aware**.

Call it something like:

```text
decionis/authority
```

or

```text
decionis/execution-gate
```

rather than a generic network proxy.

---

# 6. Docker socket interception is where I'd be careful

This is the part of Gemini's proposal I would **not build first**.

Docker extensions have powerful access. Docker explicitly warns that extensions can run Docker commands, mount folders, run binaries, access user-accessible files, and effectively operate with the Docker Desktop user's permissions. ([Docker Documentation][5])

An extension backend can also mount Docker Desktop's Engine socket and interact with the Engine. ([Docker Documentation][6])

So observing the engine is straightforward.

But **reliably becoming a transparent mandatory pre-execution authorization point for every Docker API operation is a much stronger requirement**.

I wouldn't position V1 as:

> “Decionis intercepts every Docker command before Docker executes it.”

until you've proven the enforcement mechanism across Docker Desktop versions and platforms.

Instead, V1:

### Govern agent actions.

V2:

### Govern privileged developer actions.

V3:

### Explore engine-level mandatory authorization.

That progression is safer technically and commercially.

---

# Where I think this becomes strategically important

There is an emerging stack:

```text
┌─────────────────────────────────────────────┐
│               AI APPLICATION               │
│ Claude / Cursor / VS Code / Codex / Agents │
└─────────────────────┬───────────────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│              TOOL DISCOVERY                │
│            Docker MCP Toolkit              │
└─────────────────────┬───────────────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│                AUTHORITY                    │
│                 Decionis                    │
│                                             │
│ Policy • Risk • Context • Drift • Dossiers │
└─────────────────────┬───────────────────────┘
                      │
                 HOLD │
                      ▼
┌─────────────────────────────────────────────┐
│             HUMAN AUTHORITY                 │
│                 Presence                    │
│                                             │
│ Passkey • Device • Biometric • Approval    │
└─────────────────────┬───────────────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│                EXECUTION                    │
│ GitHub • AWS • DB • Terraform • Stripe     │
│ Docker • Kubernetes • Enterprise APIs      │
└─────────────────────────────────────────────┘
```

That is a compelling architecture.

And Docker itself is moving into what it calls **AI Governance** around MCP Gateway; its current docs describe that capability as invite-only. ([Docker Documentation][2])

That is both validation **and a warning**.

Docker may increasingly provide native security/access-control capabilities.

So Decionis should not compete on:

> MCP server management.

Or:

> MCP credentials.

Or:

> MCP routing.

Own the harder layer:

> **Deterministic authorization of consequential execution, with cryptographic evidence and human authority when required.**

That's defensible across Docker, Claude, ChatGPT, Kubernetes, enterprise applications, and whatever agent runtime comes next.

---

## One particularly good developer demo

I would make this the Docker launch demo.

Developer tells Claude Code:

```text
Deploy the current branch to production.
```

Agent calls:

```text
terraform.apply
```

Docker Desktop suddenly shows:

```text
DECIONIS EXECUTION AUTHORITY

HIGH-RISK ACTION

Action
terraform.apply

Environment
production

Requested by
Claude Code

Policy
production-deployment-v12

Verdict
HOLD

Reason
Human authorization required for
production infrastructure changes.

[ Approve with Presence ]
```

Touch ID.

Then:

```text
✓ Presence verified

DECISION
PROCEED

Dossier
dos_01K...

Evidence
Ed25519 verified

terraform.apply resumed
```

That **explains Decionis + Presence in about 20 seconds**.

No slide deck necessary.

---

## And I would change the headline

I wouldn't use Gemini's:

> “Hardware-level authority gates...”

You're not actually providing hardware-level enforcement, and sophisticated developers will challenge that wording.

I'd use:

> **Decionis for Docker — deterministic authority before AI agents execute.**

Or even:

> **Your agents have tools. Decionis decides when they can use them.**

And underneath:

> Govern consequential MCP and containerized agent actions with deterministic policy, Presence approval, and signed Decision Dossiers.

That is very Decionis.

### Recommendation

**Build it**, but constrain the first release to perhaps **2–3 weeks**, not another major product branch:

**Phase 1:** containerize the existing Decionis MCP server → Docker MCP compatibility/catalog path → Docker Desktop Extension showing live decisions/dossiers → Presence approval for HOLD.

**Phase 2:** Dev Container Feature + `decionis govern` CLI/agent wrapper.

**Phase 3:** action-aware sidecar and broader container-runtime enforcement.

I would *not* spend time initially on hardened base images, generic HTTP interception, or deep Docker socket interception.

The strategic goal isn't really to “support Docker.” It's to get **Decionis inserted into the emerging local agent execution stack**, where developers can discover it, install it in minutes, and actually watch an agent hit an authority boundary. Given Docker's current push into MCP Toolkit/Gateway, the timing for that experiment looks unusually good. ([Docker Documentation][1])

[Docker MCP Toolkit documentation](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/?utm_source=chatgpt.com)
[Docker Extensions documentation](https://docs.docker.com/extensions/?utm_source=chatgpt.com)
[Dev Container Features](https://containers.dev/features?utm_source=chatgpt.com)

And if the P0 version works, **I would approach Docker itself rather than treating Marketplace publication as the end goal**: *“Docker provides the MCP gateway; Decionis provides deterministic execution authority.”* That partnership narrative may ultimately be more valuable than the extension itself.

