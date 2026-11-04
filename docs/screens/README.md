# Screenshots

Captured by the end-to-end scripts against the live stack (`make up seed`, DeepSeek as the advisor's LLM), in headless Microsoft Edge:

- `phone-*`: web app at 390×844 (phone). Demo owner "Dental clinic, Chennai (Tamil)": Tamil chat → quote → consent → KYC with specimen K03 (pass) → tracker to activation, with notify messages in Tamil. Script: `ts/apps/web/scripts/e2e.mjs`.
- `desktop-*`: web app at 1280×800. Demo owner "Physio clinic where fibre is not feasible": KYC with K09 (pass), installation delayed with fibre_not_feasible, 5G AirFiber offered and accepted, activation. Then the back office: orders, KYC review queue, messages.
- `mobile-*`: the Expo app's web build (react-native-web) at 390×844. Demo owner "Restaurant chain, 5 sites": quote per site → KYC with K12 (pass) → activation. Script: `ts/apps/web/scripts/e2e-mobile.mjs`. On a real phone the KYC slots open the rear camera (expo-image-picker); the web build uses the sample documents.

Re-run: `make web-e2e` (needs the web app on :3000) and `make mobile-e2e` (exports the Expo web build and serves it on :19006).
