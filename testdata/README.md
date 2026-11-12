# Synthetic test data

Every resource fixture in this directory is generated example data. Names use example domains, management hosts use documentation address ranges and MAC addresses are locally administered. Cell address/prefix examples are synthetic. No inventory, device-output capture or credential from an operational environment is included.

Fixtures cover a healthy two-spine/two-leaf Clos, unresolved references, ambiguous claims, Cell UID mismatch, multiple targeted Cells, unknown status/roles, forbidden/unavailable sources and partial/empty lists. They are read only by fixture mode and tests; the application never installs them into a cluster.
