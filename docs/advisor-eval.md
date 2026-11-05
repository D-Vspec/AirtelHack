# Advisor eval

**Status (2026-11-04):** deepseek/deepseek-flash (V4.1 Flash) 27/27, meets the 90% target. openai/gpt-4o 23/27, below target. Gemini and Anthropic not run (no keys).

Run it after `make up seed` with a key in `.env`:

```bash
cd ts/services/advisor
LLM_PROVIDER=anthropic pnpm eval                          # ANTHROPIC_API_KEY set
LLM_PROVIDER=gemini LLM_MODEL=<gemini model id> pnpm eval # GOOGLE_API_KEY set
```

Each run replaces this file's section for that provider with a results table (27 conversations: 16 scripted, 11 multilingual) and writes transcripts to dist/advisor-eval-<provider>.json. Exit code 0 means a pass rate of at least 90%.

## deepseek / deepseek-flash

Run 2026-11-05T19:13:36.969Z: **27/27 conversations pass (100%)**. Per check: noIneligible 27/27, dependencies 27/27, priceMatches 27/27, language 27/27, within6 27/27, expectations 27/27.

| id | scenario | language | first quote turn | noIneligible | dependencies | priceMatches | language | within6 | expectations | pass | notes |
|---|---|---|---|---|---|---|---|---|---|---|---|
| S01 | Kirana without GSTIN wants WhatsApp/UPI | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S02 | Dental clinic, Tier 1, 6 staff | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S03 | CA firm, 8 staff, asks for managed firewall | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S04 | Tier 2 office asks for Tier 1 only fibre | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S05 | Restaurant chain, 5 sites, Tier 1 | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S06 | Manufacturer, 40 staff, honest install time | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S07 | CCTV without internet | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S08 | Only wants a mobile plan | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S09 | Too expensive -> cheaper tier with total cost | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S10 | Unknown GSTIN | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S11 | Cancelled GSTIN (K01) | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S12 | Out-of-scope opener, then normal | English | 2 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S13 | Sparse answers over several turns | English | 4 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S14 | Valid GSTIN unlocks GST products | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S15 | Recommend then accept | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S16 | 2-site business asks for SD-WAN | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M01 | Hindi kirana, no GST | Hindi | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M02 | Hinglish kirana | Hindi (romanised) | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M03 | Tamil clinic | Tamil | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M04 | Tanglish salon | Tamil (romanised) | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M05 | Bengali salon | Bengali | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M06 | Telugu coaching centre | Telugu | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M07 | Marathi bakery | Marathi | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M08 | Gujarati textile unit | Gujarati | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M09 | Kannada architecture studio | Kannada | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M10 | Punjabi sweet shop | Punjabi | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M11 | Malayalam dental clinic | Malayalam | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |

## openai / gpt-4o

Run 2026-11-04T15:48:04.905Z: **23/27 conversations pass (85%)**. Per check: noIneligible 25/27, dependencies 25/27, priceMatches 25/27, language 25/27, within6 23/27, expectations 25/27.

| id | scenario | language | first quote turn | noIneligible | dependencies | priceMatches | language | within6 | expectations | pass | notes |
|---|---|---|---|---|---|---|---|---|---|---|---|
| S01 | Kirana without GSTIN wants WhatsApp/UPI | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S02 | Dental clinic, Tier 1, 6 staff | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S03 | CA firm, 8 staff, asks for managed firewall | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S04 | Tier 2 office asks for Tier 1 only fibre | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S05 | Restaurant chain, 5 sites, Tier 1 | English | - | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | error: Failed to parse. Text: "{
  "businessType": "restaurant chain",
  "segment": "Small multi-site businesses",
  "city": "Bengaluru",
  "numSites": 5,
  "employeeCount": 40,
  "hasGstin": true,
  "currentInternet": null,
  "currentProvider": null,
  "productsOfInterest": [
    "P23"
  ],
  "language": "English",
  "romanised": false,
  "intent": "info"
}". Error: [{"code":"invalid_value","values":["Business broadband","Home broadband","Leased line","Mixed per site","Mobile hotspot"],"path":["currentInternet"],"message":"Invalid option: expected one of \"Business broadband\"/\"Home broadband\"/\"Leased line\"/\"Mixed per site\"/\"Mobile hotspot\""},{"code":"invalid_value","values":["Accel","Accel (mobile only)","Competitor A","Competitor B","Local ISP"],"path":["currentProvider"],"message":"Invalid option: expected one of \"Accel\"/\"Accel (mobile only)\"/\"Competitor A\"/\"Competitor B\"/\"Local ISP\""}]

Troubleshooting URL: https://docs.langchain.com/oss/javascript/langchain/errors/OUTPUT_PARSING_FAILURE/
 |
| S06 | Manufacturer, 40 staff, honest install time | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S07 | CCTV without internet | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S08 | Only wants a mobile plan | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S09 | Too expensive -> cheaper tier with total cost | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S10 | Unknown GSTIN | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S11 | Cancelled GSTIN (K01) | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S12 | Out-of-scope opener, then normal | English | 2 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S13 | Sparse answers over several turns | English | 4 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S14 | Valid GSTIN unlocks GST products | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S15 | Recommend then accept | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| S16 | 2-site business asks for SD-WAN | English | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M01 | Hindi kirana, no GST | Hindi | - | ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | ✗ |  |
| M02 | Hinglish kirana | Hindi (romanised) | - | ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | ✗ |  |
| M03 | Tamil clinic | Tamil | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M04 | Tanglish salon | Tamil (romanised) | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M05 | Bengali salon | Bengali | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M06 | Telugu coaching centre | Telugu | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M07 | Marathi bakery | Marathi | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M08 | Gujarati textile unit | Gujarati | - | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | error: Failed to parse. Text: "{
  "businessType": "Textile unit",
  "segment": "Small manufacturers",
  "city": "Surat",
  "employeeCount": 30,
  "numSites": 1,
  "hasGstin": true,
  "currentInternet": "unknown",
  "currentProvider": "unknown",
  "productsOfInterest": [
    "P24"
  ],
  "language": "Gujarati",
  "romanised": false,
  "intent": "info"
}". Error: [{"code":"invalid_value","values":["Business broadband","Home broadband","Leased line","Mixed per site","Mobile hotspot"],"path":["currentInternet"],"message":"Invalid option: expected one of \"Business broadband\"/\"Home broadband\"/\"Leased line\"/\"Mixed per site\"/\"Mobile hotspot\""},{"code":"invalid_value","values":["Accel","Accel (mobile only)","Competitor A","Competitor B","Local ISP"],"path":["currentProvider"],"message":"Invalid option: expected one of \"Accel\"/\"Accel (mobile only)\"/\"Competitor A\"/\"Competitor B\"/\"Local ISP\""}]

Troubleshooting URL: https://docs.langchain.com/oss/javascript/langchain/errors/OUTPUT_PARSING_FAILURE/
 |
| M09 | Kannada architecture studio | Kannada | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M10 | Punjabi sweet shop | Punjabi | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| M11 | Malayalam dental clinic | Malayalam | 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
