import { Component, type ErrorInfo, type ReactNode } from "react";

/**
 * Per-section error boundary. Wraps each detail-page section so a single
 * section crash never takes down the whole page. Renders a quiet
 * fallback that names which section failed and why, so the rest of the
 * page stays usable while you debug.
 *
 * Class component is the only way to catch render-time errors in React
 * 18/19 — there is no hook equivalent today.
 */
interface Props {
  name: string;
  children: ReactNode;
}
interface State {
  error: Error | null;
}

export class SectionErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Log to the console so the error and component stack remain
    // visible in dev — production logging would go through a real
    // observability hook.
    console.error("Section error:", this.props.name, error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <section className="page-container py-16">
          <div className="border-l-[3px] border-primary pl-6">
            <div className="eyebrow text-muted-foreground">Section error</div>
            <h2 className="display-subtitle mt-2">{this.props.name} could not render</h2>
            <p className="lede mt-3">
              An unexpected error prevented this section from displaying. The rest of the
              page is unaffected.
            </p>
            <pre className="mt-4 max-w-[60ch] overflow-x-auto rounded-sm border border-border bg-muted/40 p-4 text-[12px] text-muted-foreground">
              {String(this.state.error.message || this.state.error)}
            </pre>
          </div>
        </section>
      );
    }
    return this.props.children;
  }
}
