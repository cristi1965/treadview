const major = Number.parseInt(process.versions.node.split('.')[0] || '0', 10);

if (!Number.isFinite(major) || major < 20) {
  console.error(`Node ${process.versions.node} is too old for this Vite build. Use Volta Node 20.20.2 or newer.`);
  process.exit(1);
}
