const demoSource = {
  nested: {
    label: 'optional chaining ok',
    items: ['a', 'b']
  }
};

const demoResult = {
  label: demoSource?.nested?.label ?? 'missing',
  items: [
    ...(demoSource?.nested?.items ?? []).map((item) => item.toUpperCase()),
    demoSource?.missing?.fallback ?? 'fallback'
  ]
};
