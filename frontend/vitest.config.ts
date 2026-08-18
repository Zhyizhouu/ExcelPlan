import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// Kept separate from vite.config.ts because the build tsconfig types against
// node only, and a `test` key there fails the typecheck it is not part of.
export default defineConfig({
  plugins: [react()],
  test: {
    // jsdom for localStorage, which the phase-celebration logic persists
    // through — the "don't congratulate twice" rule is only testable with it.
    environment: "jsdom",
  },
});
