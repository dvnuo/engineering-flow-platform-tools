# Studio Panel Grammar

Studio templates use panels to turn a semantic visual into an explorable presentation layer. Read this file after the selected template guide and before writing `panels[]`.

## When to use this template

Use Studio when the user asks for an explanation, walkthrough, dashboard, page, or explorable view. Do not use Studio as a replacement for a focused diagram when the user only needs one graph, matrix, timeline, or UML view.

## Semantic model

- `hero` is the first visual story surface.
- `navigation` links the viewer to hero objects or panels.
- `panels` provide inspectable explanations, metrics, evidence, and next-step detail.
- `controls` describe allowed exploration actions such as focus, filter, step, search, and toggle.
- `annotations` attach callouts to semantic ids.

## Required construction rules

1. Always include at least one panel with non-empty `id`, `type`, and `title`.
2. Every panel must explain a real semantic object, relationship, event, item, participant, message, or narrative step.
3. Use `target_refs` to connect panels to ids in `hero.data`.
4. Put prose in `summary` or `content`, not in oversized labels.
5. Include `controls[]` whenever the page is meant to be explored.
6. Include `navigation[]` when there is more than one panel or story region.
7. Use local mark fields on hero data so the Studio page has recognizable objects and arrows.

## Recommended fields

Use `panel.type`, `target_refs`, `summary`, `items`, `metrics`, `evidence`, `content`, `importance`, `visibility`, `presentation`, `view`, `renderHints`, and `visual` from `common-visual-quality.md`.

## Visual encoding rules

Panel order should match the viewer's reading path: overview, critical path, detail, risks, and actions. Use concise panel titles, metrics for comparison, evidence for trust, and controls for exploration. The hero should keep semantic marks, directed edges, arrows, color policy, and legend.

## Common mistakes to avoid

- Empty panels or panels that only repeat the hero title.
- Controls that do not name a target or action.
- Navigation labels that do not match panel titles.
- Hero data with generic nodes, missing arrows, or no legend.
- A Studio page that hides assumptions or unresolved risks.

## Quality checklist before render

- Panels cover the main story and at least one detail region.
- Panel ids are referenced by navigation or controls.
- Hero semantic ids match `target_refs`, annotations, and visual focus ids.
- Directed relationships have `directed=true` and `presentation.arrow`.
- `renderHints.showLegend=true` when color carries meaning.

## Minimal good example

```json
{"hero":{"type":"topology","title":"Order flow","data":{"nodes":[{"id":"api","label":"Order API","kind":"api","presentation":{"icon":"aws.api_gateway"}}]}},"panels":[{"id":"api-panel","type":"inspector","title":"Order API","target_refs":["api"],"summary":"Explains the ingress contract."}]}
```
