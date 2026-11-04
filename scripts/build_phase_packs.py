#!/usr/bin/env python3
"""Build dist/phaseN-<slug>.zip prompt packs for phases 1-8: PROMPT.md, CONTEXT.md, ACCEPTANCE.md."""
import json, pathlib, re, zipfile
import pandas as pd

root = pathlib.Path(__file__).resolve().parent.parent
phases = [p for p in json.load(open(root / "scripts" / "phases.json")) if p["n"] >= 1]
pre = (root / "prompts" / "00-preamble.md").read_text().strip()
dist = root / "dist"; dist.mkdir(exist_ok=True)
cat = pd.read_csv(root / "data" / "product_catalogue.csv")

def catalogue_table(cols):
    sub = cat[cols].fillna("-")
    head = "| " + " | ".join(cols) + " |\n|" + "---|" * len(cols)
    rows = ["| " + " | ".join(str(v) for v in r) + " |" for r in sub.values.tolist()]
    return head + "\n" + "\n".join(rows)

extra = {
    2: "\n\n## Catalogue rules (generated from data/product_catalogue.csv)\n`requires_one_of` uses `|` for any-of. `available_city_tiers` and `target_segments` use `;` separators. Prices are INR per month per the `unit` column.\n\n"
       + catalogue_table(["product_id", "product_name", "unit", "price_min_inr_per_month", "price_max_inr_per_month", "one_time_setup_inr", "min_contract_months", "available_city_tiers", "min_employees", "min_sites", "requires_gstin", "requires_one_of", "target_segments"]),
    5: "\n\n## Install durations (generated from data/product_catalogue.csv)\n\n" + catalogue_table(["product_id", "product_name", "typical_install_days"]),
}

for ph in phases:
    n, slug = ph["n"], ph["slug"]
    prompt = (root / "prompts" / f"phase{n}-{slug}.md").read_text().strip()
    ctx_path = root / "prompts" / "context" / f"phase{n}.md"
    ctx = ctx_path.read_text().strip() if ctx_path.exists() else f"# Context for Phase {n}\nSee docs/PLAN.md, docs/scenarios.md and docs/data-dictionary.md."
    ctx += extra.get(n, "")
    done = re.findall(r"^Done when:.*$", prompt, flags=re.M)
    accept = (f"# Acceptance checklist - Phase {n} ({slug})\n\n- [ ] " + (done[0] if done else "Done when: the goal above is met") +
              "\n- [ ] `make up` and `make smoke` still pass\n- [ ] `make test` and `make typecheck` pass\n- [ ] New or changed events and shapes are in /contracts and `make contracts` was run\n"
              "- [ ] Tests added for every behaviour this phase introduces\n- [ ] docs/CHANGELOG.md has an entry that says what was verified and what was not\n"
              f"- [ ] `make package PHASE={n}` produced dist/phase{n}-{slug}-output.zip\n")
    howto = (f"# Phase {n} - {slug}\n\nHow to use this pack:\n1. Open the udyam-digital repo (Phase {n-1} output) in your coding agent.\n2. Paste PROMPT.md as the task. Give the agent CONTEXT.md as reference.\n"
             f"3. Work until every box in ACCEPTANCE.md is ticked.\n4. Run `make package PHASE={n}` and keep the zip.\n")
    out = dist / f"phase{n}-{slug}.zip"
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("README.md", howto)
        z.writestr("PROMPT.md", pre + "\n\n---\n\n" + prompt + "\n")
        z.writestr("CONTEXT.md", ctx + "\n")
        z.writestr("ACCEPTANCE.md", accept)
    print("wrote", out.name)
