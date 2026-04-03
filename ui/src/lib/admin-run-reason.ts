export function formatRunReason(reason: string | undefined): string {
  switch (reason) {
    case "scheduled_due":
      return "scheduled due";
    case "manual_run":
      return "manual run";
    case "manual_recheck":
      return "manual recheck";
    case "manual_reprocess":
      return "manual reprocess";
    case "startup_integrity_reprocess":
      return "startup integrity recovery";
    case "integrity_reprocess":
      return "integrity recovery";
    case "dependency_update":
      return "dependency update";
    case "provider_defaults_update":
      return "provider defaults update";
    default:
      return "—";
  }
}

export function describeRunReason(reason: string | undefined): string {
  switch (reason) {
    case "scheduled_due":
      return "The scheduler picked this feed because its next check was due.";
    case "manual_run":
      return "An operator explicitly triggered a normal run.";
    case "manual_recheck":
      return "An operator explicitly forced a recheck, bypassing normal cadence.";
    case "manual_reprocess":
      return "An operator explicitly forced a reprocess.";
    case "startup_integrity_reprocess":
      return "Daemon startup integrity found local output drift and queued the required recovery plan.";
    case "integrity_reprocess":
      return "An operator used the integrity recovery action.";
    case "dependency_update":
      return "This derivative feed ran because one of its parent feeds updated.";
    case "provider_defaults_update":
      return "A supporting data provider (ASN, GeoIP, or bogons) changed its database, triggering a full reprocess wave.";
    default:
      return "No recorded reason.";
  }
}
