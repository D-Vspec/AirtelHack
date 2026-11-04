# Advisor eval

**Status: not run yet.** The build machine has no LLM API key, so no provider has been scored. Nothing below is a result.

Run it after `make up seed` with a key in `.env`:

```bash
cd ts/services/advisor
LLM_PROVIDER=anthropic pnpm eval                          # ANTHROPIC_API_KEY set
LLM_PROVIDER=gemini LLM_MODEL=<gemini model id> pnpm eval # GOOGLE_API_KEY set
```

Each run replaces this file's section for that provider with a results table (27 conversations: 16 scripted, 11 multilingual) and writes transcripts to dist/advisor-eval-<provider>.json. Exit code 0 means a pass rate of at least 90%.
