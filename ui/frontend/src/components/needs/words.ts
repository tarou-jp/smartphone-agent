// How Needs you cards name rules and agents, after ui/i18n.go.

import type { T } from '@/i18n';
import type { Need } from '@/api/types';

const ruleKeys: Record<string, string> = {
  Payments: 'rulePayments',
  Messages: 'ruleMessages',
  Commits: 'ruleCommits',
  'Installs and links': 'ruleInstallsLinks',
  Calls: 'ruleCalls',
  'Settings changes': 'ruleSettingsChanges',
  'Every action': 'ruleEveryAction',
  'Blocked by you': 'ruleBlockedByYou',
  credentials: 'ruleCredentials',
  'Read-only': 'ruleReadOnly',
  'Task apps': 'ruleTaskApps',
  'Agent scope': 'ruleAgentScope',
  'Finance apps': 'ruleFinanceApps',
  'Password managers': 'rulePasswordManagers',
  Files: 'ruleFiles',
  'Agent request': 'ruleAgentRequest',
};

const actionKeys: Record<string, string> = {
  Payments: 'ruleActionPayments',
  Messages: 'ruleActionMessages',
  Commits: 'ruleActionCommits',
  'Installs and links': 'ruleActionInstallsLinks',
  Calls: 'ruleActionCalls',
  'Settings changes': 'ruleActionSettingsChanges',
  'Every action': 'ruleActionEveryAction',
  'Agent request': 'ruleActionAgentRequest',
};

/** A rule as the person reads it; the person's own rules keep their names. */
export function ruleName(t: T, rule: string): string {
  return ruleKeys[rule] ? t(ruleKeys[rule]) : rule;
}

/** What a rule covers, inside "Always allow payments in Dinely". */
export function ruleAction(t: T, rule: string | undefined): string {
  if (!rule) return t('ruleActionThisKind');
  if (actionKeys[rule]) return t(actionKeys[rule]);
  return t('ruleActionOther', { rule: rule.charAt(0).toLowerCase() + rule.slice(1) });
}

export function agentOf(t: T, n: Need): string {
  return n.agent || t('agentGeneric');
}

/** An agent's first connection, which asks what it may do. */
export function isNewAgent(n: Need): boolean {
  return n.kind === 'approval' && n.rule === 'New agent';
}
