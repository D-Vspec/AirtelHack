"""Lead propensity model (Phase 7).

Trains XGBoost on features known when a lead is created, compares it with a logistic regression and
a segment-average baseline on a time-based split, and exports everything the Go scoring service needs:

  go/pkg/scoring/model.json            encoding spec + trees (with node values for reasons) + calibration
  go/pkg/scoring/testdata/parity.json  raw leads and the probability Python computes, for the Go parity test
  docs/model-eval.md                   generated metrics tables (docs/model-card.md explains them)

Run:  ml/.venv/Scripts/python ml/train.py   (or: make model)
"""
from __future__ import annotations

import json
import math
from pathlib import Path

import numpy as np
import pandas as pd
import xgboost as xgb
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import average_precision_score, brier_score_loss, roc_auc_score
from sklearn.preprocessing import StandardScaler

ROOT = Path(__file__).resolve().parents[1]
SEED = 7
rng = np.random.default_rng(SEED)

# ---- features known at lead creation (contracts/profile.schema.json names) -------------------
# Leakage excluded on purpose (docs/data-dictionary.md): stage_reached, outcome, drop_reason, every
# *_date after created_date, days_lead_to_activation, customer_follow_up_calls, bundle_products,
# monthly_bundle_value_inr, one_time_setup_inr, contract_months, churned_within_6_months,
# nps_score, acquisition_cost_inr. Identity columns (names, GSTIN, Udyam number, contact) are not
# features either.
CATEGORICAL = {
    "leadSource": "lead_source",
    "segment": "segment",
    "preferredLanguage": "preferred_language",
    "currentInternet": "current_internet",
    "currentProvider": "current_provider",
}
TURNOVER = ["< 40 lakh", "40 lakh - 1.5 cr", "1.5 - 5 cr", "5 - 25 cr", "25 - 100 cr"]
PRODUCTS = [f"P{i:02d}" for i in range(1, 26)]


def raw_leads(df: pd.DataFrame) -> list[dict]:
    """CSV rows -> the raw lead shape the Go service receives (profile field names)."""
    out = []
    for r in df.itertuples(index=False):
        out.append({
            "leadSource": r.lead_source, "segment": r.segment, "preferredLanguage": r.preferred_language,
            "currentInternet": r.current_internet, "currentProvider": r.current_provider,
            "cityTier": int(r.city_tier), "employeeCount": int(r.employee_count), "numSites": int(r.num_sites),
            "annualTurnoverBand": r.annual_turnover_band, "hasGstin": r.has_gstin == "Y", "hasUdyam": r.has_udyam == "Y",
            "monthlyTelecomSpendInr": float(r.monthly_telecom_spend_inr),
            "productsOfInterest": [p for p in str(r.products_of_interest).split(";") if p in PRODUCTS],
        })
    return out


def build_spec(train: pd.DataFrame) -> list[dict]:
    """Feature columns, in order. Categories come from the training months only."""
    spec = []
    for field, col in CATEGORICAL.items():
        for v in sorted(train[col].unique()):
            spec.append({"name": f"{field}={v}", "kind": "onehot", "field": field, "value": v})
    for field, default in [("cityTier", 2), ("employeeCount", float(train.employee_count.median())), ("numSites", 1),
                           ("monthlyTelecomSpendInr", float(train.monthly_telecom_spend_inr.median()))]:
        spec.append({"name": field, "kind": "num", "field": field, "default": default})
    spec.append({"name": "annualTurnoverBand", "kind": "ordinal", "field": "annualTurnoverBand", "levels": TURNOVER, "default": 1})
    spec.append({"name": "hasGstin", "kind": "bool", "field": "hasGstin", "default": 0})
    spec.append({"name": "hasUdyam", "kind": "bool", "field": "hasUdyam", "default": 0})
    spec.append({"name": "interestCount", "kind": "count", "field": "productsOfInterest"})
    for p in PRODUCTS:
        spec.append({"name": f"interest={p}", "kind": "interest", "field": "productsOfInterest", "value": p})
    return spec


