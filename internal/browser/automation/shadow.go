package automation

func shadowTraversalExpression() string {
	return `
function querySelectorAllPierce(root, selector, pierce, maxVisited) {
  maxVisited = Math.max(1, Number(maxVisited || 10000));
  if (!pierce) return Array.from(root.querySelectorAll(selector));
  const out = [];
  const seen = new Set();
  const roots = [root];
  let visited = 0;
  while (roots.length && visited < maxVisited) {
    const current = roots.shift();
    if (!current || !current.querySelectorAll) continue;
    for (const node of Array.from(current.querySelectorAll(selector))) {
      visited++;
      if (!seen.has(node)) {
        seen.add(node);
        out.push(node);
      }
      if (visited >= maxVisited) break;
    }
    if (visited >= maxVisited) break;
    for (const el of Array.from(current.querySelectorAll("*"))) {
      visited++;
      if (el.shadowRoot) roots.push(el.shadowRoot);
      if (visited >= maxVisited) break;
    }
  }
  return out;
}`
}
