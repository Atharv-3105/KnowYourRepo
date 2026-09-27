import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MotionConfig } from "motion/react";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { ApiError } from "./api";
import App from "./App.tsx";
import { TooltipProvider } from "./components/ui/Tooltip.tsx";
import "./index.css";

// Client errors (4xx) never succeed on retry, and a 429 means the rate
// limit wants fewer requests, not more. A 503 is transient by contract - e.g.
// a repo's files being restored after a server restart - so it gets a
// longer, patient retry.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (error instanceof ApiError) {
          if (error.status === 503) return failureCount < 6;
          if (error.status < 500) return false;
        }
        return failureCount < 2;
      },
      retryDelay: (attempt, error) =>
        error instanceof ApiError && error.status === 503 ? 5000 : Math.min(1000 * 2 ** attempt, 8000),
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    {/* index.css's prefers-reduced-motion rule only zeroes real CSS
        transitions/animations - it has no effect on motion's own
        JS-driven animations (the nav indicator, dialogs, Chat's message
        settle/sources reveal, Code View's slide-in), which render via
        inline styles, not CSS transition classes. reducedMotion="user"
        is motion's own mechanism for the same OS setting: it disables
        transform/layout animation on every motion component in the tree
        while still applying the final state, so nothing gets stuck
        mid-animation. */}
    <MotionConfig reducedMotion="user">
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </TooltipProvider>
      </QueryClientProvider>
    </MotionConfig>
  </StrictMode>,
);
