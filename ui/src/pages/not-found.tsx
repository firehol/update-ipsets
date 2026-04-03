import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";

export function NotFoundPage() {
  return (
    <div className="container py-24 text-center">
      <h1 className="font-display text-6xl font-bold">404</h1>
      <p className="mt-2 text-lg text-muted-foreground">
        That page does not exist on this site.
      </p>
      <Button asChild className="mt-6">
        <Link to="/">Back to the homepage</Link>
      </Button>
    </div>
  );
}
