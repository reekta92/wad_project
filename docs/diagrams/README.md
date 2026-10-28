# Diagram guide

The five diagrams are editable Mermaid blocks in the documents below. GitHub renders them directly; a text editor always exposes the source. Each diagram has one maintained source, alongside the behavior it explains.

| Diagram | Source and explanation |
| --- | --- |
| System context / components | [Architecture — system context](../architecture.md#system-context) |
| Authentication sequence | [Architecture — authentication sequence](../architecture.md#authentication-sequence) |
| Entity relationships and schema | [Data model — entity relationship diagram](../data-model.md#entity-relationship-diagram) |
| Planning, staffing, publication and clear | [Workflows — prepare, plan, staff, publish](../workflows.md#prepare-plan-staff-publish) |
| Unavailability request states | [Workflows — request lifecycle](../workflows.md#request-lifecycle) |

To edit, change the relevant fenced `mermaid` block and preview it with GitHub or an existing Mermaid-capable Markdown viewer. ER relationships describe database cardinalities; service-level constraints remain in the accompanying tables. Derived schedule conditions are not additional stored enum states. Request withdrawal means deletion, not a stored status.

For an exported image, copy a block into an available Mermaid renderer or use an already-installed Mermaid CLI with `mmdc -i diagram.mmd -o diagram.svg`. No renderer dependency is added to the application. The pre-existing [Graphviz architecture source](../architecture.dot) remains available for its original overview.
