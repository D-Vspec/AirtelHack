# Context for Phase 3
- Languages in the data: English, Hindi, Bengali, Telugu, Marathi, Tamil, Gujarati, Kannada, Malayalam, Punjabi.
- Model choice is empirical. In our research Gemini topped an independent Hindi index with Claude close behind, Sarvam is India-first (speech, OCR, code-mixing) but its own benchmarks are self-reported, and Tamil is the weakest language for most models. Run the multilingual eval on at least two providers and pick by results; use Sarvam only for voice or WhatsApp stretch.
- ts/services/advisor/src/llm.ts already switches providers. Gemini and OpenAI need LLM_MODEL set to a current model id.
- The advisor must call the rules and quote services over HTTP (http://rules:8082, http://quote:8083 in compose). It may only recommend products those tools return.
- Data on why owners drop (for conversation design): the biggest fixable losses are visit scheduling, quote complexity and quote speed. Keep the conversation short and the quote one number.
- docs/scenarios.md items 1-10 are the advisor test fixtures.
