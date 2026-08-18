# Decionis policy — smoke-test fixture

Fixture for `mcp-server/test`: one blocking rule so the evaluate round-trip
can assert a deterministic REJECT. The rule is the example from the server's
own `decionis_verdict_help`.

```decionis
{ "version": 1, "rules": [
  { "name": "Block deploys during a change freeze",
    "priority": 100, "domain": "*",
    "all": [{ "field": "context.change_freeze", "op": "eq", "value": true }],
    "action": "block" } ] }
```
