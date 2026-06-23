# Mobile Skill Workflow

This document shows how an EFP skill can orchestrate `mobile` from a natural-language test scenario without BrowserStack AI or MCP.

1. Discover the CLI contract.

```bash
mobile commands --json
mobile schema run.start --json
mobile schema observe --json
mobile schema locate --json
```

2. Start the run.

Public app:

```bash
mobile run start --file ./app.apk --platform android --network public --project EFP --build smoke --name login-smoke --json
```

Private app:

```bash
mobile run start --file ./app.apk --platform android --network private-managed --project EFP --build smoke --name private-login --json
```

3. Observe and choose a target.

```bash
mobile observe --run-id run-... --json
mobile locate --run-id run-... --role button --name Login --actionable --json
```

4. Act only with latest refs.

```bash
mobile tap --run-id run-... --ref obs-...:e17 --json
mobile observe --run-id run-... --json
mobile type --run-id run-... --ref obs-...:e21 --text-env TEST_PASSWORD --json
```

5. Assert state and wait for stability when needed.

```bash
mobile wait stable --run-id run-... --timeout 30s --poll-interval 1s --json
mobile assert visible --run-id run-... --name Dashboard --json
```

6. Handoff to a human when manual inspection is needed.

```bash
mobile run handoff --run-id run-... --hold-for 10m --json
mobile run resume --run-id run-... --json
```

7. Finish and collect evidence.

```bash
mobile artifact collect --run-id run-... --json
mobile run finish --run-id run-... --status passed --collect-artifacts --json
```

Skill authors should branch on `error.code`, not message text. Recover from `stale_observation` by observing again, from `ambiguous_element` by adding stable semantic criteria, from `control_locked` by resuming after human handoff, and from `capacity_wait_timeout` by retrying later or reducing required capacity.