def encode(lead: dict, spec: list[dict]) -> list[float]:
    """The one encoder: training uses it and the Go service re-implements it (go/pkg/scoring)."""
    x = []
    for f in spec:
        v = lead.get(f["field"])
        k = f["kind"]
        if k == "onehot":
            x.append(1.0 if v == f["value"] else 0.0)
        elif k == "num":
            x.append(float(v) if v is not None else float(f["default"]))
        elif k == "ordinal":
            x.append(float(f["levels"].index(v)) if v in f["levels"] else float(f["default"]))
        elif k == "bool":
            x.append(1.0 if v is True else 0.0 if v is False else float(f["default"]))
        elif k == "count":
            x.append(float(len(v or [])))
        elif k == "interest":
            x.append(1.0 if f["value"] in (v or []) else 0.0)
    return x


# ---- metrics ---------------------------------------------------------------------------------
def top_decile_lift(y, p):
    n = max(1, int(round(len(y) * 0.1)))
    idx = np.argsort(-p, kind="stable")[:n]
    return float(np.mean(y[idx]) / np.mean(y))


def metrics(y, p):
    return {"pr_auc": float(average_precision_score(y, p)), "roc_auc": float(roc_auc_score(y, p)),
            "top_decile_lift": top_decile_lift(y, p), "brier": float(brier_score_loss(y, p))}


def bootstrap_diff(y, a, b, n=1000):
    """95% interval of PR-AUC(a) - PR-AUC(b) over bootstrap resamples of the test set."""
    diffs = []
    for _ in range(n):
        i = rng.integers(0, len(y), len(y))
        if y[i].sum() == 0:
            continue
        diffs.append(average_precision_score(y[i], a[i]) - average_precision_score(y[i], b[i]))
    return float(np.percentile(diffs, 2.5)), float(np.percentile(diffs, 97.5))


def calibration_table(y, p, bins=10):
    order = np.argsort(p, kind="stable")
    rows = []
    for chunk in np.array_split(order, bins):
        rows.append((float(p[chunk].mean()), float(y[chunk].mean()), len(chunk)))
    return rows


# ---- tree export (Go evaluates these; node values give per-feature contributions) -------------
def export_trees(booster: xgb.Booster, names: list[str]) -> list[dict]:
    trees = []
    for dump in booster.get_dump(dump_format="json", with_stats=True):
        root = json.loads(dump)
        nodes: dict[int, dict] = {}

        def walk(n) -> tuple[float, float]:
            """Returns (value, cover): leaf value, or the cover-weighted mean of the leaves below."""
            if "leaf" in n:
                nodes[n["nodeid"]] = {"leaf": n["leaf"], "v": n["leaf"]}
                return n["leaf"], n["cover"]
            vy, cy = walk(next(c for c in n["children"] if c["nodeid"] == n["yes"]))
            vn, cn = walk(next(c for c in n["children"] if c["nodeid"] == n["no"]))
            v = (vy * cy + vn * cn) / (cy + cn)
            nodes[n["nodeid"]] = {"f": names.index(n["split"]), "t": n["split_condition"], "yes": n["yes"], "no": n["no"],
                                  "miss": n["missing"], "v": v}
            return v, cy + cn

        walk(root)
        trees.append({"nodes": [nodes[i] | {"id": i} for i in sorted(nodes)]})
    return trees


def margin_of(trees, base_margin, x):
    m = base_margin
    for t in trees:
        nodes = {n["id"]: n for n in t["nodes"]}
        n = nodes[0]
        while "leaf" not in n:
            n = nodes[n["yes"] if x[n["f"]] < n["t"] else n["no"]]
        m += n["leaf"]
    return m


def sigmoid(z):
    return 1.0 / (1.0 + np.exp(-z))


