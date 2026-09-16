const fs = require('fs');
const path = require('path');

const outDir = path.join(__dirname, '../app/frontend/public/images/avatars');
if (!fs.existsSync(outDir)) {
  fs.mkdirSync(outDir, { recursive: true });
}

const investorsPath = path.join(__dirname, '../app/backend/internal/database/investors.json');
const membersPath = path.join(__dirname, '../app/backend/internal/database/members.json');

const investors = fs.existsSync(investorsPath) ? JSON.parse(fs.readFileSync(investorsPath, 'utf8')) : [];
const members = fs.existsSync(membersPath) ? JSON.parse(fs.readFileSync(membersPath, 'utf8')) : [];

console.log(`Loaded ${investors.length} investors and ${members.length} congress members.`);

const PALETTES = [
  { g1: '#f59e0b', g2: '#78350f', border: '#f59e0b', text: '#fef3c7' }, // Gold
  { g1: '#3b82f6', g2: '#1e3a8a', border: '#3b82f6', text: '#dbeafe' }, // Blue
  { g1: '#10b981', g2: '#064e3b', border: '#10b981', text: '#d1fae5' }, // Emerald
  { g1: '#8b5cf6', g2: '#4c1d95', border: '#8b5cf6', text: '#ede9fe' }, // Purple
  { g1: '#ec4899', g2: '#831843', border: '#ec4899', text: '#fce7f3' }, // Pink
  { g1: '#06b6d4', g2: '#164e63', border: '#06b6d4', text: '#cffafe' }, // Cyan
  { g1: '#f97316', g2: '#7c2d12', border: '#f97316', text: '#ffedd5' }, // Orange
  { g1: '#6366f1', g2: '#312e81', border: '#6366f1', text: '#e0e7ff' }, // Indigo
];

function getMonogram(name, slug) {
  const raw = (name || slug || '').trim();
  const cn = raw.match(/[\u4e00-\u9fa5]/g);
  if (cn && cn.length >= 2) return cn.slice(0, 2).join('');
  if (cn && cn.length === 1) return cn[0];
  const words = raw.replace(/[^a-zA-Z0-9\s]/g, ' ').split(/\s+/).filter(Boolean);
  if (words.length >= 2) {
    return (words[0][0] + words[1][0]).toUpperCase();
  }
  return raw.slice(0, 2).toUpperCase() || 'IV';
}

function generateSvgAvatar(monogram, subtitle, palette, isCongress = false, party = '') {
  let g1 = palette.g1;
  let g2 = palette.g2;
  let border = palette.border;
  let text = palette.text;

  if (isCongress) {
    if (party === 'Democratic' || party === 'D' || party === 'democrat') {
      g1 = '#2563eb'; g2 = '#172554'; border = '#60a5fa'; text = '#eff6ff';
    } else {
      g1 = '#dc2626'; g2 = '#450a0a'; border = '#f87171'; text = '#fef2f2';
    }
  }

  const fontSize = monogram.length > 2 ? 50 : 66;

  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200" width="200" height="200">
  <defs>
    <radialGradient id="bgGrad_${monogram}" cx="35%" cy="30%" r="70%">
      <stop offset="0%" stop-color="${g1}" />
      <stop offset="100%" stop-color="${g2}" />
    </radialGradient>
    <linearGradient id="ringGrad_${monogram}" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="${border}" stop-opacity="0.95" />
      <stop offset="100%" stop-color="${border}" stop-opacity="0.3" />
    </linearGradient>
  </defs>

  <!-- Outer background circle -->
  <circle cx="100" cy="100" r="95" fill="url(#bgGrad_${monogram})"/>
  
  <!-- Sleek metallic outer ring -->
  <circle cx="100" cy="100" r="92" fill="none" stroke="url(#ringGrad_${monogram})" stroke-width="4"/>
  <circle cx="100" cy="100" r="86" fill="none" stroke="rgba(255,255,255,0.15)" stroke-width="1.5"/>

  <!-- Inner subtle pattern circle -->
  <circle cx="100" cy="100" r="80" fill="none" stroke="rgba(0,0,0,0.25)" stroke-width="1" stroke-dasharray="4 4"/>

  <!-- Main Monogram Text -->
  <text x="100" y="${subtitle ? 106 : 120}" 
        font-family="system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" 
        font-size="${fontSize}" 
        font-weight="900" 
        fill="${text}" 
        text-anchor="middle" 
        letter-spacing="1">
    ${monogram}
  </text>

  ${subtitle ? `
  <rect x="45" y="132" width="110" height="22" rx="11" fill="rgba(0,0,0,0.4)" />
  <text x="100" y="147" 
        font-family="system-ui, -apple-system, sans-serif" 
        font-size="10" 
        font-weight="700" 
        fill="${border}" 
        text-anchor="middle" 
        letter-spacing="0.5">
    ${subtitle}
  </text>` : ''}
</svg>`;
}

let count = 0;

// 1. Generate for all investors
investors.forEach((inv) => {
  const slug = inv.slug;
  if (!slug) return;

  const svgPath = path.join(outDir, `${slug}.svg`);
  const jpgPath = path.join(outDir, `${slug}.jpg`);

  // Don't overwrite real photographic JPGs if they are large (>50KB)
  const isLargePhoto = fs.existsSync(jpgPath) && fs.statSync(jpgPath).size > 50000;

  const monogram = getMonogram(inv.name || inv.name_en || inv.entity, slug);
  const hash = Array.from(slug).reduce((acc, c) => acc + c.charCodeAt(0), 0);
  const palette = PALETTES[Math.abs(hash) % PALETTES.length];
  const subtitle = inv.type === 'superinvestor' ? '13F GURU' : 'FUND';

  const svgContent = generateSvgAvatar(monogram, subtitle, palette, false);
  fs.writeFileSync(svgPath, svgContent, 'utf8');
  if (!isLargePhoto) {
    fs.writeFileSync(jpgPath, svgContent, 'utf8');
  }
  count++;
});

// 2. Generate for all congress members
members.forEach((mem, idx) => {
  const slug = mem.slug || mem.name.toLowerCase().replace(/[^a-z0-9]/g, '-');
  if (!slug) return;

  const svgPath = path.join(outDir, `${slug}.svg`);
  const jpgPath = path.join(outDir, `${slug}.jpg`);

  const isLargePhoto = fs.existsSync(jpgPath) && fs.statSync(jpgPath).size > 50000;

  const monogram = getMonogram(mem.name, slug);
  const hash = Array.from(slug).reduce((acc, c) => acc + c.charCodeAt(0), 0);
  const palette = PALETTES[Math.abs(hash) % PALETTES.length];
  const party = mem.party || (idx % 2 === 0 ? 'Democratic' : 'Republican');
  const subtitle = (party === 'Democratic' || party === 'D' || party === 'democrat') ? 'DEMOCRAT' : 'REPUBLICAN';

  const svgContent = generateSvgAvatar(monogram, subtitle, palette, true, party);
  fs.writeFileSync(svgPath, svgContent, 'utf8');
  if (!isLargePhoto) {
    fs.writeFileSync(jpgPath, svgContent, 'utf8');
  }
  count++;
});

console.log(`Successfully generated and cached ${count} local avatar image files in ${outDir}!`);
