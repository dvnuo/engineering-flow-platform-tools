# uml.state_machine_3d Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for UML-style semantic views where the user needs a precise model of interactions, structure, state, activity, or deployment. Do not use it for unordered metric dashboards or free-form knowledge graphs.

## Semantic model

- UML object = domain-specific model element, not a generic node.
- Relationship/transition/message = typed semantic connection.
- Phase/state/lane/deployment = visual grouping with meaning.
- `visual.initial_focus_ids` = first-view UML elements.
- `visual.hidden_detail_ids` = low-value implementation detail.
- `visual.annotations` = callouts for critical path, risk, or result.

## Required construction rules

1. Preserve the selected UML semantic shape; do not convert it to generic graph nodes.
2. Use stable architecture/runtime actors rather than every function unless the user explicitly asks for code-level detail.
3. Every major element should include id, label, kind, importance, summary, and a meaningful semantic group/lane/state where applicable.
4. Use phases, fragments, groups, or regions to compress repetition.
5. Hide low-value details with visibility/detail fields and visual.hidden_detail_ids.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Position should encode UML semantics such as order, containment, state, lane, component tier, or deployment boundary. Color should encode phase, state, kind, or ownership. Thickness/glow should encode importance. Labels should follow labelPriority and not show every detail at once.

## Common mistakes to avoid

- Do not turn UML into generic graph nodes.
- Do not create one element for every tiny method unless requested.
- Do not use long method signatures as labels.
- Do not omit visual focus, annotations, or summaries.
- Do not make all elements the same importance.

## Quality checklist before render

- The selected UML type matches the question.
- IDs referenced by relationships/messages/transitions exist.
- Important elements have importance >= 0.75.
- Low-value detail is marked detail/hidden.
- visual.goal, initial_focus_ids, narrative_steps, and annotations are present.

## Minimal good example

```json
{
  "visual": {"goal": "Explain the critical path first", "initial_focus_ids": ["main"], "annotations": [{"id": "a1", "target_id": "main", "label": "Critical path"}]},
  "view": {"mode": "overview", "labelMode": "overview"}
}
```
