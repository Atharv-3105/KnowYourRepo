import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MotionConfig } from "motion/react";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App.tsx";
import { TooltipProvider } from "./components/ui/Tooltip.tsx";
import "./index.css";

const queryClient = new QueryClient();

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
