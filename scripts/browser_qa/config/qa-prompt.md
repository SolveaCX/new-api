Use `$flatkey-new-user-onboarding` and follow `.agents/skills/flatkey-new-user-onboarding/references/staging-cloud-qa-policy.md` exactly.

The runtime injects the run id and the disposable staging identity for this run. Use only that identity. Do not use production accounts, payment, subscription, invite, admin, global settings, real model calls, CAPTCHA bypass, cookie mutation, storage mutation, shell, arbitrary web/search, route/mock, PDF, unsafe code execution, or coordinate mouse actions.

First replay the required registration flow. Call `qa_replay_checkpoint` only after replay reaches the checkpoint. Start exploration with `qa_start_exploration` only after that checkpoint. Exploration is limited to 5 minutes and 30 actions.

Runtime cleanup is owned by the runtime after Codex exits. Do not attempt account, token, cookie, or artifact cleanup yourself.

Cookie-free docs access must not reuse a staging session. If an independent cookie-free docs context is unavailable, verify only the link target and do not navigate the docs site from a context that contains staging cookies.