def main():
    df = pd.read_csv(ROOT / "data/smb_leads.csv", dtype=str, keep_default_na=False)
    for c in ["city_tier", "employee_count", "num_sites", "monthly_telecom_spend_inr"]:
        df[c] = pd.to_numeric(df[c])
    df["y"] = (df.outcome == "Activated").astype(int)
    df = df.sort_values("created_date", kind="stable")

    # time-based split: train on the earliest weeks, calibrate/stop on late May, test on June
    train = df[df.created_date < "2026-05-16"]
    valid = df[(df.created_date >= "2026-05-16") & (df.created_date < "2026-06-01")]
    test = df[df.created_date >= "2026-06-01"]
    spec = build_spec(train)
    names = [f["name"] for f in spec]
    X = {k: np.array([encode(l, spec) for l in raw_leads(d)]) for k, d in [("train", train), ("valid", valid), ("test", test)]}
    Y = {k: d.y.to_numpy() for k, d in [("train", train), ("valid", valid), ("test", test)]}

    # XGBoost: the 7.4% positive rate is handled with scale_pos_weight, then Platt calibration on
    # the validation weeks brings the probabilities back to the real base rate.
    pos_weight = float((Y["train"] == 0).sum() / Y["train"].sum())
    clf = xgb.XGBClassifier(n_estimators=800, max_depth=3, learning_rate=0.03, subsample=0.8, colsample_bytree=0.8,
                            min_child_weight=5, reg_lambda=2.0, scale_pos_weight=pos_weight, eval_metric="aucpr",
                            early_stopping_rounds=60, random_state=SEED, n_jobs=4)
    clf.fit(pd.DataFrame(X["train"], columns=names), Y["train"], eval_set=[(pd.DataFrame(X["valid"], columns=names), Y["valid"])], verbose=False)
    best = clf.best_iteration + 1
    booster = clf.get_booster()[:best]
    dm = {k: xgb.DMatrix(pd.DataFrame(X[k], columns=names)) for k in X}
    margin = {k: booster.predict(dm[k], output_margin=True) for k in X}
    platt = LogisticRegression(C=1e6).fit(margin["valid"].reshape(-1, 1), Y["valid"])
    a, b = float(platt.coef_[0][0]), float(platt.intercept_[0])
    p_xgb = sigmoid(a * margin["test"] + b)
    p_xgb_raw = sigmoid(margin["test"])

    # baseline 1: segment-average conversion from the training weeks
    seg_rate = train.groupby("segment").y.mean()
    p_seg = test.segment.map(seg_rate).fillna(train.y.mean()).to_numpy()
    # baseline 2: logistic regression on the same features, class-balanced, Platt-calibrated the same way
    sc = StandardScaler().fit(X["train"])
    lr = LogisticRegression(max_iter=5000, class_weight="balanced", C=0.5).fit(sc.transform(X["train"]), Y["train"])
    lr_margin = {k: lr.decision_function(sc.transform(X[k])) for k in X}
    lr_platt = LogisticRegression(C=1e6).fit(lr_margin["valid"].reshape(-1, 1), Y["valid"])
    p_lr = lr_platt.predict_proba(lr_margin["test"].reshape(-1, 1))[:, 1]

    yt = Y["test"]
    res = {"xgboost": metrics(yt, p_xgb), "logistic_regression": metrics(yt, p_lr), "segment_average": metrics(yt, p_seg)}
    res["xgboost_uncalibrated_brier"] = float(brier_score_loss(yt, p_xgb_raw))
    ci_lr, ci_seg = bootstrap_diff(yt, p_xgb, p_lr), bootstrap_diff(yt, p_xgb, p_seg)

    # export: trees, base margin, calibration, encoding spec
    trees = export_trees(booster, names)
    cfg = json.loads(booster.save_config())
    base_score = float(str(cfg["learner"]["learner_model_param"]["base_score"]).strip("[]"))
    base_margin = math.log(base_score / (1 - base_score))
    # parity inside Python first: our tree walk must equal xgboost's margin
    for i in range(0, len(X["test"]), 97):
        assert abs(margin_of(trees, base_margin, X["test"][i]) - margin["test"][i]) < 1e-4, "tree export mismatch"

    model = {
        "version": f"xgb-{xgb.__version__}-{best}trees-trained-{train.created_date.min()}..{train.created_date.max()}",
        "baseMargin": base_margin, "calibration": {"a": a, "b": b}, "spec": spec, "trees": trees,
        "baseRate": float(df.y.mean()),
        "deciles": [float(np.quantile(sigmoid(a * margin["train"] + b), q)) for q in np.linspace(0.1, 0.9, 9)],
    }
    out = ROOT / "go/pkg/scoring"
    (out / "testdata").mkdir(parents=True, exist_ok=True)
    (out / "model.json").write_text(json.dumps(model, separators=(",", ":")), encoding="utf-8")
    raws = raw_leads(test)
    parity = [{"lead": raws[i], "probability": float(p_xgb[i])} for i in range(0, len(raws), 34)]
    (out / "testdata/parity.json").write_text(json.dumps(parity, indent=1, ensure_ascii=False), encoding="utf-8")

    # importance by original field (gain summed over one-hot / interest columns)
    gain = booster.get_score(importance_type="total_gain")
    by_field: dict[str, float] = {}
    for f in spec:
        key = f["field"] if f["kind"] in ("onehot", "interest") else f["name"]
        by_field[key] = by_field.get(key, 0.0) + gain.get(f["name"], 0.0)
    total = sum(by_field.values()) or 1.0
    importance = sorted(((k, v / total) for k, v in by_field.items()), key=lambda kv: -kv[1])

    # sanity checks named in prompts/context/phase7.md
    rates = {
        "referral": float(df[df.lead_source == "Referral"].y.mean()), "whatsapp": float(df[df.lead_source == "WhatsApp campaign"].y.mean()),
        "website": float(df[df.lead_source == "Website form"].y.mean()), "micro": float(df[df.segment == "Micro retail & services"].y.mean()),
        "gstin": float(df[df.has_gstin == "Y"].y.mean()), "no_gstin": float(df[df.has_gstin == "N"].y.mean()),
    }

    lines = ["# Lead propensity model: evaluation", "",
             "Generated by `ml/train.py`; see docs/model-card.md for what it means. Do not edit by hand.", "",
             f"Split by created_date: train {train.created_date.min()}..{train.created_date.max()} ({len(train)} leads, {int(Y['train'].sum())} activated), "
             f"validation {valid.created_date.min()}..{valid.created_date.max()} ({len(valid)}, {int(Y['valid'].sum())}), "
             f"test {test.created_date.min()}..{test.created_date.max()} ({len(test)}, {int(yt.sum())}). Test base rate {yt.mean():.3f}.", "",
             f"XGBoost: {best} trees (early stopping on validation PR-AUC), depth 3, scale_pos_weight {pos_weight:.1f}, Platt-calibrated on validation.", "",
             "## Test month (June)", "", "| model | PR-AUC | ROC-AUC | top-decile lift | Brier |", "|---|---|---|---|---|"]
    for k, m in res.items():
        if isinstance(m, dict):
            lines.append(f"| {k} | {m['pr_auc']:.3f} | {m['roc_auc']:.3f} | {m['top_decile_lift']:.2f} | {m['brier']:.4f} |")
    lines += ["", f"Random ranking would give PR-AUC ≈ {yt.mean():.3f} and lift 1.00. Uncalibrated XGBoost Brier: {res['xgboost_uncalibrated_brier']:.4f} (scale_pos_weight inflates probabilities; Platt scaling fixes that).", "",
              f"PR-AUC difference, XGBoost minus logistic regression: 95% bootstrap interval [{ci_lr[0]:+.3f}, {ci_lr[1]:+.3f}].",
              f"PR-AUC difference, XGBoost minus segment average: 95% bootstrap interval [{ci_seg[0]:+.3f}, {ci_seg[1]:+.3f}].", "",
              "## Calibration on the test month (XGBoost, deciles of predicted probability)", "", "| mean predicted | observed rate | leads |", "|---|---|---|"]
    for mp, ob, n in calibration_table(yt, p_xgb):
        lines.append(f"| {mp:.3f} | {ob:.3f} | {n} |")
    lines += ["", "## Feature importance (share of total gain, grouped by input field)", "", "| field | share |", "|---|---|"]
    for k, v in importance[:15]:
        lines.append(f"| {k} | {v:.3f} |")
    lines += ["", "## Sanity checks against the raw data (all 5,000 leads)", "",
              f"Referral {rates['referral']:.3f}, WhatsApp campaign {rates['whatsapp']:.3f}, website form {rates['website']:.3f}; "
              f"micro retail {rates['micro']:.3f}; GSTIN {rates['gstin']:.3f} vs none {rates['no_gstin']:.3f}.", ""]
    (ROOT / "docs/model-eval.md").write_text("\n".join(lines), encoding="utf-8")
    json.dump({"metrics": res, "ci_vs_lr": ci_lr, "ci_vs_segment": ci_seg, "importance": importance, "trees": best},
              open(ROOT / "ml/metrics.json", "w"), indent=1)
    print("\n".join(lines))


if __name__ == "__main__":
    main()
