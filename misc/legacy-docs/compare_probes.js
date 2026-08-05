const fs = require('fs');
const path = require('path');

const liveProbePath = '/Applications/workspace/ai管力/账本/TradingAgents/recovered_source/live-core-probe/site-probe.json';
const localProbePath = '/Applications/workspace/ai管力/账本/TradingAgents/recovered_source/local-core-probe/site-probe.json';

if (!fs.existsSync(liveProbePath) || !fs.existsSync(localProbePath)) {
  console.error("Error: One of the probe files does not exist!");
  console.error("Live path:", liveProbePath, "Exists:", fs.existsSync(liveProbePath));
  console.error("Local path:", localProbePath, "Exists:", fs.existsSync(localProbePath));
  process.exit(1);
}

const liveData = JSON.parse(fs.readFileSync(liveProbePath, 'utf8'));
const localData = JSON.parse(fs.readFileSync(localProbePath, 'utf8'));

const liveRoutes = {};
liveData.routes.forEach(r => {
  liveRoutes[r.route] = r;
});

const localRoutes = {};
localData.routes.forEach(r => {
  localRoutes[r.route] = r;
});

console.log('| Route | Live Title | Local Title | Live Text | Local Text | Match % | Live Err | Local Err | Failed Requests | Console Errors |');
console.log('| --- | --- | --- | ---: | ---: | ---: | --- | --- | ---: | ---: |');

Object.keys(liveRoutes).forEach(route => {
  const live = liveRoutes[route];
  const local = localRoutes[route] || {};

  const liveText = live.textLength || 0;
  const localText = local.textLength || 0;
  const matchPct = liveText > 0 ? ((Math.min(liveText, localText) / Math.max(liveText, localText)) * 100).toFixed(1) + '%' : '100.0%';

  const liveErr = live.pageError ? 'Yes' : 'No';
  const localErr = local.pageError ? 'Yes' : 'No';

  const failedReqCount = (local.failedRequests || []).length;
  const consoleErrCount = (local.consoleErrors || []).length;

  console.log(`| \`${route}\` | ${live.title || '-'} | ${local.title || '-'} | ${liveText} | ${localText} | ${matchPct} | ${liveErr} | ${localErr} | ${failedReqCount} | ${consoleErrCount} |`);
});
