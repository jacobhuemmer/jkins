---
name: jkins-read
description: Find Jenkins jobs and inspect builds or console logs with the jkins CLI or its MCP read tool. Use for Jenkins discovery, status, and log requests.
---

# Read Jenkins with jkins

Check `jkins auth status` before authenticated reads. If no credential is stored, ask the user to run `jkins auth login` in a terminal; it prompts for the API token without echoing it. Controller setup is in the repository's `README.md`.

Use structured commands when JSON helps:

```sh
jkins job list --filter Deploy
jkins job list --path Team
jkins job get Team/Job
jkins build get Team/Job 42
jkins build log Team/Job 42
```

`job list` requires a filter or folder path; use it to find the exact job path. `build get` and `build log` require a positive build number. To identify a recent build, read `job get` first and use the returned `last_build.number`. Add `--human` only when readable text is useful; JSON is the default.

For server CLI reads, use `jkins commands` to see the controller's current commands and `jkins help COMMAND` for syntax. For example, `jkins list-jobs Team` lists a folder and `jkins console Team/Job 42 -n 200` reads a specific build. If the user explicitly wants the current last build, pass `lastBuild` to `console`.

When an MCP connection to `jkins mcp serve` is available, `jkins_read` offers `job_list` (exactly one of `filter` or `path`), `job_get` (`path`), `build_get` (`path`, positive `number`), and `build_log` (`path`, positive `number`). `jkins_status` checks stored credential presence. MCP does not expose native server CLI commands.

Console output can contain secrets from jobs or plugins. Return only the log content needed to answer the request.
