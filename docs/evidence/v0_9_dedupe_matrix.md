# v0.9 dedupe matrix — pairwise over 28 live L2 rows

Computed 2026-10-25 over all `L2_STRATEGY` nodes in the live `.ace-sandbox/ace.db`
(after the supervisor-mediated rollback restore of `ace-20261025-1617.db`).

Metrics per pair: `jac` = word-3-shingle Jaccard on title+directive,
`art` = artefact-set overlap (`[a-z0-9_]+\.(py|json)` + `c_s\d+_[a-z]+`).
Pairs >= 0.3 on either metric listed; all others < 0.3 on both.

| pair | jac | art | shared artefacts |
|---|---|---|---|
| S16–S21 | 0.00 | 1.00 | c_s13_data |
| S16–S23 | 0.00 | 1.00 | c_s13_data |
| S16–S24 | 0.00 | 1.00 | c_s13_data |
| S18–S27 | 0.04 | 1.00 | c_s13_data.py |
| S21–S23 | 0.07 | 1.00 | c_s13_data |
| S21–S24 | 0.10 | 1.00 | c_s13_data |
| S23–S24 | 0.03 | 1.00 | c_s13_data |
| S13–S18 | 0.00 | 0.50 | c_s13_data.py |
| S13–S27 | 0.00 | 0.50 | c_s13_data.py |
| S14–S16 | 0.00 | 0.50 | c_s13_data |
| S14–S21 | 0.00 | 0.50 | c_s13_data |
| S14–S23 | 0.00 | 0.50 | c_s13_data |
| S14–S24 | 0.00 | 0.50 | c_s13_data |
| S16–S28 | 0.00 | 0.50 | c_s13_data |
| S21–S28 | 0.28 | 0.50 | c_s13_data |
| S23–S28 | 0.04 | 0.50 | c_s13_data |
| S24–S28 | 0.14 | 0.50 | c_s13_data |
| S14–S28 | 0.00 | 0.33 | c_s13_data |

## Verdict

- **No false positives at 0.5 artefact overlap.** Every pair at >= 0.5 shares
  the same target module (`c_s13_data`/`c_s13_data.py`) and is the same
  corpus-widening strategy the owner audit flagged — S16/S21/S23/S24 are
  identical work (art = 1.0). Nothing unrelated shares artefacts.
- **Word shingles alone would have caught nothing**: max `jac` in the corpus
  family is 0.28 — the 0.8 threshold explains why 11 same-goal branches
  coexisted. The artefact signal is what makes `DuplicateOf` work.
- S28 falls to 0.33–0.5 vs the family (wider artefact set); still flagged vs
  S14/S16/S21/S23/S24 at exactly 0.5.
