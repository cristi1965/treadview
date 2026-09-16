import { I18nKey, translate } from '../i18n';
import { ShellLanguage } from '../stores/uiStore';
import { ReportView, formatBilingualLine } from './bilingual';

export type ConsoleLog = {
  ts: number;
  kind: 'start' | 'progress' | 'node_start' | 'node_complete' | 'done' | 'error' | 'start_fail' | 'stop_fail';
  node?: string;
  progress?: number;
  extra?: string;
};

const phaseKeys: Record<string, I18nKey> = {
  Idle: 'log.phaseIdle',
  'Initiating...': 'log.phaseInit',
  'Stopped by User': 'log.phaseStopped',
  Complete: 'log.phaseComplete',
  Failed: 'log.phaseFailed',
  'Phase 1: Analyst Team': 'log.phase1',
  'Phase 2: Research Debate': 'log.phase2',
  'Phase 3: Trading': 'log.phase3',
  'Phase 4: Risk Management': 'log.phase4',
  'Phase 5: Final Decision': 'log.phase5',
};

const agentKeys: Record<string, I18nKey> = {
  'Market Analyst': 'flow.tech',
  'Fundamentals Analyst': 'flow.fund',
  'Sentiment Analyst': 'flow.sent',
  'News Analyst': 'flow.news',
  'Research Manager': 'flow.rm',
  Trader: 'flow.trader',
  'Portfolio Manager': 'flow.pm',
  'X Players': 'log.agentX',
  'Bull Researcher': 'log.agentBull',
  'Bear Researcher': 'log.agentBear',
  'Aggressive Analyst': 'log.agentAgg',
  'Conservative Analyst': 'log.agentCon',
  'Neutral Analyst': 'log.agentNeu',
};

export function localizeAgent(name: string, lang: ShellLanguage): string {
  const key = agentKeys[name];
  return key ? translate(lang, key) : name;
}

export function localizePhase(phase: string, lang: ShellLanguage): string {
  if (!phase) return '';
  const exact = phaseKeys[phase];
  if (exact) return translate(lang, exact);
  if (phase.startsWith('Phase 1')) return translate(lang, 'log.phase1');
  if (phase.startsWith('Phase 2')) return translate(lang, 'log.phase2');
  if (phase.startsWith('Phase 3')) return translate(lang, 'log.phase3');
  if (phase.startsWith('Phase 4')) return translate(lang, 'log.phase4');
  if (phase.startsWith('Phase 5')) return translate(lang, 'log.phase5');
  return phase;
}

function line(
  view: ReportView,
  key: I18nKey,
  varsFor?: (lang: ShellLanguage) => Record<string, string | number> | undefined,
): string {
  return formatBilingualLine(
    view,
    translate('zh', key, varsFor?.('zh')),
    translate('en', key, varsFor?.('en')),
  );
}

export function formatConsoleLog(log: ConsoleLog, view: ReportView): string {
  const time = new Date(log.ts).toLocaleTimeString();
  const pct = log.progress ?? 0;
  const node = log.node || '';
  const err = log.extra || '';

  let body = '';
  switch (log.kind) {
    case 'start':
      body = line(view, 'log.pipeStart');
      break;
    case 'progress':
      body = line(view, 'log.pipeProgress', (lang) => ({ phase: localizePhase(node, lang), pct }));
      break;
    case 'node_start':
      body = line(view, 'log.agentRun', (lang) => ({ agent: localizeAgent(node, lang) }));
      break;
    case 'node_complete':
      body = line(view, 'log.agentDone', (lang) => ({ agent: localizeAgent(node, lang) }));
      break;
    case 'done':
      body = line(view, 'log.pipeDone');
      break;
    case 'error':
      body = line(view, 'log.pipeFail', () => ({ err }));
      break;
    case 'start_fail':
      body = line(view, 'log.startFail', () => ({ err }));
      break;
    case 'stop_fail':
      body = line(view, 'log.stopFail', () => ({ err }));
      break;
    default:
      body = log.extra || '';
  }
  return `[${time}] ${body}`;
}

export function formatPhaseLabel(phase: string, view: ReportView): string {
  return formatBilingualLine(view, localizePhase(phase, 'zh'), localizePhase(phase, 'en'));
}
