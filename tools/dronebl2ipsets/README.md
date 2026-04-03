# dronebl2ipsets

`dronebl2ipsets` is the isolated DroneBL implementation used by the main
`update-ipsets` daemon. It is intentionally a nested importable Go module:
DroneBL-specific rsync and `buildzone` parsing stay here, while the daemon
only calls the package and continues processing normal local `.source` files.

The package:

- fetches DroneBL `buildzone` with authenticated `rsync`;
- reads the rsync secret from `DRONEBL_RSYNC_PASSWORD` or `RSYNC_PASSWORD`;
- parses DroneBL classes into FireHOL source files;
- writes `dronebl_*.source` files for the main application to consume.

Do not add a `main` package here and do not install a separate binary. The
only supported runtime entry point is the root `update-ipsets` application.
