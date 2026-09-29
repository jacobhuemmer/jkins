# Jenkins CLI parity

Snapshot: `jenkins help` on `jenkins.example.com` (Jenkins 2.462.3), 2026-09-29. It advertised 72 commands. Jenkins plugins can add or remove commands, so run `jkins commands` for the current controller's list and `jkins help COMMAND` for its current syntax.

`jkins` implements the Jenkins WebSocket CLI protocol in Go at `/cli/ws`: command name and arguments, UTF-8 encoding, stdin, stdout, stderr, and exit status. It uses the private `jkins` vault for HTTP Basic authentication during the WebSocket handshake. It does not invoke `jenkins-cli.jar` or 1Password. All commands below use that call path; there is no separate REST endpoint implementation for each command. The existing structured `job` and `build get|log|queue` operations still use Jenkins' HTTP API.

Protocol references: [JEP-222](https://github.com/jenkinsci/jep/blob/master/jep/222/README.adoc), [Jenkins 2.462.3 `PlainCLIProtocol`](https://github.com/jenkinsci/jenkins/blob/jenkins-2.462.3/cli/src/main/java/hudson/cli/PlainCLIProtocol.java), [its WebSocket endpoint](https://github.com/jenkinsci/jenkins/blob/jenkins-2.462.3/core/src/main/java/hudson/cli/CLIAction.java), and [the Java client](https://github.com/jenkinsci/jenkins/blob/jenkins-2.462.3/cli/src/main/java/hudson/cli/CLI.java).

Source-level parity proof: the Java client sends command arguments, encoding, locale, and stdin through `PlainCLIProtocol`; the WebSocket endpoint collects them, calls `CLICommand.clone(commandName)`, and invokes `command.main(...)` with the streams and authenticated identity. [`CLICommand.clone`](https://github.com/jenkinsci/jenkins/blob/jenkins-2.462.3/core/src/main/java/hudson/cli/CLICommand.java) resolves both core and plugin commands from the same extension registry. `jkins` now sends the same frame types to that endpoint, so command implementations and their option parsing stay server-side and are shared by both clients. For example, [`BuildCommand`](https://github.com/jenkinsci/jenkins/blob/jenkins-2.462.3/core/src/main/java/hudson/cli/BuildCommand.java) handles `-f` and `-s` interruption differently on the server. This source review removes the need to reimplement or run production writes for each command to establish dispatch parity.

The Go transport and argument/stream behavior have local fake-controller tests. A checked box means a read-only command was also verified against the live controller. Open boxes are optional command-specific integration checks; they do not block source-based dispatch parity or mean the command is unavailable in Go. Live writes, builds, deployments, and administrator commands have not been run.

Read-only differential check on 2026-09-29: the installed `jenkins` client and `jkins` returned byte-identical stdout, stderr, and exit codes for `help`, `help build`, `version`, and `list-jobs Team`. The installed launcher and `jkins` authenticate as different principals (`who-am-i` differs), so this is a transport check, not a complete permissions-sensitive comparison. Identifiers, job names, and permission output were not recorded in the comparison result.

Verification rule for optional command-specific checks: run the official client and `jkins` against the same controller version, plugin set, credential, command arguments, and stdin. Compare stdout, stderr, exit status, and—when a command mutates Jenkins—the resulting server state. Use a disposable controller and fixtures for writes, build triggers, Groovy, plugin changes, credentials, and administrator actions. Transport edge cases below remain the priority because every command shares the same server dispatch.

- [ ] `add-job-to-view`
- [ ] `apply-configuration`
- [ ] `build` — verify parameters, wait/follow, stdin and interrupt behavior with a disposable job before claiming full behavioral parity.
- [ ] `cancel-quiet-down`
- [ ] `check-configuration`
- [ ] `clear-queue`
- [ ] `connect-node`
- [ ] `console` — verify a specific build, `lastBuild`, `-n`, and `-f`.
- [ ] `copy-job`
- [ ] `create-credentials-by-xml`
- [ ] `create-credentials-domain-by-xml`
- [ ] `create-job`
- [ ] `create-node`
- [ ] `create-view`
- [ ] `declarative-linter`
- [ ] `delete-builds`
- [ ] `delete-credentials`
- [ ] `delete-credentials-domain`
- [ ] `delete-job`
- [ ] `delete-node`
- [ ] `delete-view`
- [ ] `disable-job`
- [ ] `disable-plugin`
- [ ] `disconnect-node`
- [ ] `enable-job`
- [ ] `enable-plugin`
- [ ] `export-configuration`
- [ ] `get-credentials-as-xml`
- [ ] `get-credentials-domain-as-xml`
- [ ] `get-job`
- [ ] `get-node`
- [ ] `get-view`
- [ ] `groovy`
- [ ] `groovysh`
- [x] `help` — list and `help build` verified live.
- [ ] `import-credentials-as-xml`
- [ ] `install-plugin`
- [ ] `keep-build`
- [ ] `list-changes`
- [ ] `list-credentials`
- [ ] `list-credentials-as-xml`
- [ ] `list-credentials-context-resolvers`
- [ ] `list-credentials-providers`
- [x] `list-jobs` — folder listing verified live.
- [ ] `list-plugins`
- [ ] `mail`
- [ ] `offline-node`
- [ ] `online-node`
- [ ] `quiet-down`
- [ ] `reload-configuration`
- [ ] `reload-jcasc-configuration`
- [ ] `reload-job`
- [ ] `remove-job-from-view`
- [ ] `replay-pipeline`
- [ ] `restart`
- [ ] `restart-from-stage`
- [ ] `safe-restart`
- [ ] `safe-shutdown`
- [ ] `session-id`
- [ ] `set-build-description`
- [ ] `set-build-display-name`
- [ ] `shutdown`
- [ ] `stop-builds`
- [ ] `update-credentials-by-xml`
- [ ] `update-credentials-domain-by-xml`
- [ ] `update-job`
- [ ] `update-node`
- [ ] `update-view`
- [x] `version` — returned `2.462.3` live.
- [ ] `wait-node-offline`
- [ ] `wait-node-online`
- [x] `who-am-i` — authenticated identity and permissions returned live; output was not recorded.

Further transport TODO:

- [x] Test WebSocket connection loss during stdin streaming and a simulated `-f` follow; both return an error without an EXIT frame, and already received output is preserved.
- [ ] Test interrupt semantics against a disposable non-deployment job; `-s` must forward interruption, while `-f` must leave the build running.
- [ ] Compare locale-sensitive output across platforms; Go sends UTF-8 and the locale from `LC_ALL`, `LC_MESSAGES`, or `LANG` when available.
- [ ] Test plugin commands that consume stdin or produce binary output with controller-approved fixtures.
- [ ] Reconcile this inventory when Jenkins or plugins change. The live `help` command is authoritative.

Direct `jkins build JOB` follows Jenkins CLI semantics and queues immediately. Structured `jkins build queue JOB` retains its preview and `--execute` gate. For a name collision, `jkins --jenkins-command help` or `jkins --jenkins-command build get` forces the Jenkins CLI protocol path.
