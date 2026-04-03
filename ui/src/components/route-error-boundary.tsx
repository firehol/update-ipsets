import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

export class RouteErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("route render error:", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <main className="page-container py-20">
          <div className="border-l-[3px] border-primary pl-6">
            <div className="eyebrow text-muted-foreground">Page error</div>
            <h1 className="display-subtitle mt-2">This page could not render</h1>
            <p className="lede mt-3">
              An unexpected error prevented this route from displaying.
            </p>
            <pre className="mt-4 max-w-[60ch] overflow-x-auto rounded-sm border border-border bg-muted/40 p-4 text-[12px] text-muted-foreground">
              {String(this.state.error.message || this.state.error)}
            </pre>
          </div>
        </main>
      );
    }
    return this.props.children;
  }
}

export function RouteLoadingFallback() {
  return (
    <main className="page-container py-20">
      <div className="border-l-[3px] border-primary pl-6">
        <div className="eyebrow text-muted-foreground">Loading</div>
        <div className="mt-3 h-7 w-56 animate-pulse rounded-sm bg-muted" />
        <div className="mt-4 h-4 w-80 max-w-full animate-pulse rounded-sm bg-muted/70" />
      </div>
    </main>
  );
}
